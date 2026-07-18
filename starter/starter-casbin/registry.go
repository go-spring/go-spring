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

package StarterCasbin

import (
	"sync"

	"github.com/casbin/casbin/v2/persist"
)

// The starter stays free of any database/storage driver on purpose: bringing in
// a GORM/Redis/etcd adapter would drag those dependencies into every project
// that only needs the default file policy. Instead, an application registers the
// persist.Adapter (and optional persist.Watcher) it wants by name before the
// container starts, and points a Casbin instance at it via the `adapter` /
// `watcher` config keys. This keeps adapter/watcher wiring fully optional.

var (
	adaptersMu sync.RWMutex
	adapters   = make(map[string]persist.Adapter)

	watchersMu sync.RWMutex
	watchers   = make(map[string]persist.Watcher)
)

// RegisterAdapter registers a persist.Adapter under name so a Casbin instance
// can select it with `spring.casbin.<inst>.adapter=<name>`. Call it during
// application bootstrap, before gs.Run.
func RegisterAdapter(name string, a persist.Adapter) {
	adaptersMu.Lock()
	defer adaptersMu.Unlock()
	adapters[name] = a
}

// RegisterWatcher registers a persist.Watcher under name so a Casbin instance
// can select it with `spring.casbin.<inst>.watcher=<name>`. Call it during
// application bootstrap, before gs.Run.
func RegisterWatcher(name string, w persist.Watcher) {
	watchersMu.Lock()
	defer watchersMu.Unlock()
	watchers[name] = w
}

func lookupAdapter(name string) (persist.Adapter, bool) {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()
	a, ok := adapters[name]
	return a, ok
}

func lookupWatcher(name string) (persist.Watcher, bool) {
	watchersMu.RLock()
	defer watchersMu.RUnlock()
	w, ok := watchers[name]
	return w, ok
}
