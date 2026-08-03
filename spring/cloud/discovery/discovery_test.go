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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// panics reports whether fn panicked with a value whose message contains want.
func panics(want string, fn func()) (did bool) {
	defer func() {
		r := recover()
		did = r != nil && strings.Contains(fmt.Sprint(r), want)
	}()
	fn()
	return
}

func TestResolve(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.3:80", Healthy: true})
	RegisterDiscovery("test-resolve", d)

	backend, err := GetDiscovery("test-resolve")
	if err != nil {
		t.Fatalf("GetDiscovery: %v", err)
	}
	got, err := backend.Resolve(context.Background(), "svc")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got) != 1 || got[0].Addr != "10.0.0.3:80" {
		t.Fatalf("Resolve = %+v, want one endpoint at 10.0.0.3:80", got)
	}
}

func TestRegisterDiscoveryAndGet(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:80"})
	RegisterDiscovery("test-get", d)

	got, err := GetDiscovery("test-get")
	if err != nil {
		t.Fatalf("GetDiscovery: %v", err)
	}
	if got != Discovery(d) {
		t.Fatalf("GetDiscovery returned %v, want the exact Discovery registered", got)
	}
}

func TestGetDiscoveryNotFoundListsRegistered(t *testing.T) {
	// Register a sentinel so the diagnostic has a concrete name to list.
	RegisterDiscovery("test-notfound-sentinel", newStaticDiscovery())

	_, err := GetDiscovery("does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a missing Discovery, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does-not-exist") {
		t.Fatalf("error should name the requested Discovery: %v", err)
	}
	if !strings.Contains(msg, "test-notfound-sentinel") {
		t.Fatalf("error should list registered Discoveries to make typos obvious: %v", err)
	}
}

func TestRegisterDiscoveryPanics(t *testing.T) {
	if !panics("empty name", func() { RegisterDiscovery("", newStaticDiscovery()) }) {
		t.Error("registering with an empty name should panic")
	}
	if !panics("nil Discovery", func() { RegisterDiscovery("test-nil", nil) }) {
		t.Error("registering a nil Discovery should panic")
	}
	RegisterDiscovery("test-dup", newStaticDiscovery())
	if !panics("already registered", func() {
		RegisterDiscovery("test-dup", newStaticDiscovery())
	}) {
		t.Error("registering a duplicate name should panic")
	}
}

func TestCatalogIsOptional(t *testing.T) {
	// A Discovery that implements Catalog exposes enumeration.
	withCatalog := newStaticDiscovery("svc-a", "svc-b")
	RegisterDiscovery("test-catalog", withCatalog)
	b, err := GetDiscovery("test-catalog")
	if err != nil {
		t.Fatalf("GetDiscovery: %v", err)
	}
	c, ok := b.(Catalog)
	if !ok {
		t.Fatal("a Discovery with Services should satisfy Catalog")
	}
	names, err := c.Services(context.Background())
	if err != nil {
		t.Fatalf("Services: %v", err)
	}
	if len(names) != 2 || names[0] != "svc-a" || names[1] != "svc-b" {
		t.Fatalf("Services = %v, want [svc-a svc-b]", names)
	}

	// A Discovery that does NOT implement Catalog must fail the assertion — the
	// intended degradation, never an empty list or a panic.
	RegisterDiscovery("test-no-catalog", discoveryOnly{})
	b2, _ := GetDiscovery("test-no-catalog")
	if _, ok := b2.(Catalog); ok {
		t.Fatal("a Discovery without Services should not satisfy Catalog")
	}
}

// discoveryOnly implements Discovery but deliberately not Catalog, so the
// optional-Catalog assertion can be exercised in the negative.
type discoveryOnly struct{}

func (discoveryOnly) Resolve(context.Context, string) ([]Endpoint, error) {
	return nil, nil
}

func (discoveryOnly) Watch(context.Context, string) (<-chan WatchResult, error) {
	return nil, nil
}

func TestErrUnsupportedIsSentinel(t *testing.T) {
	if !errors.Is(ErrUnsupported, ErrUnsupported) {
		t.Fatal("ErrUnsupported should be usable with errors.Is")
	}
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

func TestNewStaticDiscovery_WatchSeedsAndCloses(t *testing.T) {
	d := NewStaticDiscovery(Endpoint{Addr: "10.0.0.1:80", Healthy: true})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := d.Watch(ctx, "x")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	select {
	case r := <-ch:
		if r.Err != nil || len(r.Endpoints) != 1 {
			t.Fatalf("first result = %+v, want one endpoint", r)
		}
	case <-time.After(time.Second):
		t.Fatal("first result not delivered")
	}

	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after ctx cancel")
	}
}

func TestWatchFirstResultIsCurrent(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:80", Healthy: true})
	RegisterDiscovery("test-watch", d)
	backend, _ := GetDiscovery("test-watch")

	ch, err := backend.Watch(t.Context(), "svc")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	select {
	case r := <-ch:
		if r.Err != nil {
			t.Fatalf("first result errored: %v", r.Err)
		}
		if len(r.Endpoints) != 1 || r.Endpoints[0].Addr != "10.0.0.1:80" {
			t.Fatalf("first result = %+v, want the current snapshot", r)
		}
	case <-time.After(time.Second):
		t.Fatal("first result was not delivered promptly")
	}
}

func TestWatchCancelClosesChannel(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.2:80"})
	RegisterDiscovery("test-watch-cancel", d)
	backend, _ := GetDiscovery("test-watch-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := backend.Watch(ctx, "svc")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	<-ch // drain the seed result so the next receive observes the close.

	cancel()
	select {
	case r, ok := <-ch:
		if ok {
			t.Fatalf("channel should be closed after ctx cancel, got result %+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("channel was not closed after ctx cancel")
	}
}

func TestWatchUpdatePushesNewSnapshot(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:80"})
	RegisterDiscovery("test-watch-update", d)
	backend, _ := GetDiscovery("test-watch-update")

	ch, err := backend.Watch(t.Context(), "svc")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	<-ch // drain the seed (initial single instance).

	// Simulate a topology change: two instances now. The live watcher must
	// receive the new full snapshot.
	d.Update("svc",
		Endpoint{Addr: "10.0.0.1:80"},
		Endpoint{Addr: "10.0.0.2:80"},
	)
	select {
	case r := <-ch:
		if r.Err != nil || len(r.Endpoints) != 2 {
			t.Fatalf("update result = %+v, want two endpoints", r)
		}
	case <-time.After(time.Second):
		t.Fatal("topology change was not pushed to the live watcher")
	}
}
