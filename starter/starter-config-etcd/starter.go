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

// Package StarterConfigEtcd integrates etcd as a remote configuration
// center for Go-Spring. Blank-importing this package registers an "etcd"
// config provider that can be consumed via spring.app.imports, together with
// the bridge that wires remote config changes into the application-wide
// property refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery
// (etcd naming) is a separate concern and is not provided here.
package StarterConfigEtcd

import (
	"sync"

	"go-spring.org/spring/gs"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	// Register the etcd controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "etcd" config
	// provider (so Load calls go through its method). Before wiring,
	// TriggerRefresh is a harmless no-op — the startup load already captured
	// the initial config.
	gs.Provide(etcdController).
		Name("etcdController").
		Export(gs.As[gs.Rooter]())
}

// etcdController is the global singleton. It is ONLY referenced in init
// functions (here and in provider.go). All other code operates on the
// receiver without touching this global.
var etcdController = &etcdCtrl{}

// etcdCtrl is the single object that owns the full lifecycle of etcd
// configuration: loading keys, watching for changes, and triggering
// property refresh.
type etcdCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu       sync.Mutex
	clients  map[string]*clientv3.Client
	listened map[string]struct{}
}

// TriggerRefresh is called by the watch goroutines when a watched key
// changes. Before the IoC container wires the controller, this is a no-op —
// the initial config load already captured the state.
func (c *etcdCtrl) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}
