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

// watchCore is the machinery both providers embed: the deduplicated set of
// watched directories plus the change-to-refresh bridge. The per-provider
// controllers (fileWatchCtrl in filewatch.go, configTreeCtrl in configtree.go)
// read config; ensureWatch/watchLoop deliver change events; TriggerRefresh
// fans them out into a full application property refresh via the
// gs.RefreshProperties package-level facade — no bean wiring needed.
type watchCore struct {
	mu      sync.Mutex
	watched map[string]struct{} // directories already watched
}

// newWatchCore creates the shared watch machinery with its dedup set ready,
// so ensureWatch needs no lazy nil-check. The per-provider constructors in
// filewatch.go and configtree.go embed it.
func newWatchCore() watchCore {
	return watchCore{watched: map[string]struct{}{}}
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
func (c *watchCore) ensureWatch(dir string) {
	c.mu.Lock()
	if _, ok := c.watched[dir]; ok {
		c.mu.Unlock()
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		c.mu.Unlock()
		log.Warnf(context.Background(), starterTag,
			"create file watcher for %s failed, hot-reload disabled for it (static snapshot kept): %v", dir, err)
		return
	}
	if err = w.Add(dir); err != nil {
		_ = w.Close()
		c.mu.Unlock()
		log.Warnf(context.Background(), starterTag,
			"watch directory %s failed, hot-reload disabled for it (static snapshot kept): %v", dir, err)
		return
	}
	c.watched[dir] = struct{}{}
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
		case _, ok := <-w.Events:
			if !ok {
				// The events channel closes only when the watcher is closed
				// out from under us; without this line the goroutine would
				// exit silently and hot-reload would stop working.
				log.Errorf(context.Background(), starterTag,
					"file watcher closed; hot-reload is disabled until restart")
				return
			}
			c.TriggerRefresh(context.Background())
		case err, ok := <-w.Errors:
			if !ok {
				log.Errorf(context.Background(), starterTag,
					"file watcher closed; hot-reload is disabled until restart")
				return
			}
			log.Warnf(context.Background(), starterTag, "file watcher error: %v", err)
		}
	}
}
