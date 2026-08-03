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
//     overrides compose via spring.app.imports order; it never merges a directory.
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
	"sync"

	"github.com/fsnotify/fsnotify"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

// starterTag is the infra log tag shared by both providers. fileWatchController
// is the global singleton: the only places it is referenced outside its own
// methods are the init functions (bean wiring here; provider registration in
// filewatch.go and configtree.go). All other code operates on the receiver.
var (
	starterTag          = log.RegisterInfraTag("starter_config_file", "")
	fileWatchController = &configFileController{}
)

func init() {
	// Register the shared controller as a root bean so the IoC container injects
	// its PropertiesRefresher via autowire. Each provider registers itself in its
	// own init (file-watch in filewatch.go, configtree in configtree.go). Before
	// wiring, TriggerRefresh is a harmless no-op — the startup load already
	// captured the initial config.
	gs.Provide(fileWatchController).Export(gs.As[gs.Rooter]())
}

// configFileController owns the lifecycle shared by both providers: it holds the
// IoC-injected PropertiesRefresher and the deduplicated set of watched
// directories. The per-provider Load methods (Load in filewatch.go,
// LoadConfigTree in configtree.go) read config; ensureWatch/watchLoop deliver
// change events; TriggerRefresh fans them out into a full application property
// refresh.
type configFileController struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu      sync.Mutex
	watched map[string]struct{} // directories already watched
}

// TriggerRefresh is called by the watcher goroutines when a watched directory
// changes. Before the IoC container wires the controller, this is a no-op — the
// initial config load already captured the state.
func (c *configFileController) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}

// ensureWatch starts a background directory watcher for dir, deduplicated so
// repeated Load calls (startup + every refresh) do not stack watchers on the
// same directory. Watching is best-effort: if a watcher cannot be created,
// startup still succeeds with a static snapshot, only losing hot-reload.
func (c *configFileController) ensureWatch(dir string) {
	c.mu.Lock()
	if c.watched == nil {
		c.watched = map[string]struct{}{}
	}
	if _, ok := c.watched[dir]; ok {
		c.mu.Unlock()
		return
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		c.mu.Unlock()
		return
	}
	if err = w.Add(dir); err != nil {
		_ = w.Close()
		c.mu.Unlock()
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
func (c *configFileController) watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case _, ok := <-w.Events:
			if !ok {
				return
			}
			c.TriggerRefresh()
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}
