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

package StarterConfigK8s

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Register the refresh bridge as a root object so it is always created. It
	// links the "k8s" provider's informer-driven changes to the application's
	// property refresh, enabling hot-reload of bound beans, and stops the
	// informers on shutdown via its destructor. A stable name keeps it from
	// colliding with the application's own root beans, which also export
	// gs.Rooter (an alias for any) under the default bean name.
	gs.Provide(newConfigRefreshBridge).
		Name("configK8sRefreshBridge").
		Export(gs.As[gs.Rooter]()).
		Destroy((*configRefreshBridge).stop)
}

// configRefreshBridge connects k8s config changes to the application-wide
// property refresh mechanism and owns the informer lifecycle.
type configRefreshBridge struct{}

// newConfigRefreshBridge installs the refresh hook used by the "k8s" config
// provider. It injects the framework's PropertiesRefresher so that an object
// change reloads all sources and updates bound gs.Dync fields.
func newConfigRefreshBridge(r *gs.PropertiesRefresher) *configRefreshBridge {
	setRefreshHook(r.RefreshProperties)
	return &configRefreshBridge{}
}

// stop tears down every informer started by the provider. It is the bean
// destructor, invoked once by the container on shutdown.
func (*configRefreshBridge) stop() {
	manager.stopAll()
}
