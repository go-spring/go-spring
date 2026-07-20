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
	"net"
	"sync"
	"testing"
	"time"

	"go-spring.org/spring/cloud/discovery"
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

func TestDNS_WatchDetectsChange(t *testing.T) {
	f := &fakeResolver{}
	f.set(nil, []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}})
	d := newDNSDiscovery(Config{
		Mode: ModeDNS, Namespace: "ns", Port: 6379,
		ClusterDomain: "cluster.local", RefreshInterval: 10 * time.Millisecond,
	}, f)

	w, err := d.Watch(context.Background(), "redis")
	assert.Error(t, err).Nil()
	defer w.Stop()

	// Simulate a scale-up: the next poll must surface the new endpoint.
	f.set(nil, []net.IPAddr{
		{IP: net.ParseIP("10.0.0.1")},
		{IP: net.ParseIP("10.0.0.2")},
	})

	eps := nextWithTimeout(t, w, time.Second)
	assert.Slice(t, addrsOf(eps)).Equal([]string{"10.0.0.1:6379", "10.0.0.2:6379"})
}

func TestDNS_WatchStopUnblocksNext(t *testing.T) {
	f := &fakeResolver{}
	f.set(nil, []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}})
	d := newDNSDiscovery(Config{
		Mode: ModeDNS, Namespace: "ns", Port: 6379,
		ClusterDomain: "cluster.local", RefreshInterval: time.Hour,
	}, f)
	w, err := d.Watch(context.Background(), "redis")
	assert.Error(t, err).Nil()

	_ = w.Stop()
	_, err = w.Next()
	assert.Error(t, err).Is(context.Canceled)
	// Stop is idempotent.
	assert.Error(t, w.Stop()).Nil()
}

// nextWithTimeout fails the test if the watcher does not yield within d.
func nextWithTimeout(t *testing.T, w discovery.Watcher, d time.Duration) []discovery.Endpoint {
	t.Helper()
	type result struct {
		eps []discovery.Endpoint
		err error
	}
	ch := make(chan result, 1)
	go func() {
		eps, err := w.Next()
		ch <- result{eps, err}
	}()
	select {
	case r := <-ch:
		assert.Error(t, r.err).Nil()
		return r.eps
	case <-time.After(d):
		t.Fatal("watcher did not yield a snapshot in time")
		return nil
	}
}
