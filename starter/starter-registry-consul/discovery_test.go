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

package StarterRegistryConsul

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// fakeConsul stands in for a Consul agent's health endpoint: it serves one
// snapshot per index and honors the blocking-query contract — a request whose
// index is not older than the current one blocks until the snapshot advances
// (or the request context ends). set() advances both.
type fakeConsul struct {
	mu      sync.Mutex
	entries []*api.ServiceEntry
	index   uint64
	changed chan struct{}

	// tags records every tag carried by a query, in order; the background
	// blocking query races per-call lookups, so "most recent" is not stable.
	tags []string
}

// snapshotTags returns a copy of the recorded tags for failure messages.
func (f *fakeConsul) snapshotTags() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.tags...)
}

// saw reports whether any query carried tag.
func (f *fakeConsul) saw(tag string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tags {
		if t == tag {
			return true
		}
	}
	return false
}

// set replaces the served snapshot and unblocks pending blocking queries.
func (f *fakeConsul) set(entries []*api.ServiceEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = entries
	f.index++
	if f.changed != nil {
		close(f.changed)
	}
	f.changed = make(chan struct{})
}

func (f *fakeConsul) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	idx, _ := strconv.ParseUint(r.URL.Query().Get("index"), 10, 64)

	f.mu.Lock()
	f.tags = append(f.tags, tag)
	entries, index, changed := f.entries, f.index, f.changed
	// Not stale enough to answer immediately: block until the snapshot
	// advances or the client gives up (context canceled / wait elapsed).
	if idx >= index && idx != 0 {
		f.mu.Unlock()
		select {
		case <-changed:
			f.mu.Lock()
			entries, index = f.entries, f.index
			f.mu.Unlock()
		case <-r.Context().Done():
			return
		}
	} else {
		f.mu.Unlock()
	}

	w.Header().Set("X-Consul-Index", strconv.FormatUint(index, 10))
	_ = json.NewEncoder(w).Encode(entries)
}

// newTestDiscovery wires a consulDiscovery against a fake agent, plus a stop
// func releasing the background watchers.
func newTestDiscovery(t *testing.T, fake *fakeConsul, tag string) *consulDiscovery {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&api.Config{Address: srv.URL})
	assert.That(t, err).Nil()
	d := newConsulDiscovery(client, tag)
	t.Cleanup(d.Close)
	return d
}

func svcEntry(id, address string, port int, passing int, meta map[string]string) *api.ServiceEntry {
	return &api.ServiceEntry{
		Node: &api.Node{Address: "10.9.9.9"},
		Service: &api.AgentService{
			ID:      id,
			Service: "orders",
			Address: address,
			Port:    port,
			Meta:    meta,
			Weights: api.AgentWeights{Passing: passing, Warning: 1},
		},
	}
}

