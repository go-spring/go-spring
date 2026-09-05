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
// of [go-spring.org/cloud/discovery].
//
// Discovery answers "which instances exist right now?"; this package answers
// the next question — "given that live set, which one do I send this request
// to?". It is deliberately split into two concerns, mirroring the discovery and
// resilience packages:
//
//   - [Balancer] is the pluggable selection strategy (round-robin, least-conn,
//     consistent-hash, weighted, zone-aware). It is pure: given a candidate
//     endpoint set and a [PickInfo] it returns one endpoint, plus the
//     Complete method that settles the request it issued.
//   - [Pool] binds a live discovery source (a [discovery.Resolver] bound to one
//     service name via [discovery.NewResolver], adapted by [SourceFunc]) and a
//     [Tracker] (outlier suspension) to a Balancer, so the candidate set stays
//     fresh as instances come and go and unhealthy instances are suspended.
//
// The package has zero third-party dependencies; RPC-framework adapters (gRPC
// balancer.Builder, kitex loadbalance.Loadbalancer, ...) live in their starters
// and translate a Balancer into the framework's own picker interface.
package loadbalance

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"go-spring.org/cloud/discovery"
)

// Names of the built-in strategies, registered by their respective files.
// The names appear in service configs and gRPC LB config, so they are stable.
// New strategies are admitted only after a survey of industry practice and a
// real consumer; implementation variants with equivalent semantics (maglev,
// rendezvous hashing; peak-ewma beside p2c) are deliberately out.
const (
	RoundRobin     = "round_robin"
	LeastConn      = "least_conn"
	ConsistentHash = "consistent_hash"
	Weighted       = "weighted"
	ZoneAware      = "zone_aware"
	Random         = "random"
	P2C            = "p2c"
)

// ErrNoAvailable is returned by a [Balancer] or [Pool] when there is no eligible
// endpoint to pick — the candidate set is empty after discovery and suspension
// filtering.
var ErrNoAvailable = errors.New("loadbalance: no available endpoint")

// PickInfo carries the per-request inputs a [Balancer] may route on. All fields
// are optional; a plain round-robin balancer ignores them entirely.
//
// The struct admits a field only when a strategy consumes it. The leading
// candidates for future fields are Subset (canary metadata routing,
// the biggest gap) and Attempt (retry avoidance); both wait for a consumer.
type PickInfo struct {
	// HashKey selects the instance for hash-based strategies (consistent hash).
	// Requests sharing a HashKey land on the same instance while the topology is
	// stable. Ignored by strategies that do not hash.
	HashKey string

	// Zone is the caller's locality (region/zone/unit). Zone-aware strategies
	// prefer endpoints whose Metadata advertises the same zone and only spill
	// over to remote ones when no local endpoint is available.
	Zone string
}

// Balancer selects one endpoint from a live candidate set per request. The
// candidate slice is supplied on every call (the caller owns discovery and
// suspension filtering), so a Balancer only needs to hold selection state such as a
// round-robin cursor or a hash ring cache — binding the set at construction
// would force a rebuild on every topology change and zero that state, and a
// push model would duplicate the discovery snapshot per balancer. All state
// must be keyed by endpoint address (never by index or order) so an arbitrary
// topology change adapts: a removed address is cleaned up or drains, and a
// returning address resumes with its state intact. Implementations must be
// safe for concurrent use.
type Balancer interface {
	// Pick returns one endpoint from eps for the given info. eps is the already
	// filtered eligible set; Pick must not mutate it. It returns [ErrNoAvailable]
	// when eps is empty.
	Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error)

	// Complete marks the request that Pick issued to ep as finished: err is the
	// request's final error, nil on success. Strategies that keep per-request
	// state settle it here (least-conn decrements its in-flight count);
	// stateless strategies declare a no-op so callers never nil-check. The
	// two-method shape keeps the hot path free of per-request closures. Callers
	// must invoke it exactly once per picked endpoint, after the request ends
	// (least_conn tolerates a repeat — its count deletes at zero).
	Complete(ep discovery.Endpoint, err error)
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
// is empty, f is nil, or name is already registered.
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
		names := slices.Sorted(maps.Keys(registry))
		mu.RUnlock()
		return nil, fmt.Errorf("loadbalance: no strategy registered as %q (registered: %v)", name, names)
	}
	mu.RUnlock()
	return f(), nil
}
