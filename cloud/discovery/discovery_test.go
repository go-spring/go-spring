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
	"fmt"
	"sync"
	"testing"
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

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	eps, err := r()
	if err != nil || len(eps) != 1 || eps[0].Addr != "10.0.0.1:80" {
		t.Fatalf("resolver = %v, err %v — want the bound backend's snapshot", eps, err)
	}
}

func TestNewResolverUnknownServiceFailsSeed(t *testing.T) {
	// The seed Resolve surfaces an unknown service at construction time.
	failing := &failingDiscovery{}
	if _, err := NewResolver(context.Background(), failing, "svc"); err == nil {
		t.Fatal("expected the seed Resolve error to propagate, got nil")
	}
}

func TestNewResolverNotInEffect(t *testing.T) {
	// A nil backend (no discovery configured) means "not in effect".
	r, err := NewResolver(context.Background(), nil, "svc")
	if r != nil || err != nil {
		t.Fatalf("nil backend => (%v, %v), want (nil, nil)", r, err)
	}
	// An empty service name means the caller dials its configured address.
	r, err = NewResolver(context.Background(), newStaticDiscovery(), "")
	if r != nil || err != nil {
		t.Fatalf("empty name => (%v, %v), want (nil, nil)", r, err)
	}
}

// failingDiscovery always fails Resolve, to exercise seed error propagation.
type failingDiscovery struct{}

func (failingDiscovery) Resolve(context.Context, string, ...Option) ([]Endpoint, error) {
	return nil, fmt.Errorf("boom")
}

func TestNewStaticDiscovery(t *testing.T) {
	d := NewStaticDiscovery(
		Endpoint{Addr: "10.0.0.1:8080", Healthy: true},
		Endpoint{Addr: "10.0.0.2:8080"},
	)

	// Resolve serves the fixed set for any name, and returns a copy.
	got, err := d.Resolve(context.Background(), "any")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Resolve = %d endpoints, want 2", len(got))
	}
	got[0] = Endpoint{Addr: "mutated"}
	again, _ := d.Resolve(context.Background(), "any")
	if again[0].Addr == "mutated" {
		t.Fatal("Resolve must return a copy, not the internal slice")
	}
}

func TestSchemeOptionNarrows(t *testing.T) {
	// One service exposing three schemes: plain (""), tls, and grpc.
	d := NewStaticDiscovery(
		Endpoint{Addr: "10.0.0.1:80", Scheme: "", Healthy: true},
		Endpoint{Addr: "10.0.0.1:443", Scheme: "tls", Healthy: true},
		Endpoint{Addr: "10.0.0.1:9090", Scheme: "grpc", Healthy: true},
	)

	// No option: every scheme comes back.
	all, err := d.Resolve(context.Background(), "svc")
	if err != nil || len(all) != 3 {
		t.Fatalf("Resolve without scheme = %v, err %v — want 3 endpoints", all, err)
	}

	// WithScheme("tls"): only the tls endpoint.
	tls, err := d.Resolve(context.Background(), "svc", WithScheme("tls"))
	if err != nil || len(tls) != 1 || tls[0].Scheme != "tls" {
		t.Fatalf("Resolve(tls) = %+v, err %v — want the single tls endpoint", tls, err)
	}

	// WithScheme("grpc"): only the grpc endpoint.
	grpc, err := d.Resolve(context.Background(), "svc", WithScheme("grpc"))
	if err != nil || len(grpc) != 1 || grpc[0].Scheme != "grpc" {
		t.Fatalf("Resolve(grpc) = %+v, err %v — want the single grpc endpoint", grpc, err)
	}

	// WithScheme("tcp") matches the plain "" endpoint (the documented tcp/"" plain
	// equivalence), not the tls/grpc ones.
	plain, err := d.Resolve(context.Background(), "svc", WithScheme("tcp"))
	if err != nil || len(plain) != 1 || plain[0].Addr != "10.0.0.1:80" {
		t.Fatalf("Resolve(tcp) = %+v, err %v — want the plain endpoint", plain, err)
	}

	// WithScheme("") is a no-op: behaves like no option at all.
	noop, err := d.Resolve(context.Background(), "svc", WithScheme(""))
	if err != nil || len(noop) != 3 {
		t.Fatalf("Resolve(\"\") = %d endpoints, want 3 (no-op)", len(noop))
	}
}

func TestQueryOptionsCompose(t *testing.T) {
	// NewQuery applies every option; an empty option set leaves narrowing at its
	// zero defaults. This is the plumbing backends rely on to read opts.
	q := NewQuery("svc",
		WithScheme("tls"),
		WithTag("v2"),
	)
	if q.Name != "svc" || q.Scheme != "tls" || q.Tag != "v2" {
		t.Fatalf("NewQuery = %+v, want all three dimensions set", q)
	}

	// Empty values are no-ops, so conditionally passing a config field needs no
	// guard - WithScheme("") leaves the default.
	q2 := NewQuery("svc", WithScheme(""), WithTag(""))
	if q2.Scheme != "" || q2.Tag != "" {
		t.Fatalf("empty options should be no-ops, got %+v", q2)
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

func (s *staticDiscovery) Resolve(_ context.Context, name string, opts ...Option) ([]Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return FilterByScheme(append([]Endpoint(nil), s.eps[name]...), NewQuery("", opts...).Scheme), nil
}

// Update replaces the snapshot for name. It is how tests simulate a topology
// change.
func (s *staticDiscovery) Update(name string, eps ...Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eps[name] = append([]Endpoint(nil), eps...)
}
