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

package discovery

import (
	"context"
	"slices"
	"sync"
	"testing"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/stdlib/errutil"
)

func TestResolve(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.3:80", Healthy: true})

	got, err := d.Resolve(context.Background(), "svc")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 1 || got[0].Addr != "10.0.0.3:80" {
		t.Fatalf("Resolve = %+v, want one endpoint at 10.0.0.3:80", got)
	}
}

func TestNewResolverBindsBackendAndSeeds(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:80", Healthy: true})

	r, err := NewResolver(context.Background(), d, "svc", "")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	eps, err := r(context.Background())
	if err != nil || len(eps) != 1 || eps[0].Addr != "10.0.0.1:80" {
		t.Fatalf("resolver = %v, err %v — want the bound backend's snapshot", eps, err)
	}
}

func TestNewResolverUnknownServiceFailsSeed(t *testing.T) {
	// The seed Resolve surfaces an unknown service at construction time.
	failing := &failingDiscovery{}
	if _, err := NewResolver(context.Background(), failing, "svc", ""); err == nil {
		t.Fatal("expected the seed Resolve error to propagate, got nil")
	}
}

// countingDiscovery records how often Resolve runs, making the seed-vs-read
// contract visible: construction seeds once, and every call re-reads.
type countingDiscovery struct{ calls int }

func (d *countingDiscovery) Resolve(context.Context, string) ([]Endpoint, error) {
	d.calls++
	return []Endpoint{{Addr: "10.0.0.1:80"}}, nil
}

func TestNewResolverSeedsOnceThenReads(t *testing.T) {
	d := &countingDiscovery{}
	r, err := NewResolver(context.Background(), d, "svc", "")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	if d.calls != 1 {
		t.Fatalf("construction resolved %d times, want 1 (the seed)", d.calls)
	}
	if _, err := r(context.Background()); err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if d.calls != 2 {
		t.Fatalf("after one read, Resolve calls = %d, want 2", d.calls)
	}
}

// failingDiscovery always fails Resolve, to exercise seed error propagation.
type failingDiscovery struct{}

func (failingDiscovery) Resolve(context.Context, string) ([]Endpoint, error) {
	return nil, errutil.Explain(nil, "boom")
}

// resolved returns the endpoint set a consumer sees through a Resolver bound to
// d with scheme: the backend's raw snapshot narrowed by the bound scheme.
func resolved(t *testing.T, d Discovery, scheme string) []Endpoint {
	t.Helper()
	r, err := NewResolver(context.Background(), d, "svc", scheme)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	eps, err := r(context.Background())
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	return eps
}

func TestNewStaticDiscovery(t *testing.T) {
	d := NewStaticDiscovery(
		Endpoint{Addr: "10.0.0.1:8080", Healthy: true},
		Endpoint{Addr: "10.0.0.2:8080"},
	)

	// Resolve serves the fixed set for any name; the result is read-only by
	// contract (see [Discovery.Resolve]).
	got, err := d.Resolve(context.Background(), "any")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 2 || got[0].Addr != "10.0.0.1:8080" || got[1].Addr != "10.0.0.2:8080" {
		t.Fatalf("Resolve = %+v, want the fixed two-endpoint set", got)
	}
}

func TestResolverNarrowsByScheme(t *testing.T) {
	// One service exposing three schemes: plain (""), tls, and grpc.
	d := NewStaticDiscovery(
		Endpoint{Addr: "10.0.0.1:80", Scheme: "", Healthy: true},
		Endpoint{Addr: "10.0.0.1:443", Scheme: "tls", Healthy: true},
		Endpoint{Addr: "10.0.0.1:9090", Scheme: "grpc", Healthy: true},
	)

	tests := []struct {
		name   string
		scheme string
		want   []string
	}{
		{"no scheme keeps everything", "", []string{"10.0.0.1:80", "10.0.0.1:443", "10.0.0.1:9090"}},
		{"tls", "tls", []string{"10.0.0.1:443"}},
		{"grpc", "grpc", []string{"10.0.0.1:9090"}},
		{"tcp matches the plain endpoint", "tcp", []string{"10.0.0.1:80"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, ep := range resolved(t, d, tt.scheme) {
				got = append(got, ep.Addr)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("resolve(scheme=%q) = %v, want %v", tt.scheme, got, tt.want)
			}
		})
	}
}

