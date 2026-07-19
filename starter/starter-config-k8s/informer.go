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
	"fmt"
	"sync"
	"sync/atomic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"go-spring.org/stdlib/errutil"
)

// k8sClient is the subset of the Kubernetes API used here. It is
// kubernetes.Interface so both the real clientset and the client-go fake
// clientset (used in tests) satisfy it without a live cluster.
type k8sClient = kubernetes.Interface

// buildClient builds a clientset: in-cluster when kubeconfig is empty, otherwise
// from the kubeconfig file. Built eagerly so a missing ServiceAccount or bad
// kubeconfig surfaces at config-load time.
func buildClient(kubeconfig string) (k8sClient, error) {
	var (
		cfg *rest.Config
		err error
	)
	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: load kubeconfig %q", kubeconfig)
		}
	} else {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, errutil.Explain(err, "k8s config: in-cluster config (set kubeconfig when running outside a cluster)")
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errutil.Explain(err, "k8s config: build clientset")
	}
	return client, nil
}

// refreshHook holds the callback used to reload application properties when a
// watched object changes. It is populated by the refresh bridge bean during
// container wiring (see starter.go). A change that arrives before the bridge is
// wired is safely ignored; the value is picked up on the next refresh.
var refreshHook atomic.Pointer[func() error]

// setRefreshHook installs the callback that reloads application properties.
func setRefreshHook(fn func() error) {
	refreshHook.Store(&fn)
}

// triggerRefresh invokes the installed refresh callback, if any.
func triggerRefresh() {
	if p := refreshHook.Load(); p != nil {
		_ = (*p)()
	}
}

// watchManager tracks the informers started by this provider so they can be
// stopped on container shutdown, and deduplicates watchers so repeated Load
// calls (startup + every refresh) do not stack informers on the same object.
type watchManager struct {
	mu      sync.Mutex
	watched map[string]struct{}
	stops   []chan struct{}
}

var manager = &watchManager{watched: map[string]struct{}{}}

// ensureWatch starts a namespaced, name-scoped informer on the target object and
// triggers a full property refresh on every add/update/delete. It is
// deduplicated per kind/namespace/name and best-effort: an informer that fails
// to sync is torn down and the object simply loses hot-reload.
func ensureWatch(client k8sClient, cs configSource) {
	id := fmt.Sprintf("%s/%s/%s", cs.kind, cs.namespace, cs.name)

	manager.mu.Lock()
	if _, ok := manager.watched[id]; ok {
		manager.mu.Unlock()
		return
	}
	manager.watched[id] = struct{}{}
	manager.mu.Unlock()

	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0, // event-driven only; no periodic resync needed for a single object
		informers.WithNamespace(cs.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = "metadata.name=" + cs.name
		}),
	)

	var informer cache.SharedIndexInformer
	switch cs.kind {
	case kindConfigMap:
		informer = factory.Core().V1().ConfigMaps().Informer()
	case kindSecret:
		informer = factory.Core().V1().Secrets().Informer()
	default:
		return
	}

	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { triggerRefresh() },
		UpdateFunc: func(any, any) { triggerRefresh() },
		DeleteFunc: func(any) { triggerRefresh() },
	}
	if _, err := informer.AddEventHandler(handler); err != nil {
		manager.forget(id)
		return
	}

	stop := make(chan struct{})
	factory.Start(stop)
	if !cache.WaitForCacheSync(stop, informer.HasSynced) {
		close(stop)
		manager.forget(id)
		return
	}

	manager.mu.Lock()
	manager.stops = append(manager.stops, stop)
	manager.mu.Unlock()
}

// forget drops a watcher id so a later Load may retry starting it.
func (m *watchManager) forget(id string) {
	m.mu.Lock()
	delete(m.watched, id)
	m.mu.Unlock()
}

// stopAll stops every running informer. It is invoked by the refresh bridge
// bean's destructor on container shutdown.
func (m *watchManager) stopAll() {
	m.mu.Lock()
	stops := m.stops
	m.stops = nil
	m.watched = map[string]struct{}{}
	m.mu.Unlock()
	for _, s := range stops {
		close(s)
	}
}
