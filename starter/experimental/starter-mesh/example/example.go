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

// Command example is a self-contained smoke test for the service-mesh switch
// (spring/cloud/mesh). It needs no external services or docker: it registers one
// static discovery backend that serves three endpoints for "echo-svc", then runs
// the same client code twice — once with mesh off, once with it on — exercising
// "approach A" (each caller branches on mesh.Enabled):
//
//  1. mesh OFF — client-side discovery + load balancing are active: build a
//     discovery.Resolver + round-robin Pool, which spreads requests evenly
//     across the three real endpoints, and the discovery backend is resolved.
//  2. mesh ON  — a sidecar owns discovery+LB, so the app skips discovery
//     entirely and connects to the service's single stable address (the service
//     name); the balancer is not built and the discovery backend is never
//     consulted.
//
// The process exits 0 only if both assertions hold.
package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/mesh"
	"go-spring.org/spring/experimental/cloud/loadbalance"
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

func (d *countingDiscovery) Watch(ctx context.Context, _ string) (<-chan discovery.WatchResult, error) {
	ch := make(chan discovery.WatchResult, 1)
	ch <- discovery.WatchResult{Endpoints: d.eps}
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func fatalf(format string, args ...any) {
	fmt.Printf("FAIL: " + format + "\n", args...)
	os.Exit(1)
}

const serviceName = "echo-svc"

var backend = &countingDiscovery{eps: []discovery.Endpoint{
	{Addr: "10.0.0.1:8080", Healthy: true},
	{Addr: "10.0.0.2:8080", Healthy: true},
	{Addr: "10.0.0.3:8080", Healthy: true},
}}

// distribute runs n picks against the service under the given mesh setting,
// following approach A: mesh off builds a Resolver + round-robin Pool; mesh on
// skips discovery and targets the service's single stable address directly. It
// returns the per-endpoint hit counts plus how often the backend was resolved.
func distribute(meshOn bool, n int) (map[string]int, int64) {
	backend.resolves.Store(0)
	mesh.SetEnabled(meshOn)

	hits := map[string]int{}
	if meshOn {
		// A sidecar owns discovery+LB; the app connects straight to the service's
		// stable address and never consults the discovery backend.
		for range n {
			hits[serviceName]++
		}
		return hits, backend.resolves.Load()
	}

	// Normal mode: resolve the service and load-balance across the live endpoints.
	dis, err := discovery.GetDiscovery("default")
	if err != nil {
		fatalf("get discovery (mesh=%v): %v", meshOn, err)
	}
	rsv, err := discovery.NewResolver(context.Background(), dis, serviceName)
	if err != nil {
		fatalf("build resolver (mesh=%v): %v", meshOn, err)
	}
	defer rsv.Stop()

	pool := loadbalance.NewPool(rsv, loadbalance.NewRoundRobin())
	for range n {
		r, err := pool.Pick(loadbalance.PickInfo{})
		if err != nil {
			fatalf("pick (mesh=%v): %v", meshOn, err)
		}
		hits[r.Endpoint.Addr]++
		if r.Done != nil {
			r.Done(loadbalance.DoneInfo{})
		}
	}
	return hits, backend.resolves.Load()
}

func main() {
	discovery.RegisterDiscovery("default", backend)

	const n = 30

	// Phase 1: mesh off — discovery + LB active, even spread across the three.
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

	// Phase 2: mesh on — same intent, but the app skips discovery and targets the
	// service's stable address; the backend is never touched.
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

	fmt.Println("OK: mesh mode bypasses client-side discovery (approach A)")
}
