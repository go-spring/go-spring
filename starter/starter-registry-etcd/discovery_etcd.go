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

// This file is the CONSUMER half of etcd service discovery: the
// cloud/discovery Discovery backend serving snapshots of the instances the
// registrar (registrar.go) publishes. It is derived from the
// ${spring.registry.etcd} center (center.go) under the fixed label "etcd" —
// there is no separate discovery config block. Freshness is internal: each
// resolved service gets a background etcd watcher that keeps the cached
// snapshot current, so Resolve is a cheap read after the first call.
//
// Health is derived from key liveness: an instance key only exists while its
// lease is alive (the registrar's keep-alive renews it, process death lets it
// expire), so every key found under the service prefix is a live instance.

package StarterRegistryEtcd

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdDiscovery serves snapshots of service instances stored under one etcd
// prefix. It implements [discovery.Discovery]. The
// first Resolve of a service seeds a per-service cache and starts a background
// etcd watcher that keeps it current; later calls are in-memory reads.
type etcdDiscovery struct {
	client    *clientv3.Client
	keyPrefix string

	// bgCtx anchors the background watchers for the backend's lifetime.
	bgCtx context.Context

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

// servicePrefix returns the etcd key prefix holding name's instances:
// keyPrefix + name + "/" — the layout etcdRegistrar.keyFor writes.
func (d *etcdDiscovery) servicePrefix(name string) string {
	return d.keyPrefix + name + "/"
}

// entry returns (creating if needed) the cache entry for name.
func (d *etcdDiscovery) entry(name string) *serviceEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[name]
	if !ok {
		e = &serviceEntry{}
		d.entries[name] = e
	}
	return e
}

// fetch reads the current instance set for prefix from etcd.
func (d *etcdDiscovery) fetch(ctx context.Context, prefix string) ([]discovery.Endpoint, error) {
	resp, err := d.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	return kvsToEndpoints(resp.Kvs), nil
}

// Resolve returns the current instance set for name. A key's existence is the
// health signal: leases delete expired keys, so everything found is live. The
// first call pays the seed fetch (bounded by ctx) and starts the background
// watcher; later calls read the cache.
func (d *etcdDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	e := d.entry(name)
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.seeded {
		eps, err := d.fetch(ctx, d.servicePrefix(name))
		if err != nil {
			return nil, errutil.Explain(err, "registry-etcd: get %q failed", name)
		}
		e.eps, e.seeded = eps, true
		go d.watchLoop(d.servicePrefix(name), e)
	}
	eps := discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), discovery.NewQuery("", opts...).Scheme)
	return eps, nil
}

// watchLoop refreshes a service's cache on every etcd change below prefix
// until the backend's lifetime context ends. A failed refresh keeps the stale
// snapshot — stale addresses are safer than none.
func (d *etcdDiscovery) watchLoop(prefix string, e *serviceEntry) {
	wch := d.client.Watch(d.bgCtx, prefix, clientv3.WithPrefix())
	for range wch {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		eps, err := d.fetch(ctx, prefix)
		cancel()
		if err != nil {
			log.Warnf(context.Background(), starterTag, "registry-etcd: refresh %q failed (keeping stale snapshot): %v", prefix, err)
			continue
		}
		e.mu.Lock()
		e.eps = eps
		e.mu.Unlock()
	}
}

// kvsToEndpoints maps one etcd Get/watch response's key-value pairs to
// discovery endpoints, decoding the instanceValue JSON the registrar writes.
// Every key found is healthy (lease liveness) and enabled (the registrar has
// no disabled state); an optional "scheme" metadata key carries transport
// selection, mirroring the nacos adapter.
func kvsToEndpoints(kvs []*mvccpb.KeyValue) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(kvs))
	for _, kv := range kvs {
		var v instanceValue
		if err := json.Unmarshal(kv.Value, &v); err != nil {
			// A malformed payload is one bad instance, not a broken snapshot.
			log.Warnf(context.Background(), starterTag, "registry-etcd: skip malformed instance %q: %v", string(kv.Key), err)
			continue
		}
		eps = append(eps, discovery.Endpoint{
			Addr:     v.Addr,
			Scheme:   v.Metadata["scheme"],
			Weight:   v.Weight,
			Healthy:  true,
			Metadata: v.Metadata,
		})
	}
	sortEndpoints(eps)
	return eps
}

// sortEndpoints orders endpoints by address so snapshots are comparable.
func sortEndpoints(eps []discovery.Endpoint) {
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
}

// endpointsKey renders a snapshot as a comparable string for change detection.
func endpointsKey(eps []discovery.Endpoint) string {
	var b strings.Builder
	for _, e := range eps {
		b.WriteString(e.Addr)
		b.WriteByte(',')
		b.WriteString(e.Scheme)
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(e.Weight))
		b.WriteByte(';')
	}
	return b.String()
}
