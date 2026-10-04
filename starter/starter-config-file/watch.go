/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package StarterConfigFile integrates local filesystem configuration as
// hot-reloadable configuration sources for Go-Spring. One blank-import registers
// two providers that share a single watch + refresh bridge:
//
//   - "file-watch" (filewatch.go) — one configuration document per import (a
//     ConfigMap key holding application.yaml), parsed by extension. Layered
//     overrides compose via spring.config.import order; it never merges a directory.
//   - "configtree" (configtree.go) — a directory of scalar key files (a Secret /
//     env-style ConfigMap mount). Each leaf file maps to one property keyed by
//     its dotted relative path, valued by its unparsed content.
//
// Both watch the parent directory (never the file itself), so the kubelet's
// atomic "..data" symlink swap on a Kubernetes ConfigMap/Secret update is
// detected and turned into a live property refresh without a restart.
//
// This starter covers local file/volume watching only. Remote configuration
// centers (Nacos, etcd, Consul) are separate starters.
package StarterConfigFile

import (
	"context"
	"errors"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

// starterTag is the infra log tag shared by both providers. Each provider
// owns its own controller type (fileWatchCtrl in filewatch.go,
// configTreeCtrl in configtree.go), created in its init closure — no
// package-level controller variable exists.
var starterTag = log.RegisterAppTag("config_file", "")

// watchCore is the machinery both providers embed: the watched directories
// plus the change-to-refresh bridge. The per-provider controllers
// (fileWatchCtrl in filewatch.go, configTreeCtrl in configtree.go) read
// config; ensureWatch/watchLoop deliver change events; TriggerRefresh fans
// them out into a full application property refresh via the
// gs.RefreshProperties package-level facade — no bean wiring needed. Close
// stops every watcher, which is how both controllers satisfy
// conf/provider.Provider.
type watchCore struct {
	mu       sync.Mutex
	watchers map[string]*fsnotify.Watcher // directory -> its watcher
}

// newWatchCore creates the shared watch machinery with its watcher set ready,
// so ensureWatch needs no lazy nil-check. The per-provider constructors in
// filewatch.go and configtree.go embed it.
func newWatchCore() watchCore {
	return watchCore{watchers: map[string]*fsnotify.Watcher{}}
}

// Close stops every watcher. A Load that follows re-arms watchCore: ensureWatch
// finds the directory absent from the set and starts a fresh watcher.
func (c *watchCore) Close(ctx context.Context) error {
	c.mu.Lock()
	watchers := c.watchers
	c.watchers = map[string]*fsnotify.Watcher{}
	c.mu.Unlock()

	var errs []error
	for _, w := range watchers {
		if err := w.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// isCurrent reports whether w is still the watcher registered for its
// directory. A watcher that Close removed is no longer current, so its event
// channel closing is expected and must not be reported as a failure.
func (c *watchCore) isCurrent(w *fsnotify.Watcher) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, watcher := range c.watchers {
		if watcher == w {
			return true
		}
	}
	return false
}

// TriggerRefresh is called by the watcher goroutines when a watched directory
// changes. Before the app has started, gs.RefreshProperties returns an error
// and the change is dropped — the initial config load already captured the
// state.
func (c *watchCore) TriggerRefresh(ctx context.Context) {
	// The refresh outcome (status, duration, error) is logged and metered
	// centrally by observability.RefreshConf; this layer only records backend events.
	_ = observability.RefreshConf(ctx, gs.RefreshProperties)
}

// ensureWatch starts a background directory watcher for dir, deduplicated so
// repeated Load calls (startup + every refresh) do not stack watchers on the
// same directory. Watching is best-effort: if a watcher cannot be created,
// startup still succeeds with a static snapshot, only losing hot-reload.
func (c *watchCore) ensureWatch(ctx context.Context, dir string) {
	c.mu.Lock()
	if _, ok := c.watchers[dir]; ok {
		c.mu.Unlock()
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		c.mu.Unlock()
		log.Warnf(ctx, starterTag,
			"create file watcher for %s failed, hot-reload disabled for it (static snapshot kept): %v", dir, err)
		return
	}
	if err = w.Add(dir); err != nil {
		_ = w.Close()
		c.mu.Unlock()
		log.Warnf(ctx, starterTag,
			"watch directory %s failed, hot-reload disabled for it (static snapshot kept): %v", dir, err)
		return
	}
	c.watchers[dir] = w
	c.mu.Unlock()

	go c.watchLoop(w)
}

// watchLoop drains a watcher's events and triggers a full application property
// refresh on any change. It intentionally reacts to every event rather than
// filtering by file name: a Kubernetes ConfigMap/Secret update surfaces as a
// CREATE/RENAME on the "..data" symlink (not on the individual key files), so
// coalescing every event into one refresh is both correct and simplest.
func (c *watchCore) watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				c.reportClosed(w)
				return
			}
			// Stamp the trigger with the change's identity (which path changed)
			// so the refresh records logged and metered by observability.RefreshConf
			// carry what this round is about. fsnotify hands over no revision, so
			// the event name is all the identity available; a Kubernetes ConfigMap
			// update surfaces here as the "..data" symlink.
			c.TriggerRefresh(log.WithFields(context.Background(),
				log.String("source", "file"),
				log.String("path", e.Name),
				log.String("op", e.Op.String()),
			))
		case err, ok := <-w.Errors:
			if !ok {
				c.reportClosed(w)
				return
			}
			log.Warnf(context.Background(), starterTag, "file watcher error: %v", err)
		}
	}
}

// reportClosed handles a watcher whose channels closed. Close closing this
// watcher is the expected path and returns quietly; a channel closing while the
// watcher is still registered means hot-reload died on its own, and a silent
// exit there would hide it until restart.
func (c *watchCore) reportClosed(w *fsnotify.Watcher) {
	if !c.isCurrent(w) {
		return
	}
	log.Errorf(context.Background(), starterTag,
		"file watcher closed; hot-reload is disabled until restart")
}
