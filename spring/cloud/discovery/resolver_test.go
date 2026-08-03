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
	"testing"
	"time"
)

func TestResolver_ColdStartSeedsFromResolve(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "10.0.0.1:6379"}, Endpoint{Addr: "10.0.0.2:6379"})

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	got := addrsOf(r.Endpoints())
	if len(got) != 2 || got[0] != "10.0.0.1:6379" || got[1] != "10.0.0.2:6379" {
		t.Fatalf("Endpoints = %v, want the seeded snapshot", got)
	}
}

func TestResolver_PickRoundRobin(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "a:1"}, Endpoint{Addr: "b:2"}, Endpoint{Addr: "c:3"})

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	var got []string
	for i := range 6 {
		ep, err := r.Pick()
		if err != nil {
			t.Fatalf("Pick %d: %v", i, err)
		}
		got = append(got, ep.Addr)
	}
	// Two full cycles over the three endpoints, in order.
	want := []string{"a:1", "b:2", "c:3", "a:1", "b:2", "c:3"}
	if len(got) != len(want) {
		t.Fatalf("Pick sequence = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Pick sequence = %v, want %v", got, want)
		}
	}
}

func TestResolver_PickPrefersHealthy(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc",
		Endpoint{Addr: "down:1", Healthy: false},
		Endpoint{Addr: "up:2", Healthy: true},
	)

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	for i := range 4 {
		ep, err := r.Pick()
		if err != nil {
			t.Fatalf("Pick %d: %v", i, err)
		}
		if ep.Addr != "up:2" {
			t.Fatalf("Pick %d = %s, want the healthy up:2", i, ep.Addr)
		}
	}
}

func TestResolver_PickFallsBackWhenNoneHealthy(t *testing.T) {
	d := newStaticDiscovery()
	// No endpoint marked healthy — all eligible so discovery still works.
	d.set("svc", Endpoint{Addr: "x:1"}, Endpoint{Addr: "y:2"})

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	ep, err := r.Pick()
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	if ep.Addr != "x:1" && ep.Addr != "y:2" {
		t.Fatalf("Pick = %s, want one of x:1/y:2", ep.Addr)
	}
}

func TestResolver_PickExcludesDisabled(t *testing.T) {
	d := newStaticDiscovery()
	// A disabled instance must NEVER be picked — not even as a fallback.
	d.set("svc",
		Endpoint{Addr: "drained:1", Disabled: true},
		Endpoint{Addr: "live:2", Healthy: true},
	)

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	for i := range 8 {
		ep, err := r.Pick()
		if err != nil {
			t.Fatalf("Pick %d: %v", i, err)
		}
		if ep.Addr == "drained:1" {
			t.Fatalf("Pick %d returned the disabled endpoint", i)
		}
	}

	// Every endpoint disabled → error, never silently fall back to disabled.
	d2 := newStaticDiscovery()
	d2.set("svc",
		Endpoint{Addr: "off:1", Disabled: true},
		Endpoint{Addr: "off:2", Disabled: true},
	)
	r2, err := NewResolver(context.Background(), d2, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r2.Stop()
	if _, err := r2.Pick(); err == nil {
		t.Fatal("Pick should error when every endpoint is disabled")
	}
}

func TestResolver_PickEmpty(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc") // empty

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	if _, err := r.Pick(); err == nil {
		t.Fatal("Pick should error when the service has no endpoints")
	}
}

func TestResolver_WatchUpdatesSnapshot(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "old:1"})

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	defer r.Stop()

	d.Update("svc", Endpoint{Addr: "new:1"}, Endpoint{Addr: "new:2"})

	// loop applies the snapshot asynchronously; poll briefly.
	if !waitFor(func() bool { return len(r.Endpoints()) == 2 }) {
		t.Fatalf("snapshot not updated, Endpoints = %v", r.Endpoints())
	}
	got := addrsOf(r.Endpoints())
	if got[0] != "new:1" || got[1] != "new:2" {
		t.Fatalf("Endpoints = %v, want [new:1 new:2]", got)
	}
}

func TestResolver_StopIdempotent(t *testing.T) {
	d := newStaticDiscovery()
	d.set("svc", Endpoint{Addr: "a:1"})

	r, err := NewResolver(context.Background(), d, "svc")
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func addrsOf(eps []Endpoint) []string {
	out := make([]string, len(eps))
	for i, ep := range eps {
		out[i] = ep.Addr
	}
	return out
}

func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
