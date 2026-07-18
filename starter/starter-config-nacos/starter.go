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

// Package StarterConfigNacos integrates Nacos as a remote configuration
// center for Go-Spring. Blank-importing this package registers a "nacos"
// config provider (see provider.go) that can be consumed via
// spring.app.imports, together with the bridge that wires remote config
// changes into the application-wide property refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery
// (Nacos naming) is a separate concern and is not provided here.
package StarterConfigNacos

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Register the refresh bridge as a root object so it is always created.
	// It links the "nacos" remote config provider's change listener to the
	// application's property refresh, enabling hot-reload of bound beans. A
	// stable name keeps it from colliding with the application's own root beans,
	// which also export gs.Rooter (an alias for any) under the default bean name.
	gs.Provide(newConfigRefreshBridge).
		Name("nacosConfigRefreshBridge").
		Export(gs.As[gs.Rooter]())
}

// configRefreshBridge connects remote Nacos config changes to the
// application-wide property refresh mechanism.
type configRefreshBridge struct{}

// newConfigRefreshBridge installs the refresh hook used by the "nacos" config
// provider. It injects the framework's PropertiesRefresher so that a remote
// config change reloads all sources and updates bound gs.Dync fields.
func newConfigRefreshBridge(r *gs.PropertiesRefresher) *configRefreshBridge {
	setRefreshHook(r.RefreshProperties)
	return &configRefreshBridge{}
}