// TestFilterByMetadata covers the metadata dimension, the runtime half: the
// construction-time scheme has already settled the other one.
func TestFilterByMetadata(t *testing.T) {
	eps := []Endpoint{
		{Addr: "10.0.0.1:80", Metadata: map[string]string{"version": "v1.9"}},
		{Addr: "10.0.0.2:80", Metadata: map[string]string{"version": "v2.1"}},
		{Addr: "10.0.0.3:80", Metadata: map[string]string{"version": "v2.2"}},
		{Addr: "10.0.0.4:80"}, // no metadata: naming "version" must drop it
	}
	sel := matcher.Selector{{Key: "version", Op: matcher.Prefix, Value: "v2."}}

	got := FilterByMetadata(eps, sel)
	if len(got) != 2 || got[0].Addr != "10.0.0.2:80" || got[1].Addr != "10.0.0.3:80" {
		t.Fatalf("FilterByMetadata(selector) = %+v — want the two v2 instances", got)
	}

	// An empty selector narrows nothing.
	if all := FilterByMetadata(eps, nil); len(all) != len(eps) {
		t.Fatalf("FilterByMetadata(nil selector) = %d endpoints, want %d", len(all), len(eps))
	}

	// No match returns an empty set, not the input.
	if none := FilterByMetadata(eps, matcher.Selector{{Key: "version", Op: matcher.Exact, Value: "v9"}}); len(none) != 0 {
		t.Fatalf("FilterByMetadata(no match) = %+v, want empty", none)
	}
}

// TestResolverSchemeThenMetadataFilters pins the two halves together: the
// constructor settles the scheme, then the caller narrows by metadata per read.
func TestResolverSchemeThenMetadataFilters(t *testing.T) {
	d := NewStaticDiscovery(
		Endpoint{Addr: "a", Scheme: "tls", Metadata: map[string]string{"version": "v2.1"}},
		Endpoint{Addr: "b", Scheme: "tls", Metadata: map[string]string{"version": "v1.9"}},
		Endpoint{Addr: "c", Scheme: "", Metadata: map[string]string{"version": "v2.2"}},
	)
	r, err := NewResolver(context.Background(), d, "svc", "tls")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	eps, err := r(context.Background())
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	got := FilterByMetadata(eps, matcher.Selector{{Key: "version", Op: matcher.Prefix, Value: "v2."}})
	if len(got) != 1 || got[0].Addr != "a" {
		t.Fatalf("schema+metadata narrowing = %+v, want only the tls v2 instance", got)
	}
}

func TestFilterByMetadataAliasing(t *testing.T) {
	eps := []Endpoint{
		{Addr: "a", Metadata: map[string]string{"version": "v"}},
		{Addr: "b", Metadata: map[string]string{"version": "v"}},
	}

	// No selector: the input is returned unchanged.
	got := FilterByMetadata(eps, nil)
	if len(got) != 2 || &got[0] != &eps[0] {
		t.Fatal("FilterByMetadata without a selector must return the input slice")
	}

	// A selector always yields a fresh slice, even when every endpoint matches,
	// so mutating the result cannot reach the caller's snapshot.
	got = FilterByMetadata(eps, matcher.Selector{{Key: "version", Op: matcher.Exact, Value: "v"}})
	if len(got) != 2 {
		t.Fatalf("FilterByMetadata = %d endpoints, want 2", len(got))
	}
	got[0].Addr = "mutated"
	if eps[0].Addr == "mutated" {
		t.Fatal("FilterByMetadata must not alias the input when it filters")
	}
}

func TestResolveSeesTopologyChange(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:80"})

	got, err := d.Resolve(context.Background(), "svc")
	if err != nil || len(got) != 1 {
		t.Fatalf("Resolve = %v, err %v — want one endpoint", got, err)
	}

	// Simulate a topology change: two instances now. The next snapshot read
	// must observe it — freshness is the backend's job in the Resolve-only model.
	d.Update("svc",
		Endpoint{Addr: "10.0.0.1:80"},
		Endpoint{Addr: "10.0.0.2:80"},
	)
	got, err = d.Resolve(context.Background(), "svc")
	if err != nil || len(got) != 2 {
		t.Fatalf("Resolve after update = %v, err %v — want two endpoints", got, err)
	}
}

// staticDiscovery is an in-memory backend used only by tests in this package.
// Update replaces the snapshot for a name, so tests can simulate a topology
// change and observe it through Resolve.
type staticDiscovery struct {
	mu  sync.Mutex
	eps map[string][]Endpoint
}

func newStaticDiscovery() *staticDiscovery {
	return &staticDiscovery{eps: map[string][]Endpoint{}}
}

// set replaces the endpoint snapshot stored for name.
func (s *staticDiscovery) set(name string, eps ...Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = eps
}

func (s *staticDiscovery) Resolve(_ context.Context, name string) ([]Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.eps[name], nil
}

// Update replaces the snapshot for name. It is how tests simulate a topology
// change.
func (s *staticDiscovery) Update(name string, eps ...Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = append([]Endpoint(nil), eps...)
}
