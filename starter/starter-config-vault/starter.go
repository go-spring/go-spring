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

// Package StarterConfigVault integrates HashiCorp Vault as a remote
// configuration center for Go-Spring. Blank-importing this package registers a
// "vault" config provider that can be consumed via spring.app.imports, together
// with the bridge that wires secret changes into the application-wide property
// refresh for live hot-reload.
//
// This starter covers the config-center role only: it reads a KV secret and
// exposes its fields as application properties. A Vault Agent / CSI-mounted
// secret file is read with starter-config-file instead; this starter talks to
// the Vault API directly.
package StarterConfigVault

import (
	"sync"

	"github.com/hashicorp/vault/api"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the vault controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "vault" config
	// provider (so Load calls go through its method). Before wiring,
	// TriggerRefresh is a harmless no-op — the startup load already captured
	// the initial config.
	gs.Provide(vaultController).
		Name("vaultController").
		Export(gs.As[gs.Rooter]())
}

// vaultController is the global singleton. It is ONLY referenced in init
// functions (here and in provider.go). All other code operates on the
// receiver without touching this global.
var vaultController = &vaultCtrl{}

// vaultCtrl is the single object that owns the full lifecycle of vault
// configuration: loading secrets, polling for changes, and triggering
// property refresh.
type vaultCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu       sync.Mutex
	clients  map[string]*api.Client
	listened map[string]struct{}
	loadedFP map[string]string // fingerprint of last loaded data
}

// TriggerRefresh is called by the polling watchers when a secret's content
// fingerprint changes. Before the IoC container wires the controller, this
// is a no-op — the initial config load already captured the state.
func (c *vaultCtrl) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}
