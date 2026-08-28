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

// This file adds the CONSUMER half of Nacos service discovery to the registry
// starter: a cloud/discovery Discovery backend that resolves and watches
// instances from Nacos naming. Registration (the provider half, registrar.go)
// and discovery now share one starter and one naming client idiom.
//
// Like starter-discovery-k8s, backends are named adapters in the discovery
// registry, not injectable beans: configure one block per Nacos cluster under
// ${spring.discovery.nacos.<name>} and a client starter cites the name.
//
//	spring.discovery.nacos.prod.server=127.0.0.1:8848
//	spring.discovery.nacos.prod.namespace=8f3b...
//	spring.discovery.nacos.prod.group=DEFAULT_GROUP

package StarterRegistryNacos

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// DiscoveryConfig binds one Nacos discovery adapter under
// ${spring.discovery.nacos.<name>}. It mirrors the registry-side NacosConfig
// fields (same client idiom, same auth knobs) plus the consumer-side cluster
// scoping.
type DiscoveryConfig struct {
	// Server is the Nacos server address, e.g. "127.0.0.1:8848". Required.
	Server string `value:"${server}" expr:"$ != ''"`

	// Namespace is the Nacos namespace id; empty uses "public". It must match
	// the namespace the provider registered into.
	Namespace string `value:"${namespace:=}"`

	// Group is the service group to resolve within.
	Group string `value:"${group:=DEFAULT_GROUP}"`

	// Cluster narrows resolution to one Nacos cluster. It defaults to
	// "DEFAULT" — the same cluster the registrar publishes into by default —
	// so a no-config consumer sees a no-config provider. Set it explicitly to
	// another cluster to scope resolution there, or to an empty value to
	// span all clusters.
	Cluster string `value:"${cluster:=DEFAULT}"`

	// Username / Password authenticate against Nacos when auth is enabled.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// TimeoutMs bounds each Nacos API call.
	TimeoutMs uint64 `value:"${timeout-ms:=5000}"`
}

func init() {
	gs.Module(gs.OnProperty("spring.discovery.nacos"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.discovery.nacos}", func(name string, c DiscoveryConfig) error {
			if _, err := discovery.GetDiscovery(name); err == nil {
				return errutil.Explain(nil, "registry-nacos: discovery backend %q already registered", name)
			}
			warnRegistryDivergence(p, name, c)
			b, err := newNacosDiscovery(c)
			if err != nil {
				return errutil.Explain(err, "registry-nacos: build discovery backend %q", name)
			}
			discovery.RegisterDiscovery(name, b)
			log.Infof(context.Background(), starterTag, "registered nacos discovery backend name=%s server=%s", name, c.Server)
			return nil
		})
	})
}

// warnRegistryDivergence compares a discovery adapter's namespace/group with
// the registrar's ${spring.registry.nacos} when that side is configured.
// Registration and discovery live under different config prefixes, so a typo'd
// namespace or group would otherwise fail silently as an empty instance set;
// the WARN names the divergence at startup instead. It is only a warning —
// legitimately pointing discovery at a different Nacos tenant than the one
// this process registers into is a valid deployment.
func warnRegistryDivergence(p flatten.Storage, name string, c DiscoveryConfig) {
	var reg NacosConfig
	if err := conf.Bind(p, &reg, "${spring.registry.nacos}"); err != nil || reg.Server == "" {
		return // registrar not in play (or not bindable) — nothing to compare
	}
	if reg.Namespace != c.Namespace {
		log.Warnf(context.Background(), starterTag,
			"registry-nacos: discovery backend %q uses namespace %q but this instance registers into namespace %q — cross-namespace resolution returns nothing unless that is intended",
			name, c.Namespace, reg.Namespace)
	}
	if reg.Group != c.Group {
		log.Warnf(context.Background(), starterTag,
			"registry-nacos: discovery backend %q uses group %q but this instance registers into group %q — cross-group resolution returns nothing unless that is intended",
			name, c.Group, reg.Group)
	}
}

// newNacosDiscovery builds a Discovery backed by a Nacos naming client for c.
// It probes the server before returning (same fail-fast as the registrar), so
// an unreachable or misauthenticated Nacos fails startup.
func newNacosDiscovery(c DiscoveryConfig) (*nacosDiscovery, error) {
	host, portStr, err := net.SplitHostPort(c.Server)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: server %q must be host:port", c.Server)
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: server %q has a non-numeric port", c.Server)
	}

	sc := []constant.ServerConfig{*constant.NewServerConfig(host, port)}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId(c.Namespace),
		constant.WithTimeoutMs(c.TimeoutMs),
		constant.WithUsername(c.Username),
		constant.WithPassword(c.Password),
		constant.WithNotLoadCacheAtStart(true),
	)
	client, err := clients.NewNamingClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc})
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: create naming client for %s", c.Server)
	}
	if _, err := client.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		NameSpace: c.Namespace, GroupName: c.Group, PageNo: 1, PageSize: 1,
	}); err != nil {
		return nil, errutil.Explain(err, "registry-nacos: discovery startup probe failed for %s", c.Server)
	}
	return &nacosDiscovery{client: client, group: c.Group, cluster: c.Cluster}, nil
}

