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

// Package StarterConfigFile integrates a mounted directory (or file) as a
// hot-reloadable configuration source for Go-Spring. Blank-importing this
// package registers a "file-watch" config provider that can be consumed via
// spring.app.imports, together with the bridge that wires file changes into
// the application-wide property refresh for live hot-reload.
//
// Its primary purpose is Kubernetes: a ConfigMap or Secret mounted as a volume
// becomes hot-reloadable without any custom code. The kubelet updates such a
// volume by atomically swapping the "..data" symlink, which the directory
// watcher detects and turns into a refresh, so bound gs.Dync fields update
// within seconds of `kubectl edit configmap`.
//
// This starter covers local file/volume watching only. Remote configuration
// centers (Nacos, etcd, Consul) are separate starters.
package StarterConfigFile

import (
	"sync"

	"go-spring.org/spring/gs"
)

func init() {
	// Register the file-watch controller as both a root bean (so the IoC
	// container injects its PropertiesRefresher via autowire) and the
	// "file-watch" config provider (so Load calls go through its method).
	// Before wiring, TriggerRefresh is a harmless no-op — the startup load
	// already captured the initial config.
	gs.Provide(fileWatchController).
		Name("configFileController").
		Export(gs.As[gs.Rooter]())
}

// fileWatchController is the global singleton. It is the ONLY place the
// controller is referenced outside its own methods: the init functions in this
// file and in provider.go wire it into the IoC container and the conf provider
// respectively. All other code (Load, ensureWatch, watchLoop, readDir, etc.)
// operates on the receiver without touching this global.
var fileWatchController = &configFileController{}

// configFileController is the single object that owns the full lifecycle of
// file-watch configuration: loading files, watching directories, and triggering
// property refresh on changes.
type configFileController struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu      sync.Mutex
	watched map[string]struct{} // directories already watched
}

// TriggerRefresh is called by the watcher goroutines when a mounted directory
// changes. Before the IoC container wires the controller, this is a no-op —
// the initial config load already captured the state.
func (c *configFileController) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}
