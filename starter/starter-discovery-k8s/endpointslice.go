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

package StarterDiscoveryK8s

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"go-spring.org/spring/discovery"
)

// serviceNameLabel is the well-known label Kubernetes sets on every
// EndpointSlice, naming the Service that owns it. Filtering on it selects the
// slices for one Service.
const serviceNameLabel = "kubernetes.io/service-name"

// endpointSliceDiscovery resolves a Service through its EndpointSlices using a
// client-go informer. Compared with dns mode it is real-time (informer events
// fire on scale up/down) and carries per-endpoint metadata (zone, ready state),
// at the cost of a client-go dependency and get/list/watch RBAC on
// endpointslices.
type endpointSliceDiscovery struct {
	cfg    Config
	client kubernetes.Interface

	mu       sync.Mutex
	watchers map[*endpointSliceWatcher]struct{}
}

// newEndpointSliceDiscovery builds a client (in-cluster when Kubeconfig is
// empty, otherwise from the kubeconfig file) and returns an informer-backed
// backend. The client is built eagerly so a missing ServiceAccount or bad
// kubeconfig fails at startup.
func newEndpointSliceDiscovery(cfg Config) (*endpointSliceDiscovery, error) {
	restCfg, err := buildRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("discovery-k8s: build clientset: %w", err)
	}
	return &endpointSliceDiscovery{
		cfg:      cfg,
		client:   client,
		watchers: map[*endpointSliceWatcher]struct{}{},
	}, nil
}

// buildRESTConfig selects in-cluster config or an explicit kubeconfig file.
func buildRESTConfig(cfg Config) (*rest.Config, error) {
	if cfg.Kubeconfig != "" {
		c, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("discovery-k8s: load kubeconfig %q: %w", cfg.Kubeconfig, err)
		}
		return c, nil
	}
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("discovery-k8s: in-cluster config (set kubeconfig when running outside a cluster): %w", err)
	}
	return c, nil
}

// selector builds the label selector that scopes list/watch to one Service.
func (d *endpointSliceDiscovery) selector(name string) string {
	return labels.SelectorFromSet(labels.Set{serviceNameLabel: name}).String()
}

// Resolve lists the Service's EndpointSlices once and flattens them into the
// current endpoint set.
func (d *endpointSliceDiscovery) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	list, err := d.client.DiscoveryV1().EndpointSlices(d.cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: d.selector(name),
	})
	if err != nil {
		return nil, fmt.Errorf("discovery-k8s: list endpointslices for %q: %w", name, err)
	}
	slices := make([]*discoveryv1.EndpointSlice, 0, len(list.Items))
	for i := range list.Items {
		slices = append(slices, &list.Items[i])
	}
	eps := slicesToEndpoints(d.cfg, slices)
	sortEndpoints(eps)
	return eps, nil
}

// Watch starts an informer scoped to the Service's EndpointSlices and pushes a
// fresh snapshot on every add/update/delete. The caller owns Stop, which tears
// the informer down.
func (d *endpointSliceDiscovery) Watch(ctx context.Context, name string) (discovery.Watcher, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		d.client,
		d.cfg.ResyncPeriod,
		informers.WithNamespace(d.cfg.Namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = d.selector(name)
		}),
	)
	informer := factory.Discovery().V1().EndpointSlices().Informer()
	lister := factory.Discovery().V1().EndpointSlices().Lister().EndpointSlices(d.cfg.Namespace)

	w := &endpointSliceWatcher{
		parent: d,
		cfg:    d.cfg,
		name:   name,
		lister: lister,
		ch:     make(chan []discovery.Endpoint, 1),
		done:   make(chan struct{}),
	}

	// Every store mutation recomputes the snapshot from the informer cache and
	// pushes the latest (a full snapshot, coalescing bursts via emit).
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { w.emit() },
		UpdateFunc: func(any, any) { w.emit() },
		DeleteFunc: func(any) { w.emit() },
	}
	if _, err := informer.AddEventHandler(handler); err != nil {
		return nil, fmt.Errorf("discovery-k8s: add informer handler for %q: %w", name, err)
	}

	factory.Start(w.done)
	if !cache.WaitForCacheSync(w.done, informer.HasSynced) {
		w.Stop()
		return nil, fmt.Errorf("discovery-k8s: cache sync failed for %q", name)
	}

	d.mu.Lock()
	d.watchers[w] = struct{}{}
	d.mu.Unlock()
	return w, nil
}