// TestConsulDiscovery_Resolve covers the query path: field mapping (addr,
// weight from the passing weight, meta scheme passthrough, node-address
// fallback) and scheme narrowing.
func TestConsulDiscovery_Resolve(t *testing.T) {
	fake := &fakeConsul{entries: []*api.ServiceEntry{
		svcEntry("a", "10.0.0.2", 8080, 3, map[string]string{"zone": "b", "scheme": "tls"}),
		svcEntry("b", "10.0.0.1", 8080, 1, nil),
		// A service with no address of its own falls back to its node's.
		{Node: &api.Node{Address: "10.0.0.3"}, Service: &api.AgentService{ID: "c", Port: 8081}},
	}}
	d := newTestDiscovery(t, fake, "")

	eps, err := d.Resolve(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("want 3 endpoints, got %d", len(eps))
	}
	// Sorted by address; passing=true kept them all healthy.
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:8080")
	assert.That(t, eps[1].Addr).Equal("10.0.0.2:8080")
	assert.That(t, eps[2].Addr).Equal("10.0.0.3:8081")
	assert.That(t, eps[0].Healthy).True()
	// Weight and metadata (incl. scheme passthrough) survive the mapping.
	assert.That(t, eps[1].Weight).Equal(3)
	assert.That(t, eps[1].Scheme).Equal("tls")
	assert.That(t, eps[1].Metadata["zone"]).Equal("b")

	// Scheme narrowing drops the plain instances.
	eps, err = d.Resolve(context.Background(), "orders", discovery.WithScheme("tls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].Addr != "10.0.0.2:8080" {
		t.Fatalf("scheme filter wrong: %+v", eps)
	}
}

// TestConsulDiscovery_WatchRefreshesSnapshot covers the internal freshness
// chain: the first Resolve seeds the cache and starts the blocking query, and
// a later snapshot advance refreshes it so the next Resolve observes it.
func TestConsulDiscovery_WatchRefreshesSnapshot(t *testing.T) {
	fake := &fakeConsul{entries: []*api.ServiceEntry{
		svcEntry("a", "10.0.0.1", 8080, 1, nil),
	}}
	d := newTestDiscovery(t, fake, "")

	eps, err := d.Resolve(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 || eps[0].Addr != "10.0.0.1:8080" {
		t.Fatalf("seed snapshot wrong: %+v", eps)
	}

	// One more instance appears: the blocking query delivers the new full
	// snapshot and the next Resolve reads it from the cache.
	fake.set([]*api.ServiceEntry{
		svcEntry("a", "10.0.0.1", 8080, 1, nil),
		svcEntry("b", "10.0.0.2", 8080, 1, nil),
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		eps, err = d.Resolve(context.Background(), "orders")
		if err != nil {
			t.Fatal(err)
		}
		if len(eps) == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("watched snapshot never refreshed: %+v", eps)
}

// TestConsulDiscovery_TagNarrowsQuery covers the tag contract: the block's
// tag reaches the registry query server-side, and a per-call WithTag option
// overrides it for that one lookup.
func TestConsulDiscovery_TagNarrowsQuery(t *testing.T) {
	fake := &fakeConsul{entries: []*api.ServiceEntry{svcEntry("a", "10.0.0.1", 8080, 1, nil)}}
	d := newTestDiscovery(t, fake, "v2")

	if _, err := d.Resolve(context.Background(), "orders"); err != nil {
		t.Fatal(err)
	}
	if !fake.saw("v2") {
		t.Fatalf("block tag never reached the registry query (tags seen: %v)", fake.snapshotTags())
	}

	// A per-call tag overrides the backend's for that lookup.
	eps, err := d.Resolve(context.Background(), "orders", discovery.WithTag("canary"))
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 1 {
		t.Fatalf("tag override query wrong: %+v", eps)
	}
	if !fake.saw("canary") {
		t.Fatalf("per-call WithTag never reached the registry query (tags seen: %v)", fake.snapshotTags())
	}
}

// TestServiceEntriesToEndpoints covers the pure mapping without HTTP: passing
// weight becomes endpoint weight, meta scheme becomes Endpoint.Scheme, and a
// nil Service entry is skipped rather than fatal.
func TestServiceEntriesToEndpoints(t *testing.T) {
	eps := serviceEntriesToEndpoints([]*api.ServiceEntry{
		svcEntry("a", "10.0.0.1", 80, 7, map[string]string{"scheme": "grpc"}),
		{Node: &api.Node{Address: "10.0.0.2"}}, // Service nil — skipped
	})
	assert.That(t, len(eps)).Equal(1)
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:80")
	assert.That(t, eps[0].Weight).Equal(7)
	assert.That(t, eps[0].Scheme).Equal("grpc")
	assert.That(t, eps[0].Healthy).True()
}

// compile-time contract: the adapter satisfies the discovery interface.
var _ discovery.Discovery = (*consulDiscovery)(nil)

// TestConsulDiscoveryNoClientPanics guards the zero-value degenerate case: the
// adapter is only constructible via newConsulDiscovery, which always sets the
// client; nothing in the type relies on lazy initialization.
func TestConsulDiscoveryNoClientPanics(t *testing.T) {
	d := &consulDiscovery{}
	_ = d.entry("x")
}
