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

// This file is the CONSUMER half of Nacos service discovery: the
// cloud/discovery Discovery backend serving snapshots of instances from Nacos
// naming. It is derived from the ${spring.registry.nacos} center (center.go)
// under the fixed label "nacos" — there is no separate discovery config block;
// read and write share the center's namespace/group/cluster, so they can never
// diverge. Freshness is internal: the first Resolve of a service subscribes to
// Nacos pushes that keep the cached snapshot current, so later calls are
// in-memory reads.

package StarterRegistryNacos

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// nacosDiscovery serves snapshots of service instances in one Nacos
// namespace/group. It implements [discovery.Discovery]. The first Resolve of a
// service seeds a per-service cache and subscribes to Nacos pushes that keep
// it current; later calls are in-memory reads.
type nacosDiscovery struct {
	client  naming_client.INamingClient
	group   string
	cluster string

	mu      sync.Mutex // guards entries
	entries map[string]*nacosEntry
}

// nacosEntry is the cached snapshot for one service name. eps holds the FULL
// (unfiltered) set; scheme narrowing happens per Resolve call.
type nacosEntry struct {
	mu     sync.Mutex // guards eps; held across the seed so subscribe runs once
	eps    []discovery.Endpoint
	seeded bool
}

// newNacosDiscovery returns the read side of one block, backed by client. The
// cache map is created here: a caller that builds the struct literally leaves
// it nil and panics on the first Resolve.
func newNacosDiscovery(client naming_client.INamingClient, group, cluster string) *nacosDiscovery {
	return &nacosDiscovery{
		client:  client,
		group:   group,
		cluster: cluster,
		entries: map[string]*nacosEntry{},
	}
}

// entry returns (creating if needed) the cache entry for name.
func (d *nacosDiscovery) entry(name string) *nacosEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[name]
	if !ok {
		e = &nacosEntry{}
		d.entries[name] = e
	}
	return e
}

// Resolve returns the current healthy instance set for name. The first call
// pays the seed query (bounded by ctx) and opens the Nacos subscription; later
// calls read the cache.
func (d *nacosDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	e := d.entry(name)
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.seeded {
		instances, err := d.selectInstances(ctx, name)
		if err != nil {
			discovery.Synced(obsSystem, name, err)
			return nil, err
		}
		// A failed subscription means the cache can never be kept current, so
		// it counts as a failed sync rather than a merely partial seed.
		if err := d.subscribe(name, e); err != nil {
			discovery.Synced(obsSystem, name, err)
			return nil, err
		}
		e.eps = instancesToEndpoints(instances)
		sortEndpoints(e.eps)
		e.seeded = true
		// The seed is this service's first confirmed snapshot; reporting it
		// starts the freshness clock before any push arrives.
		discovery.Synced(obsSystem, name, nil)
	}
	eps := discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), discovery.NewQuery("", opts...).Scheme)
	return eps, nil
}

// clusterList returns the configured cluster as a filter, or nil when the
// cluster is empty (span all clusters).
func (d *nacosDiscovery) clusterList() []string {
	if d.cluster == "" {
		return nil
	}
	return []string{d.cluster}
}

// selectInstances queries Nacos for name's healthy instances within the
// configured group (and cluster, when set).
func (d *nacosDiscovery) selectInstances(_ context.Context, name string) ([]model.Instance, error) {
	p := vo.SelectInstancesParam{}
	p.ServiceName = name
	p.GroupName = d.group
	p.Clusters = d.clusterList()
	p.HealthyOnly = true
	instances, err := d.client.SelectInstances(p)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: select %s in %s failed", name, d.group)
	}
	return instances, nil
}

// subscribe registers e's Nacos push callback, which refreshes the cache on
// every push. A failed push keeps the stale snapshot — stale addresses are
// safer than none. The subscription lives for the backend's lifetime.
func (d *nacosDiscovery) subscribe(name string, e *nacosEntry) error {
	cb := func(services []model.Instance, err error) {
		if err != nil {
			discovery.Synced(obsSystem, name, err)
			log.Warnf(context.Background(), starterTag, "registry-nacos: push for %s failed (keeping last snapshot): %v", name, err)
			return
		}
		eps := instancesToEndpoints(services)
		sortEndpoints(eps)
		e.mu.Lock()
		e.eps = eps
		e.mu.Unlock()
		discovery.Synced(obsSystem, name, nil)
	}
	return d.client.Subscribe(&vo.SubscribeParam{
		ServiceName: name, GroupName: d.group, Clusters: d.clusterList(), SubscribeCallback: cb,
	})
}

// instancesToEndpoints maps Nacos instances to discovery endpoints. Enable
// maps to Disabled inverted (Nacos "enabled=false" is an operator removal);
// an optional "scheme" metadata key carries transport selection.
func instancesToEndpoints(instances []model.Instance) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(instances))
	for _, in := range instances {
		eps = append(eps, discovery.Endpoint{
			Addr:     fmt.Sprintf("%s:%d", in.Ip, in.Port),
			Scheme:   in.Metadata["scheme"],
			Weight:   int(in.Weight),
			Disabled: !in.Enable,
			Healthy:  in.Healthy,
			// The full metadata rides along, as in the other backends: zone /
			// version attributes are what zone-aware selection reads.
			Metadata: in.Metadata,
		})
	}
	return eps
}

// sortEndpoints orders endpoints by address so snapshots are comparable.
func sortEndpoints(eps []discovery.Endpoint) {
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
}
