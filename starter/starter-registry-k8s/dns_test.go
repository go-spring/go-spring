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
	"net"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// fakeResolver is an injectable stand-in for net.Resolver so DNS-mode tests run
// without a cluster. It serves canned SRV/A answers and lets a test swap them
// mid-run to simulate a scale event.
type fakeResolver struct {
	mu   sync.Mutex
	srv  []*net.SRV
	ips  []net.IPAddr
	name string // records the last queried FQDN for assertions
}

func (f *fakeResolver) set(srv []*net.SRV, ips []net.IPAddr) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.srv, f.ips = srv, ips
}

func (f *fakeResolver) LookupSRV(_ context.Context, _, _, name string) (string, []*net.SRV, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.name = name
	return "", f.srv, nil
}

func (f *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.name = host
	return f.ips, nil
}

func addrsOf(eps []discovery.Endpoint) []string {
	out := make([]string, len(eps))
	for i, e := range eps {
		out[i] = e.Addr
	}
	return out
}

func TestDNS_ResolveSRV(t *testing.T) {
	f := &fakeResolver{}
	f.set([]*net.SRV{
		{Target: "10-0-0-2.svc.ns.svc.cluster.local.", Port: 8080},
		{Target: "10-0-0-1.svc.ns.svc.cluster.local.", Port: 8080},
	}, nil)
	d := newDNSDiscovery(Config{
		Mode: ModeDNS, Namespace: "ns", PortName: "grpc", ClusterDomain: "cluster.local",
	}, f)

	eps, err := d.Resolve(context.Background(), "svc")
	assert.Error(t, err).Nil()
	// SRV targets keep their hostname; the set is sorted for a stable snapshot.
	assert.Slice(t, addrsOf(eps)).Equal([]string{
		"10-0-0-1.svc.ns.svc.cluster.local:8080",
		"10-0-0-2.svc.ns.svc.cluster.local:8080",
	})
	assert.String(t, f.name).Equal("svc.ns.svc.cluster.local")
	assert.That(t, eps[0].Healthy).True()
}

func TestDNS_ResolveA(t *testing.T) {
	f := &fakeResolver{}
	f.set(nil, []net.IPAddr{
		{IP: net.ParseIP("10.0.0.2")},
		{IP: net.ParseIP("10.0.0.1")},
	})
	d := newDNSDiscovery(Config{
		Mode: ModeDNS, Namespace: "ns", Port: 6379, ClusterDomain: "cluster.local",
	}, f)

	eps, err := d.Resolve(context.Background(), "redis")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:6379", "10.0.0.2:6379"})
	assert.String(t, f.name).Equal("redis.ns.svc.cluster.local")
}

func TestDNS_CacheRefreshesAfterTTL(t *testing.T) {
	f := &fakeResolver{}
	f.set(nil, []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}})
	d := newDNSDiscovery(Config{
		Mode: ModeDNS, Namespace: "ns", Port: 6379,
		ClusterDomain: "cluster.local", RefreshInterval: 10 * time.Millisecond,
	}, f)

	eps, err := d.Resolve(context.Background(), "redis")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:6379"})

	// Simulate a scale-up: after the TTL elapses, the next read re-fetches.
	f.set(nil, []net.IPAddr{
		{IP: net.ParseIP("10.0.0.1")},
		{IP: net.ParseIP("10.0.0.2")},
	})
	time.Sleep(20 * time.Millisecond)
	eps, err = d.Resolve(context.Background(), "redis")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:6379", "10.0.0.2:6379"})
}

func TestDNS_CacheServesWithinTTL(t *testing.T) {
	f := &fakeResolver{}
	f.set(nil, []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}})
	d := newDNSDiscovery(Config{
		Mode: ModeDNS, Namespace: "ns", Port: 6379,
		ClusterDomain: "cluster.local", RefreshInterval: time.Hour,
	}, f)

	_, err := d.Resolve(context.Background(), "redis")
	assert.Error(t, err).Nil()

	// A scale-up within the TTL window must NOT trigger a fetch — the cached
	// snapshot is served as-is.
	f.set(nil, []net.IPAddr{
		{IP: net.ParseIP("10.0.0.1")},
		{IP: net.ParseIP("10.0.0.2")},
	})
	eps, err := d.Resolve(context.Background(), "redis")
	assert.Error(t, err).Nil()
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:6379"})
}
