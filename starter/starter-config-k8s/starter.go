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
	// Register the k8s controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "k8s" config
	// provider (so Load calls go through its method). The controller owns the
	// informer lifecycle via its destructor. Before wiring, TriggerRefresh is
	// a harmless no-op — the startup load already captured the initial config.
	gs.Provide(k8sController).
		Name("k8sController").
		Export(gs.As[gs.Rooter]()).
		Destroy((*k8sCtrl).stop)
}

// k8sController is the global singleton. It is ONLY referenced in init
// functions (here and in provider.go). All other code operates on the
// receiver without touching this global.
var k8sController = &k8sCtrl{}

// k8sCtrl is the single object that owns the full lifecycle of k8s
// configuration: loading ConfigMaps/Secrets, watching via informers, and
// triggering property refresh.
type k8sCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	// manager tracks informers so they can be stopped on shutdown.
	manager *watchManager

	onTrigger func() // test hook; nil in production
}

// TriggerRefresh is called by the informer event handlers when a watched
// ConfigMap or Secret changes. Before the IoC container wires the controller,
// this is a no-op — the initial config load already captured the state.
func (c *k8sCtrl) TriggerRefresh() {
	if c.onTrigger != nil {
		c.onTrigger()
		return
	}
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}

// stop tears down every informer. It is the bean destructor, invoked once by
// the container on shutdown.
func (c *k8sCtrl) stop() {
	if c.manager != nil {
		c.manager.stopAll()
	}
}
