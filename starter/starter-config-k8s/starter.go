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
	"context"
	"sync"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
)

func init() {
	// Register the controller itself, not just its Load method: the runtime
	// holds the registered provider so it can stop the informers at shutdown
	// (see provider.Provider). Refreshes go through the gs.RefreshProperties
	// package-level facade, so the controller needs no bean wiring at all.
	conf.RegisterProvider("k8s", newK8sCtrl())
}

var starterTag = log.RegisterAppTag("config_k8s", "")

// k8sCtrl is the single object that owns the full lifecycle of k8s
// configuration: loading ConfigMaps/Secrets, watching via informers, and
// triggering property refresh.
type k8sCtrl struct {
	// manager tracks informers so they can be stopped on shutdown.
	manager *watchManager

	// clientMu guards clients, the clientset cache keyed by kubeconfig path.
	// Load runs on every property refresh; without the cache each refresh
	// would build a fresh clientset (re-reading the in-cluster config, and
	// leaving the old connection pool behind) instead of reusing one.
	clientMu sync.Mutex
	clients  map[string]k8sClient

	onTrigger func() // test hook; nil in production
}

// newK8sCtrl creates a controller with its watch manager and client cache
// ready, so neither ensureWatch nor clientFor needs a lazy nil-check.
func newK8sCtrl() *k8sCtrl {
	return &k8sCtrl{
		manager: &watchManager{watched: map[string]struct{}{}},
		clients: map[string]k8sClient{},
	}
}

// clientFor returns a cached clientset for the kubeconfig path, creating one
// if necessary. The cache is what keeps repeated refreshes from leaking a
// clientset's connection pool per Load call.
func (c *k8sCtrl) clientFor(kubeconfig string) (k8sClient, error) {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()

	if cli, ok := c.clients[kubeconfig]; ok {
		return cli, nil
	}
	cli, err := buildClient(kubeconfig)
	if err != nil {
		return nil, err
	}
	c.clients[kubeconfig] = cli
	return cli, nil
}

// TriggerRefresh is called by the informer event handlers when a watched
// ConfigMap or Secret changes. Before the app has started,
// gs.RefreshProperties returns an error and the change is dropped — the
// initial config load already captured the state.
func (c *k8sCtrl) TriggerRefresh(ctx context.Context) {
	if c.onTrigger != nil {
		c.onTrigger()
		return
	}
	// The refresh outcome (status, duration, error) is logged and metered
	// centrally by observability.RefreshConf; this layer only records backend events.
	_ = observability.RefreshConf(ctx, gs.RefreshProperties)
}

// Close tears down every informer. It is the provider lifecycle hook, invoked
// once by the runtime on shutdown. A Load that follows re-arms: the watch
// manager forgets its watcher ids, so ensureWatch starts fresh informers.
func (c *k8sCtrl) Close() error {
	if c.manager != nil {
		c.manager.stopAll()
	}
	return nil
}
