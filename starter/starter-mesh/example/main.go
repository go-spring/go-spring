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

// Command example is a self-contained smoke test for service-mesh mode. It
// needs no external services or docker: it registers one static discovery
// backend that serves three endpoints for "echo-svc", then runs the exact same
// client code twice — once with mesh mode off, once with it on — and asserts
// the two acceptance behaviours:
//
//  1. mesh OFF — client-side discovery + load balancing are active: the
//     round-robin Pool spreads requests evenly across the three real endpoints,
//     and the discovery backend is resolved.
//  2. mesh ON  — both layers degrade to a pass-through: every request goes to a
//     single stable endpoint (the service name the sidecar intercepts), the
//     balancer does not select, and the discovery backend is never consulted.
//
// The process exits 0 only if both assertions hold.
package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"go-spring.org/spring/discovery"
	"go-spring.org/spring/loadbalance"
)

// countingDiscovery serves a fixed set of endpoints and counts how often it is
// resolved, so the smoke can prove mesh mode bypasses the backend entirely.
type countingDiscovery struct {
	eps      []discovery.Endpoint
	resolves atomic.Int64
}

func (d *countingDiscovery) Resolve(context.Context, string) ([]discovery.Endpoint, error) {
	d.resolves.Add(1)
	return d.eps, nil
}

func (d *countingDiscovery) Watch(context.Context, string) (discovery.Watcher, error) {
	return noopWatcher{}, nil
}

// noopWatcher never yields; the smoke does not exercise topology changes.
type noopWatcher struct{}

func (noopWatcher) Next() ([]discovery.Endpoint, error) { select {} }
func (noopWatcher) Stop() error                         { return nil }

func fatalf(format string, args ...any) {
	fmt.Printf("FAIL: "+format+"\n", args...)
	os.Exit(1)
}

const serviceName = "echo-svc"

var backend = &countingDiscovery{eps: []discovery.Endpoint{
	{Addr: "10.0.0.1:8080", Healthy: true},
	{Addr: "10.0.0.2:8080", Healthy: true},
	{Addr: "10.0.0.3:8080", Healthy: true},
}}

// distribute builds a live dialer + round-robin pool for the service under the
// given mesh setting, picks n times, and returns the per-endpoint hit counts
// plus how many times the discovery backend was resolved.
func distribute(mesh bool, n int) (map[string]int, int64) {
	backend.resolves.Store(0)
	discovery.SetMeshMode(mesh)

	ld, err := discovery.NewClientDialer(context.Background(), "default", serviceName)
	if err != nil {
		fatalf("build dialer (mesh=%v): %v", mesh, err)
	}
	defer ld.Stop()

	pool := loadbalance.NewPool(ld, loadbalance.NewRoundRobin())
	hits := map[string]int{}
	for range n {
		r, err := pool.Pick(loadbalance.PickInfo{})
		if err != nil {
			fatalf("pick (mesh=%v): %v", mesh, err)
		}
		hits[r.Endpoint.Addr]++
		if r.Done != nil {
			r.Done(loadbalance.DoneInfo{})
		}
	}
	return hits, backend.resolves.Load()
}

func main() {
	discovery.Register("default", backend)

	const n = 30

	// Phase 1: mesh off — LB active, even spread across the three endpoints.
	hits, resolves := distribute(false, n)
	fmt.Printf("mesh OFF: hits=%v resolves=%d\n", hits, resolves)
	if len(hits) != 3 {
		fatalf("mesh off: expected traffic across 3 endpoints, got %d: %v", len(hits), hits)
	}
	for addr, c := range hits {
		if c != n/3 {
			fatalf("mesh off: uneven distribution, %s got %d (want %d)", addr, c, n/3)
		}
	}
	if resolves == 0 {
		fatalf("mesh off: discovery backend was never resolved")
	}

	// Phase 2: mesh on — same code, degraded to a single stable endpoint and the
	// backend is never touched.
	hits, resolves = distribute(true, n)
	fmt.Printf("mesh ON:  hits=%v resolves=%d\n", hits, resolves)
	if len(hits) != 1 {
		fatalf("mesh on: expected a single stable endpoint, got %d: %v", len(hits), hits)
	}
	if hits[serviceName] != n {
		fatalf("mesh on: expected all %d requests to %q, got %v", n, serviceName, hits)
	}
	if resolves != 0 {
		fatalf("mesh on: discovery backend was resolved %d times (should be bypassed)", resolves)
	}

	fmt.Println("OK: mesh mode degrades discovery + load balancing to a pass-through")
}
