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
// package registers a "file-watch" config provider (see provider.go) that can
// be consumed via spring.app.imports, together with the bridge that wires file
// changes into the application-wide property refresh for live hot-reload.
//
// Its primary purpose is Kubernetes: a ConfigMap or Secret mounted as a volume
// becomes hot-reloadable without any custom code. The kubelet updates such a
// volume by atomically swapping the "..data" symlink, which the provider's
// directory watcher (see watch.go) detects and turns into a refresh, so bound
// gs.Dync fields update within seconds of `kubectl edit configmap`.
//
// This starter covers local file/volume watching only. Remote configuration
// centers (Nacos, etcd, Consul) are separate starters.
package StarterConfigFile

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Register the refresh bridge as a root object so it is always created.
	// It links the "file-watch" provider's change watcher to the application's
	// property refresh, enabling hot-reload of bound beans. A stable name keeps
	// it from colliding with the application's own root beans, which also export
	// gs.Rooter (an alias for any) under the default bean name.
	gs.Provide(newConfigRefreshBridge).
		Name("configFileRefreshBridge").
		Export(gs.As[gs.Rooter]())
}

// configRefreshBridge connects file-watch config changes to the
// application-wide property refresh mechanism.
type configRefreshBridge struct{}

// newConfigRefreshBridge installs the refresh hook used by the "file-watch"
// config provider. It injects the framework's PropertiesRefresher so that a
// file change reloads all sources and updates bound gs.Dync fields.
func newConfigRefreshBridge(r *gs.PropertiesRefresher) *configRefreshBridge {
	setRefreshHook(r.RefreshProperties)
	return &configRefreshBridge{}
}