// nacosDiscovery resolves and watches service instances in one Nacos
// namespace/group. It implements [discovery.Discovery].
type nacosDiscovery struct {
	client  naming_client.INamingClient
	group   string
	cluster string
}

// Resolve returns the current healthy instance set for name.
func (d *nacosDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	instances, err := d.selectInstances(ctx, name)
	if err != nil {
		return nil, err
	}
	eps := instancesToEndpoints(instances)
	eps = discovery.FilterByScheme(eps, discovery.NewQuery("", opts...).Scheme)
	sortEndpoints(eps)
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

// Watch subscribes to name's instance set and pushes a fresh full snapshot on
// the returned channel on every Nacos push. The first result carries the
// current set (from an explicit query — the SDK's first callback delivery is
// an implementation detail, the channel contract is not); the channel closes
// when ctx is cancelled, which also unsubscribes.
//
// Channel discipline follows starter-discovery-k8s: the Nacos callback never
// touches the channel — it only records the latest snapshot and signals; a
// single goroutine owns the channel as sole writer and closer, so there is no
// send-after-close race.
func (d *nacosDiscovery) Watch(ctx context.Context, name string, opts ...discovery.Option) (<-chan discovery.WatchResult, error) {
	scheme := discovery.NewQuery("", opts...).Scheme

	out := make(chan discovery.WatchResult, 1)
	updates := make(chan struct{}, 1)
	var mu sync.Mutex
	var latest []discovery.Endpoint
	record := func(instances []model.Instance) {
		eps := instancesToEndpoints(instances)
		eps = discovery.FilterByScheme(eps, scheme)
		sortEndpoints(eps)
		mu.Lock()
		latest = eps
		mu.Unlock()
		select {
		case updates <- struct{}{}:
		default: // a pending signal is enough; snapshots are full, not deltas
		}
	}

	cb := func(services []model.Instance, err error) {
		if err != nil {
			// A failed push keeps the last snapshot: stale addresses are
			// safer than none (the Discovery contract's own degradation rule).
			log.Warnf(context.Background(), starterTag, "registry-nacos: watch %s callback error (keeping last snapshot): %v", name, err)
			return
		}
		record(services)
	}
	if err := d.client.Subscribe(&vo.SubscribeParam{
		ServiceName: name, GroupName: d.group, Clusters: d.clusterList(), SubscribeCallback: cb,
	}); err != nil {
		return nil, errutil.Explain(err, "registry-nacos: subscribe %s in %s failed", name, d.group)
	}

	go func() {
		defer close(out)
		defer func() {
			_ = d.client.Unsubscribe(&vo.SubscribeParam{ServiceName: name, GroupName: d.group, Clusters: d.clusterList(), SubscribeCallback: cb})
		}()

		// Seed the channel with the current set; dedupe against the first
		// callback delivery when Nacos also fires it with the same content.
		if instances, err := d.selectInstances(ctx, name); err == nil {
			record(instances)
		} else {
			log.Warnf(context.Background(), starterTag, "registry-nacos: initial query for %s failed (waiting for first push): %v", name, err)
		}

		var lastKey string
		for {
			select {
			case <-ctx.Done():
				return
			case <-updates:
				mu.Lock()
				eps := latest
				mu.Unlock()
				key := endpointsKey(eps)
				if key == lastKey {
					continue // no-op re-delivery must not churn consumers
				}
				lastKey = key
				out <- discovery.WatchResult{Endpoints: eps}
			}
		}
	}()
	return out, nil
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
		})
	}
	return eps
}

// sortEndpoints orders endpoints by address so snapshots are comparable.
func sortEndpoints(eps []discovery.Endpoint) {
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
}

// endpointsKey renders a snapshot as a comparable string for change
// detection.
func endpointsKey(eps []discovery.Endpoint) string {
	var b strings.Builder
	for _, e := range eps {
		b.WriteString(e.Addr)
		b.WriteByte(',')
		b.WriteString(e.Scheme)
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(e.Weight))
		b.WriteByte(';')
		b.WriteString(strconv.FormatBool(e.Disabled))
		b.WriteByte(',')
		b.WriteString(strconv.FormatBool(e.Healthy))
		b.WriteByte('|')
	}
	return b.String()
}
