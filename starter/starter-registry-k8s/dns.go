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

package StarterRegistryK8s

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-spring.org/cloud/discovery"
)

// dnsResolver is the subset of net.Resolver the DNS backend needs. Abstracting
// it lets tests inject a fake resolver without a live cluster; production uses
// net.DefaultResolver.
type dnsResolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// dnsDiscovery resolves a headless Service's DNS records into endpoints.
//
// It needs no RBAC and no Kubernetes client — only cluster DNS, which every
// Pod already has. The trade-off versus the informer mode is propagation
// latency (bounded by DNS TTL plus RefreshInterval) and the absence of
// per-endpoint metadata such as zone.
//
// Freshness is internal: snapshots are cached per service for RefreshInterval,
// so a read is a DNS query at most once per interval and an in-memory read in
// between. DNS offers no push notification, so a read-through TTL cache is the
// whole freshness story.
type dnsDiscovery struct {
	cfg Config
	res dnsResolver

	mu      sync.Mutex // guards entries
	entries map[string]*dnsEntry
}

// dnsEntry is the cached snapshot for one service name. eps holds the FULL
// (unfiltered) set; scheme narrowing happens per Resolve call.
type dnsEntry struct {
	mu        sync.Mutex // guards eps/fetchedAt; held across the fetch so it runs once per TTL window
	eps       []discovery.Endpoint
	fetchedAt time.Time
}

// newDNSDiscovery builds a DNS-mode backend. A nil res selects
// net.DefaultResolver; tests pass a fake.
func newDNSDiscovery(cfg Config, res dnsResolver) *dnsDiscovery {
	if res == nil {
		res = net.DefaultResolver
	}
	return &dnsDiscovery{cfg: cfg, res: res, entries: map[string]*dnsEntry{}}
}

// fqdn builds the cluster-internal FQDN for a Service name:
// "<service>.<namespace>.svc.<cluster-domain>".
func (d *dnsDiscovery) fqdn(service string) string {
	return fmt.Sprintf("%s.%s.svc.%s", service, d.cfg.Namespace, d.cfg.ClusterDomain)
}

// Resolve returns the current endpoint snapshot for the Service. When PortName
// is set it performs an SRV lookup (address and port come from the record);
// otherwise it performs an A/AAAA lookup and pairs each IP with the configured
// Port. A headless Service publishes records only for ready addresses by
// default, so returned endpoints are marked Healthy. A result older than
// RefreshInterval triggers a re-fetch (bounded by ctx); younger ones read the
// cache. opts narrow the result; [discovery.WithScheme] filters by transport
// scheme.
func (d *dnsDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	interval := d.cfg.RefreshInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}

	d.mu.Lock()
	e, ok := d.entries[name]
	if !ok {
		e = &dnsEntry{}
		d.entries[name] = e
	}
	d.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.eps == nil || time.Since(e.fetchedAt) >= interval {
		eps, err := d.fetch(ctx, name)
		if err != nil {
			// A failed refresh serves the stale snapshot; the next call retries.
			// Both outcomes are reported: only a success advances the freshness
			// clock, so a lookup that has stopped working shows up as a climbing
			// cache age rather than a healthy-looking stale snapshot.
			discovery.Synced(obsSystem, name, err)
			if e.eps == nil {
				return nil, err
			}
		} else {
			e.eps, e.fetchedAt = eps, time.Now()
			discovery.Synced(obsSystem, name, nil)
		}
	}
	return discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), discovery.NewQuery("", opts...).Scheme), nil
}

// fetch runs the live DNS query for name.
func (d *dnsDiscovery) fetch(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	if d.cfg.PortName != "" {
		return d.resolveSRV(ctx, name)
	}
	return d.resolveA(ctx, name)
}

// resolveSRV looks up "_<port-name>._tcp.<fqdn>" and turns each SRV record into
// a "target:port" endpoint (the target hostname is itself DNS-resolvable).
func (d *dnsDiscovery) resolveSRV(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	_, srvs, err := d.res.LookupSRV(ctx, d.cfg.PortName, "tcp", d.fqdn(name))
	if err != nil {
		return nil, fmt.Errorf("registry-k8s: SRV lookup %q: %w", d.fqdn(name), err)
	}
	eps := make([]discovery.Endpoint, 0, len(srvs))
	for _, srv := range srvs {
		target := strings.TrimSuffix(srv.Target, ".")
		eps = append(eps, discovery.Endpoint{
			Addr:    net.JoinHostPort(target, strconv.Itoa(int(srv.Port))),
			Weight:  int(srv.Weight),
			Healthy: true,
		})
	}
	sortEndpoints(eps)
	return eps, nil
}

// resolveA looks up the Service A/AAAA records and pairs each IP with the
// configured Port.
func (d *dnsDiscovery) resolveA(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	ips, err := d.res.LookupIPAddr(ctx, d.fqdn(name))
	if err != nil {
		return nil, fmt.Errorf("registry-k8s: A lookup %q: %w", d.fqdn(name), err)
	}
	port := strconv.Itoa(d.cfg.Port)
	eps := make([]discovery.Endpoint, 0, len(ips))
	for _, ip := range ips {
		eps = append(eps, discovery.Endpoint{
			Addr:    net.JoinHostPort(ip.IP.String(), port),
			Healthy: true,
		})
	}
	sortEndpoints(eps)
	return eps, nil
}

// sortEndpoints orders endpoints by address for a stable snapshot, so change
// detection compares sets rather than transient orderings.
func sortEndpoints(eps []discovery.Endpoint) {
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
}

// addrKey builds a stable string key from an endpoint set's addresses, used to
// detect whether a re-resolve changed the set.
func addrKey(eps []discovery.Endpoint) string {
	addrs := make([]string, len(eps))
	for i, ep := range eps {
		addrs[i] = ep.Addr
	}
	sort.Strings(addrs)
	return strings.Join(addrs, ",")
}
