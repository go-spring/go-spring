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

// This file adds the CONSUMER half of etcd service discovery to the registry
// starter: a cloud/discovery Discovery backend that serves snapshots of the
// instances this starter's own registrar (registrar.go) publishes — closing the
// loop so one starter serves both sides of the etcd naming idiom. Freshness is
// internal: each resolved service gets a background etcd watcher that keeps
// the cached snapshot current, so Resolve is a cheap read after the first call.
//
// Backends are named adapters in the discovery registry, not injectable beans
// (same idiom as starter-registry-nacos): configure one block per etcd cluster
// under ${spring.discovery.etcd.<name>} and a client starter cites the name.
//
//	spring.discovery.etcd.prod.endpoints=127.0.0.1:2379
//	spring.discovery.etcd.prod.key-prefix=/services/
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
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// DiscoveryConfig binds one etcd discovery adapter under
// ${spring.discovery.etcd.<name>}. It mirrors the registry-side EtcdConfig
// connection fields (same client idiom, same auth/TLS knobs) — the consumer
// side needs no TTL because it never writes.
type DiscoveryConfig struct {
	// Endpoints lists the etcd cluster nodes to dial. Required; setting any
	// endpoint is what activates the adapter block.
	Endpoints []string `value:"${endpoints}" expr:"len($) > 0"`

	// Username / Password authenticate against etcd when auth is enabled.
	// Leave empty for anonymous clusters.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DialTimeout bounds the initial connection attempt and the startup probe.
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// KeyPrefix is the same prefix the registrar writes under; the adapter only
	// reads keys below it. It must match ${spring.registry.etcd.key-prefix} of
	// the registering applications or nothing resolves.
	KeyPrefix string `value:"${key-prefix:=/services/}"`

	// TLS configures optional transport-layer security, using the shared
	// tlsconf block so every starter exposes the same tls.* keys.
	TLS tlsconf.TLSConfig `value:"${tls}"`
}

func init() {
	gs.Module(gs.OnProperty("spring.discovery.etcd"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.discovery.etcd}", func(name string, c DiscoveryConfig) error {
			if _, err := discovery.GetDiscovery(name); err == nil {
				return errutil.Explain(nil, "registry-etcd: discovery backend %q already registered", name)
			}
			b, err := newEtcdDiscovery(c)
			if err != nil {
				return errutil.Explain(err, "registry-etcd: build discovery backend %q", name)
			}
			discovery.RegisterDiscovery(name, b)
			log.Infof(context.Background(), starterTag, "registered etcd discovery backend name=%s endpoints=%v", name, c.Endpoints)
			return nil
		})
	})
}

// newEtcdDiscovery builds a Discovery backed by an etcd cluster for c. It
// probes the cluster before returning (same fail-fast as the registrar), so an
// unreachable or misauthenticated cluster fails startup.
func newEtcdDiscovery(c DiscoveryConfig) (*etcdDiscovery, error) {
	tlsCfg, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: build TLS")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.DialTimeout,
		TLS:         tlsCfg,
	})
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: failed to create etcd client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()
	if _, err := cli.Status(ctx, c.Endpoints[0]); err != nil {
		_ = cli.Close()
		return nil, errutil.Explain(err, "registry-etcd: discovery startup probe failed for %s", c.Endpoints[0])
	}
	return &etcdDiscovery{
		client:    cli,
		keyPrefix: c.KeyPrefix,
		bgCtx:     context.Background(),
		entries:   map[string]*serviceEntry{},
	}, nil
}

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
