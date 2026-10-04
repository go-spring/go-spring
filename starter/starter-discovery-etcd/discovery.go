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
// registry (registry.go) publishes. It is derived from the
// ${spring.discovery.etcd} center (starter.go) under the fixed label "etcd" —
// there is no separate discovery config block. Freshness is internal: each
// resolved service gets a background etcd watcher that keeps the cached
// snapshot current, so Resolve is a cheap read after the first call.
//
// Health is derived from key liveness: an instance key only exists while its
// lease is alive (the registry's keep-alive renews it, process death lets it
// expire), so every key found under the service prefix is a live instance.

package StarterDiscoveryEtcd

import (
	"context"
	"encoding/json"
	"errors"
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

const (
	// watchReArmBase/watchReArmCap pace the watch re-arm after etcd cancels a
	// watch: 1s doubling up to 1min, matching the registry's self-heal pacing.
	watchReArmBase = time.Second
	watchReArmCap  = time.Minute

	// fetchTimeout bounds one snapshot read: the watch loop has no caller
	// context to inherit a deadline from.
	fetchTimeout = 5 * time.Second
)

// errWatchEnded reports a watch channel closed without etcd cancelling it —
// the stream ended for a reason the client did not name.
var errWatchEnded = errors.New("discovery-etcd: watch channel closed")

// etcdDiscovery serves snapshots of service instances stored under one etcd
// prefix. It implements [discovery.Discovery]. The
// first Resolve of a service seeds a per-service cache and starts a background
// etcd watcher that keeps it current; later calls are in-memory reads.
type etcdDiscovery struct {
	// client is the read-side seam (client.go): the production value wraps the
	// backend's *clientv3.Client, tests inject an in-process double.
	client    discoveryKV
	keyPrefix string

	// bgCtx anchors the background watchers for the backend's lifetime.
	bgCtx context.Context

	// obs is the backend block's observability layer; it carries the identity
	// (system, center) every reported sync and log line is labelled with.
	obs *discovery.Observer

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
// keyPrefix + name + "/" — the layout etcdRegistry.keyFor writes.
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
	return kvsToEndpoints(d.obs, resp.Kvs), nil
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
			d.obs.Synced(name, err)
			return nil, errutil.Explain(err, "discovery-etcd: get %q failed", name)
		}
		e.eps, e.seeded = eps, true
		// The seed is this service's first confirmed snapshot; reporting it
		// starts the freshness clock before any watch event arrives.
		d.obs.Synced(name, nil)
		go d.watchLoop(name, e)
	}
	eps := discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), discovery.NewQuery("", opts...).Scheme)
	return eps, nil
}

// watchLoop keeps a service's cache current with an etcd watch and re-arms that
// watch if etcd cancels it — compaction, a revoked permission, or a revision
// etcd can no longer serve. Re-arming is what makes a cancelled watch
// survivable: the stream carries no events and never recovers on its own, so
// without it the cache would freeze at its last value while every gauge stayed
// nominally alive, the failure looking exactly like a quiet cluster. A failed
// refresh keeps the stale snapshot — stale addresses are safer than none — and
// is reported to the freshness metric so the stale window stays visible instead
// of only ever appearing as a warn line.
func (d *etcdDiscovery) watchLoop(name string, e *serviceEntry) {
	// The watched service's identity rides on the context for the life of this
	// watcher: every line below carries it without repeating it. The goroutine
	// outlives any request, so the context is minted here.
	ctx := log.WithFields(context.Background(),
		log.String("system", d.obs.System()),
		log.String("center", d.obs.Center()),
		log.String("service", name),
		log.String("operation", "sync"),
	)
	prefix := d.servicePrefix(name)
	backoff := watchReArmBase
	for d.bgCtx.Err() == nil {
		wch := d.client.Watch(d.bgCtx, prefix, clientv3.WithPrefix())
		err := d.drainWatch(name, e, prefix, wch)
		if d.bgCtx.Err() != nil {
			// The watch ended because the backend is closing, not because it
			// failed: nothing to report and nothing to re-arm.
			return
		}
		d.obs.Synced(name, err)
		log.Error(ctx, starterTag, err,
			log.String("status", discovery.StatusOf(err)),
			log.String("prefix", prefix),
			log.String("backoff", backoff.String()),
			log.Msg("discovery-etcd: watch ended; re-arming"))
		select {
		case <-d.bgCtx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > watchReArmCap {
			backoff = watchReArmCap
		}
	}
}

// drainWatch consumes one watch stream, refreshing the cache on every event,
// and returns why the stream ended: etcd's own reason when it cancelled the
// watch, or errWatchEnded when the channel simply closed.
func (d *etcdDiscovery) drainWatch(name string, e *serviceEntry, prefix string, wch clientv3.WatchChan) error {
	// The watched service's identity rides on the context for the life of this
	// drain: every line below carries it without repeating it. The watcher
	// outlives any request, so the context is minted here.
	ctx := log.WithFields(context.Background(),
		log.String("system", d.obs.System()),
		log.String("center", d.obs.Center()),
		log.String("service", name),
		log.String("operation", "sync"),
	)
	for resp := range wch {
		if err := resp.Err(); err != nil {
			return errutil.Explain(err, "discovery-etcd: watch %q cancelled", prefix)
		}
		fetchCtx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		eps, err := d.fetch(fetchCtx, prefix)
		cancel()
		if err != nil {
			d.obs.Synced(name, err)
			log.Warn(ctx, starterTag,
				log.String("status", discovery.StatusOf(err)),
				log.String("prefix", prefix),
				log.Err(err),
				log.Msg("discovery-etcd: refresh failed (keeping stale snapshot)"))
			continue
		}
		e.mu.Lock()
		e.eps = eps
		e.mu.Unlock()
		d.obs.Synced(name, nil)
	}
	return errWatchEnded
}

// kvsToEndpoints maps one etcd Get/watch response's key-value pairs to
// discovery endpoints, decoding the instanceValue JSON the registry writes.
// Every key found is healthy (lease liveness) and enabled (the registry has
// no disabled state); the payload's scheme field carries transport selection.
func kvsToEndpoints(obs *discovery.Observer, kvs []*mvccpb.KeyValue) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(kvs))
	for _, kv := range kvs {
		var v instanceValue
		if err := json.Unmarshal(kv.Value, &v); err != nil {
			// A malformed payload is one bad discovery.Instance, not a broken snapshot.
			log.Warn(context.Background(), starterTag,
				log.String("system", obs.System()),
				log.String("center", obs.Center()),
				log.String("key", string(kv.Key)),
				log.Err(err),
				log.Msg("discovery-etcd: skip malformed instance payload"))
			continue
		}
		eps = append(eps, discovery.Endpoint{
			Addr:     v.Addr,
			Scheme:   v.Scheme,
			Weight:   v.Weight,
			Healthy:  true,
			Metadata: withDimensions(v.Metadata, v.Version, v.Zone),
		})
	}
	sortEndpoints(eps)
	return eps
}

// withDimensions surfaces the payload's version/zone fields through the
// metadata map consumers route on (zone-aware balancing reads the zone key),
// copying only when something must be added.
func withDimensions(md map[string]string, version, zone string) map[string]string {
	if version == "" && zone == "" {
		return md
	}
	out := make(map[string]string, len(md)+2)
	for k, v := range md {
		out[k] = v
	}
	if version != "" {
		out[discovery.MetaKeyVersion] = version
	}
	if zone != "" {
		out[discovery.MetaKeyZone] = zone
	}
	return out
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
