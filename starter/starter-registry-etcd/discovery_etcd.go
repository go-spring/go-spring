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
// starter: a cloud/discovery Discovery backend that resolves and watches the
// instances this starter's own registrar (registrar.go) publishes — closing the
// loop so one starter serves both sides of the etcd naming idiom.
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
	tlsCfg, err := c.TLS.Build()
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
	return &etcdDiscovery{client: cli, keyPrefix: c.KeyPrefix}, nil
}

// etcdDiscovery resolves and watches service instances stored under one etcd
// prefix. It implements [discovery.Discovery] and [discovery.Catalog].
type etcdDiscovery struct {
	client    *clientv3.Client
	keyPrefix string
}

// servicePrefix returns the etcd key prefix holding name's instances:
// keyPrefix + name + "/" — the layout etcdRegistrar.keyFor writes.
func (d *etcdDiscovery) servicePrefix(name string) string {
	return d.keyPrefix + name + "/"
}

// Resolve returns the current instance set for name. A key's existence is the
// health signal: leases delete expired keys, so everything found is live.
func (d *etcdDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	resp, err := d.client.Get(ctx, d.servicePrefix(name), clientv3.WithPrefix())
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: get %q failed", name)
	}
	eps := kvsToEndpoints(resp.Kvs)
	eps = discovery.FilterByScheme(eps, discovery.NewQuery("", opts...).Scheme)
	return eps, nil
}

// Watch subscribes to name's instance set and pushes a fresh full snapshot on
// the returned channel on every etcd change below the service prefix. The
// first result carries the current set; the channel closes when ctx is
// cancelled, which also ends the etcd watch.
//
// Channel discipline follows the registry-nacos idiom: the etcd watch channel
// is consumed by a single goroutine that is the sole writer and closer of the
// output channel, so there is no send-after-close race.
func (d *etcdDiscovery) Watch(ctx context.Context, name string, opts ...discovery.Option) (<-chan discovery.WatchResult, error) {
	scheme := discovery.NewQuery("", opts...).Scheme
	prefix := d.servicePrefix(name)

	snapshot := func() []discovery.Endpoint {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		resp, err := d.client.Get(cctx, prefix, clientv3.WithPrefix())
		if err != nil {
			// Keep serving from etcd watch events; stale addresses are safer
			// than none (the Discovery contract's degradation rule).
			log.Warnf(context.Background(), starterTag, "registry-etcd: snapshot %q failed (waiting for watch): %v", name, err)
			return nil
		}
		return kvsToEndpoints(resp.Kvs)
	}

	out := make(chan discovery.WatchResult, 1)
	wch := d.client.Watch(ctx, prefix, clientv3.WithPrefix())
	go func() {
		defer close(out)

		// Seed the channel with the current set so a long-lived caller need
		// not Resolve first (the Watch contract).
		eps := discovery.FilterByScheme(snapshot(), scheme)
		lastKey := endpointsKey(eps)
		out <- discovery.WatchResult{Endpoints: eps}

		for range wch {
			eps := discovery.FilterByScheme(snapshot(), scheme)
			key := endpointsKey(eps)
			if key == lastKey {
				continue // no-op re-delivery must not churn consumers
			}
			lastKey = key
			out <- discovery.WatchResult{Endpoints: eps}
		}
	}()
	return out, nil
}

// Services enumerates every service name with at least one live key below the
// key prefix — the discovery.Catalog capability, used by gateways and catalogs.
func (d *etcdDiscovery) Services(ctx context.Context) ([]string, error) {
	resp, err := d.client.Get(ctx, d.keyPrefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: list services failed")
	}
	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}
	return serviceNames(keys, d.keyPrefix), nil
}

// serviceNames extracts the distinct service names from instance keys below
// prefix, whose layout is <prefix><service>/<instance>. Keys without the
// service/instance split are skipped.
func serviceNames(keys []string, prefix string) []string {
	seen := map[string]struct{}{}
	for _, k := range keys {
		rel := strings.TrimPrefix(k, prefix)
		if service, _, ok := strings.Cut(rel, "/"); ok && service != "" {
			seen[service] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for s := range seen {
		names = append(names, s)
	}
	sort.Strings(names)
	return names
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
