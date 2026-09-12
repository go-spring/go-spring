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

// This file is the CONSUMER half of Consul service discovery: the
// cloud/discovery Discovery backend serving snapshots of the instances the
// registrar (registrar.go) publishes. It is derived from the
// ${spring.registry.consul} center (center.go) under the fixed label "consul"
// — there is no separate discovery config block. Freshness is internal: each
// resolved service gets a background Consul blocking query (index-based long
// poll) that keeps the cached snapshot current, so Resolve is a cheap read
// after the first call.
//
// Health comes from the query itself: Resolve asks Consul for PASSING
// instances only, so unhealthy instances never enter the snapshot.

package StarterRegistryConsul

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// newConsulDiscovery builds a Discovery backed by a Consul client. tag is the
// service tag narrowing every query (empty spans all tags).
func newConsulDiscovery(client *api.Client, tag string) *consulDiscovery {
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &consulDiscovery{
		client:   client,
		tag:      tag,
		bgCtx:    bgCtx,
		bgCancel: bgCancel,
		entries:  map[string]*serviceEntry{},
	}
}

// consulDiscovery serves snapshots of healthy service instances reported by
// one Consul agent. It implements [discovery.Discovery]. The first Resolve of
// a service seeds a per-service cache and starts a background blocking query
// that keeps it current; later calls are in-memory reads.
type consulDiscovery struct {
	client *api.Client
	tag    string

	// bgCtx anchors the background blocking queries for the backend's
	// lifetime; bgCancel ends them on Close.
	bgCtx    context.Context
	bgCancel context.CancelFunc

	mu      sync.Mutex // guards entries
	entries map[string]*serviceEntry
}

// serviceEntry is the cached snapshot for one service name. eps holds the FULL
// (unfiltered) set; scheme narrowing happens per Resolve call.
type serviceEntry struct {
	mu     sync.Mutex // guards eps; held across the seed fetch so it runs once
	eps    []discovery.Endpoint
	seeded bool
}

// Close stops the background blocking queries. It is NOT a bean destructor
// (nothing in the container calls it); tests use it to release goroutines.
// After Close, Resolve still serves the last cached snapshot.
func (d *consulDiscovery) Close() {
	if d != nil && d.bgCancel != nil {
		d.bgCancel()
	}
}

// entry returns (creating if needed) the cache entry for name.
func (d *consulDiscovery) entry(name string) *serviceEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.entries == nil {
		d.entries = map[string]*serviceEntry{}
	}
	e, ok := d.entries[name]
	if !ok {
		e = &serviceEntry{}
		d.entries[name] = e
	}
	return e
}

// fetch reads the current healthy instance set for name (optionally narrowed
// by tag) from Consul. passingOnly=true keeps unhealthy instances out of the
// snapshot entirely — the package health contract (unhealthy instances do not
// enter the snapshot) is enforced server-side.
func (d *consulDiscovery) fetch(ctx context.Context, name, tag string) ([]discovery.Endpoint, error) {
	entries, _, err := d.client.Health().Service(name, tag, true, (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return serviceEntriesToEndpoints(entries), nil
}

// Resolve returns the current healthy instance set for name. The first call
// pays the seed query (bounded by ctx) and starts the background blocking
// query; later calls read the cache. A per-call WithTag option narrows that
// one lookup server-side (a one-off query, since the cache is untagged).
func (d *consulDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	q := discovery.NewQuery(name, opts...)
	if q.Tag != "" && q.Tag != d.tag {
		eps, err := d.fetch(ctx, name, q.Tag)
		if err != nil {
			return nil, errutil.Explain(err, "registry-consul: query %q (tag %q) failed", name, q.Tag)
		}
		return discovery.FilterByScheme(eps, q.Scheme), nil
	}
	e := d.entry(name)
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.seeded {
		eps, err := d.fetch(ctx, name, d.tag)
		if err != nil {
			discovery.Synced(obsSystem, name, err)
			return nil, errutil.Explain(err, "registry-consul: query %q failed", name)
		}
		e.eps, e.seeded = eps, true
		// The seed is this service's first confirmed snapshot; reporting it
		// starts the freshness clock before any watch event arrives.
		discovery.Synced(obsSystem, name, nil)
		go d.watchLoop(name, e)
	}
	return discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), q.Scheme), nil
}

// watchLoop keeps a service's cache current with a Consul blocking query:
// each call carries the index of the snapshot it last saw and blocks server-
// side until that index advances (or WaitTime elapses), then delivers the
// fresh full snapshot. A failed query keeps the stale snapshot and retries —
// stale addresses are safer than none. It runs until the backend's lifetime
// context ends.
func (d *consulDiscovery) watchLoop(name string, e *serviceEntry) {
	var idx uint64
	for d.bgCtx.Err() == nil {
		q := (&api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: idx}).WithContext(d.bgCtx)
		entries, meta, err := d.client.Health().Service(name, d.tag, true, q)
		if err != nil {
			if d.bgCtx.Err() != nil {
				return
			}
			discovery.Synced(obsSystem, name, err)
			log.Warnf(context.Background(), starterTag, "registry-consul: watch %q failed (keeping stale snapshot): %v", name, err)
			select {
			case <-d.bgCtx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		// A smaller index than we hold means the agent restarted (its index
		// counter reset); drop ours so the next query is answered immediately.
		if meta.LastIndex < idx {
			idx = 0
			continue
		}
		idx = meta.LastIndex
		eps := serviceEntriesToEndpoints(entries)
		e.mu.Lock()
		e.eps = eps
		e.mu.Unlock()
		discovery.Synced(obsSystem, name, nil)
	}
}

// serviceEntriesToEndpoints maps one Consul health response to discovery
// endpoints. The query already filtered to passing instances, so every entry
// is healthy; the advertised passing weight becomes the endpoint weight; an
// optional "scheme" meta key carries transport selection, mirroring the
// etcd/nacos adapters. A service with no address of its own falls back to its
// node's address (the classic Consul catalog idiom).
func serviceEntriesToEndpoints(entries []*api.ServiceEntry) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(entries))
	for _, se := range entries {
		svc := se.Service
		if svc == nil {
			continue
		}
		host := svc.Address
		if host == "" && se.Node != nil {
			host = se.Node.Address
		}
		eps = append(eps, discovery.Endpoint{
			Addr:     fmt.Sprintf("%s:%d", host, svc.Port),
			Scheme:   svc.Meta["scheme"],
			Weight:   svc.Weights.Passing,
			Healthy:  true,
			Metadata: svc.Meta,
		})
	}
	sortEndpoints(eps)
	return eps
}

// sortEndpoints orders endpoints by address so snapshots are comparable.
func sortEndpoints(eps []discovery.Endpoint) {
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
}
