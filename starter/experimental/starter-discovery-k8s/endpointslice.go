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

	"go-spring.org/spring/cloud/discovery"
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
	watchers map[*watcherHandle]struct{}
}

// watcherHandle is the bookkeeping for one live watch: done closes to stop the
// informer factory and the result channel (exactly once, via stopOnce). Tracked
// by the parent so Close can tear every watch down as a shutdown safety net.
type watcherHandle struct {
	done     chan struct{}
	stopOnce sync.Once
}

func (h *watcherHandle) stop() {
	h.stopOnce.Do(func() { close(h.done) })
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
		watchers: map[*watcherHandle]struct{}{},
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
// fresh full snapshot on the returned channel on every add/update/delete. The
// first result carries the snapshot at cache-sync time; the channel closes when
// ctx is cancelled or Close stops the backend.
func (d *endpointSliceDiscovery) Watch(ctx context.Context, name string) (<-chan discovery.WatchResult, error) {
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

	// Informer handlers only signal a change; they never touch the result
	// channel. A single goroutine (below) owns the channel — it is the sole
	// writer and the sole closer, so there is no send-after-close race.
	updates := make(chan struct{}, 1)
	enqueue := func() {
		select {
		case updates <- struct{}{}:
		default:
		}
	}
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { enqueue() },
		UpdateFunc: func(any, any) { enqueue() },
		DeleteFunc: func(any) { enqueue() },
	}
	if _, err := informer.AddEventHandler(handler); err != nil {
		return nil, fmt.Errorf("discovery-k8s: add informer handler for %q: %w", name, err)
	}

	h := &watcherHandle{done: make(chan struct{})}
	factory.Start(h.done)
	if !cache.WaitForCacheSync(h.done, informer.HasSynced) {
		h.stop()
		return nil, fmt.Errorf("discovery-k8s: cache sync failed for %q", name)
	}

	cfg := d.cfg
	snapshotCh := make(chan discovery.WatchResult, 1)
	var last string
	snapshot := func() {
		slices, err := lister.List(labels.Everything())
		if err != nil {
			return
		}
		eps := slicesToEndpoints(cfg, slices)
		sortEndpoints(eps)
		// Skip unchanged snapshots: the cache-sync burst fires one Add event per
		// object, which would otherwise queue stale duplicates ahead of a real
		// change (mirrors dnsDiscovery.Watch's addrKey guard).
		if key := addrKey(eps); key == last {
			return
		} else {
			last = key
		}
		select {
		case snapshotCh <- discovery.WatchResult{Endpoints: eps}:
		case <-h.done:
		}
	}

	// Seed the current snapshot before the writer goroutine starts, so the first
	// result is the state at watch time.
	snapshot()

	d.mu.Lock()
	d.watchers[h] = struct{}{}
	d.mu.Unlock()

	go func() {
		defer close(snapshotCh)
		defer d.untrack(h)
		for {
			select {
			case <-ctx.Done():
				h.stop()
				return
			case <-h.done:
				return // Close() stopped this watch.
			case <-updates:
				snapshot()
			}
		}
	}()
	return snapshotCh, nil
}

// untrack removes a watch handle from the parent's tracking set.
func (d *endpointSliceDiscovery) untrack(h *watcherHandle) {
	d.mu.Lock()
	delete(d.watchers, h)
	d.mu.Unlock()
}

// Close stops every still-running watch, a safety net for shutdown when a
// consumer forgot to cancel its watch context.
func (d *endpointSliceDiscovery) Close() error {
	d.mu.Lock()
	ws := make([]*watcherHandle, 0, len(d.watchers))
	for h := range d.watchers {
		ws = append(ws, h)
	}
	d.mu.Unlock()
	for _, h := range ws {
		h.stop()
	}
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
