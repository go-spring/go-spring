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

// Package loadbalance is the client-side load-balancing layer that sits on top
// of [go-spring.org/spring/discovery].
//
// Discovery answers "which instances exist right now?"; this package answers
// the next question — "given that live set, which one do I send this request
// to?". It is deliberately split into two concerns, mirroring the discovery and
// resilience packages:
//
//   - [Balancer] is the pluggable selection strategy (round-robin, least-conn,
//     consistent-hash, weighted, zone-aware). It is pure: given a candidate
//     endpoint set and a [PickInfo] it returns one [Result].
//   - [Pool] binds a live discovery source (via [discovery.LiveDialer]) and a
//     [Tracker] (outlier ejection) to a Balancer, so the candidate set stays
//     fresh as instances come and go and unhealthy instances are evicted.
//
// The package has zero third-party dependencies; RPC-framework adapters (gRPC
// balancer.Builder, kitex loadbalance.Loadbalancer, ...) live in their starters
// and translate a Balancer into the framework's own picker interface.
package loadbalance

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"go-spring.org/spring/cloud/discovery"
)

// ErrNoAvailable is returned by a [Balancer] or [Pool] when there is no eligible
// endpoint to pick — the candidate set is empty after discovery and eviction
// filtering.
var ErrNoAvailable = errors.New("loadbalance: no available endpoint")

// PickInfo carries the per-request inputs a [Balancer] may route on. All fields
// are optional; a plain round-robin balancer ignores them entirely.
type PickInfo struct {
	// Ctx is the request context. Balancers may read routing hints from it but
	// must not retain it beyond the Pick call.
	Ctx context.Context

	// HashKey selects the instance for hash-based strategies (consistent hash).
	// Requests sharing a HashKey land on the same instance while the topology is
	// stable. Ignored by strategies that do not hash.
	HashKey string

	// Zone is the caller's locality (region/zone/unit). Zone-aware strategies
	// prefer endpoints whose Metadata advertises the same zone and only spill
	// over to remote ones when no local endpoint is available.
	Zone string
}

// Result is the outcome of a [Balancer.Pick]: the chosen endpoint plus an
// optional Done callback the caller invokes once the request finishes.
type Result struct {
	// Endpoint is the selected instance.
	Endpoint discovery.Endpoint

	// Done reports the request outcome back to the balancer. It is non-nil for
	// strategies that keep per-request state (least-conn decrements its in-flight
	// count) or that feed an ejection [Tracker]. Callers should always invoke it
	// when non-nil, exactly once, after the call completes. It is safe to guard
	// with `if r.Done != nil`.
	Done func(DoneInfo)
}

// DoneInfo describes how a balanced request ended. It is passed to [Result.Done].
type DoneInfo struct {
	// Err is the request's final error, or nil on success. Ejection trackers
	// treat a non-nil Err as a failure signal for the picked endpoint.
	Err error
}

// Balancer selects one endpoint from a live candidate set per request. The
// candidate slice is supplied on every call (the caller owns discovery and
// eviction), so a Balancer only needs to hold selection state such as a
// round-robin cursor or a hash ring cache. Implementations must be safe for
// concurrent use.
type Balancer interface {
	// Pick returns one endpoint from eps for the given info. eps is the already
	// filtered eligible set; Pick must not mutate it. It returns [ErrNoAvailable]
	// when eps is empty.
	Pick(eps []discovery.Endpoint, info PickInfo) (Result, error)
}

// Factory builds a fresh, independent [Balancer]. The registry stores factories
// (not balancers) because balancers hold mutable per-target state, so every
// target gets its own instance.
type Factory func() Balancer

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register makes a [Balancer] strategy available under name. It panics if name
// is empty, f is nil, or name is already registered, matching the driver-registry
// idiom used across stdlib (discovery.Register, resilience.RegisterDriver) so
// duplicate wiring fails loudly at init.
func Register(name string, f Factory) {
	if name == "" {
		panic("loadbalance: register with empty name")
	}
	if f == nil {
		panic("loadbalance: register nil factory for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("loadbalance: strategy already registered: " + name)
	}
	registry[name] = f
}

// New builds a new [Balancer] for the registered strategy name, or returns an
// error listing the available strategies when none matches.
func New(name string) (Balancer, error) {
	mu.RLock()
	f, ok := registry[name]
	if !ok {
		names := make([]string, 0, len(registry))
		for k := range registry {
			names = append(names, k)
		}
		mu.RUnlock()
		sort.Strings(names)
		return nil, fmt.Errorf("loadbalance: no strategy registered as %q (registered: %v)", name, names)
	}
	mu.RUnlock()
	return f(), nil
}

// Names of the built-in strategies, registered by their respective files.
const (
	RoundRobin     = "round_robin"
	LeastConn      = "least_conn"
	ConsistentHash = "consistent_hash"
	Weighted       = "weighted"
)