// Close stops any watchers still running, a safety net for shutdown when a
// consumer forgot to Stop its watcher.
func (d *endpointSliceDiscovery) Close() error {
	d.mu.Lock()
	ws := make([]*endpointSliceWatcher, 0, len(d.watchers))
	for w := range d.watchers {
		ws = append(ws, w)
	}
	d.mu.Unlock()
	for _, w := range ws {
		_ = w.Stop()
	}
	return nil
}

// endpointSliceWatcher streams snapshots computed from an informer cache.
type endpointSliceWatcher struct {
	parent *endpointSliceDiscovery
	cfg    Config
	name   string
	lister interface {
		List(selector labels.Selector) ([]*discoveryv1.EndpointSlice, error)
	}
	ch       chan []discovery.Endpoint
	done     chan struct{}
	stopOnce sync.Once
}

// emit recomputes the snapshot from the informer cache and delivers the latest
// to Next, dropping a stale queued snapshot so the consumer always sees current
// state rather than a backlog.
func (w *endpointSliceWatcher) emit() {
	slices, err := w.lister.List(labels.Everything())
	if err != nil {
		return
	}
	eps := slicesToEndpoints(w.cfg, slices)
	sortEndpoints(eps)
	// Coalesce: drop a pending snapshot before queuing the newest.
	select {
	case <-w.ch:
	default:
	}
	select {
	case w.ch <- eps:
	case <-w.done:
	}
}

func (w *endpointSliceWatcher) Next() ([]discovery.Endpoint, error) {
	select {
	case eps := <-w.ch:
		return eps, nil
	case <-w.done:
		return nil, context.Canceled
	}
}

func (w *endpointSliceWatcher) Stop() error {
	w.stopOnce.Do(func() {
		close(w.done)
		if w.parent != nil {
			w.parent.mu.Lock()
			delete(w.parent.watchers, w)
			w.parent.mu.Unlock()
		}
	})
	return nil
}

// slicesToEndpoints flattens EndpointSlices into discovery endpoints, selecting
// the port per Config and carrying ready/zone as health and metadata.
func slicesToEndpoints(cfg Config, slices []*discoveryv1.EndpointSlice) []discovery.Endpoint {
	var eps []discovery.Endpoint
	for _, sl := range slices {
		port, ok := pickPort(cfg, sl.Ports)
		if !ok {
			continue
		}
		portStr := strconv.Itoa(port)
		for i := range sl.Endpoints {
			e := &sl.Endpoints[i]
			ready := e.Conditions.Ready == nil || *e.Conditions.Ready
			var md map[string]string
			if e.Zone != nil && *e.Zone != "" {
				md = map[string]string{"zone": *e.Zone}
			}
			for _, addr := range e.Addresses {
				eps = append(eps, discovery.Endpoint{
					Addr:     net.JoinHostPort(addr, portStr),
					Healthy:  ready,
					Metadata: md,
				})
			}
		}
	}
	return eps
}

// pickPort chooses the numeric port for a slice: the named port when PortName
// is set, else the configured Port, else the slice's sole port. It returns
// false when no port can be determined so the slice is skipped rather than
// yielding a bogus ":0" address.
func pickPort(cfg Config, ports []discoveryv1.EndpointPort) (int, bool) {
	if cfg.PortName != "" {
		for _, p := range ports {
			if p.Name != nil && *p.Name == cfg.PortName && p.Port != nil {
				return int(*p.Port), true
			}
		}
		return 0, false
	}
	if cfg.Port > 0 {
		return cfg.Port, true
	}
	if len(ports) == 1 && ports[0].Port != nil {
		return int(*ports[0].Port), true
	}
	return 0, false
}
