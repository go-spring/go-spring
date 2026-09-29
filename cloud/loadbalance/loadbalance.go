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
//     service name via [discovery.NewResolver]) and a
//     [Tracker] (outlier suspension) to a Balancer, so the candidate set stays
//     fresh as instances come and go and unhealthy instances are suspended.
//
// The package has zero third-party dependencies; RPC-framework adapters (gRPC
// balancer.Builder, kitex loadbalance.Loadbalancer, ...) live in their starters
// and translate a Balancer into the framework's own picker interface.
package loadbalance

import (
	"errors"
	"maps"
	"slices"
	"sync"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/errutil"
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

// Config carries the tunable parameters a [Balancer] strategy may consume at
// construction — the same shared-config shape as governance/resilience's
// PolicyConfig, so the registry stays name-keyed while instances can differ in
// parameters. Each strategy consumes only its own fields and REJECTS a Config
// that sets any other field (see [Config.only]): a parameter aimed at another
// strategy is a construction error, never a silently dropped value. The zero
// Config builds every strategy with its documented defaults.
type Config struct {
	// Replicas is the number of virtual nodes per endpoint for consistent-hash
	// strategies. <=0 uses the strategy default (100).
	Replicas int

	// ZoneKey is the [discovery.Endpoint.Metadata] key a zone-aware strategy
	// reads for an endpoint's locality. Empty uses [DefaultZoneKey].
	ZoneKey string

	// Delegate names the strategy a zone-aware balancer delegates the final
	// choice to (e.g. "least_conn" for least-conn inside the zone). Empty uses
	// round-robin. Naming zone_aware itself is an error (it would recurse).
	Delegate string
}

// only verifies that no field of c is set except the ones named in fields,
// returning an error listing the set-but-uncategorized ones. It is the shared
// strict-partition check every registered factory runs before building. A
// Config field added later joins the checked set here, so every strategy
// rejects it by default until it opts in by naming it.
func (c Config) only(fields ...string) error {
	set := map[string]bool{}
	if c.Replicas != 0 {
		set["replicas"] = true
	}
	if c.ZoneKey != "" {
		set["zone_key"] = true
	}
	if c.Delegate != "" {
		set["delegate"] = true
	}
	for _, f := range fields {
		delete(set, f)
	}
	if len(set) == 0 {
		return nil
	}
	return errutil.Explain(nil, "loadbalance: config fields ignored by this strategy: %v", slices.Sorted(maps.Keys(set)))
}

// Factory builds a fresh, independent [Balancer] from cfg. The registry stores
// factories (not balancers) because balancers hold mutable per-target state,
// so every target gets its own instance.
//
// A factory first validates strict field partition via [Config.only] and
// errors on a misdirected parameter (replicas on least_conn), so a bad Config
// surfaces at construction instead of silently doing nothing. The error
// propagates out of [New]; [Pool.ApplySelection] degrades it to "keep the
// current strategy", the same fail-static as an unknown name.
type Factory func(cfg Config) (Balancer, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register makes a [Balancer] strategy available under name. It panics if name
// is empty, f is nil, or name is already registered.
func Register(name string, f Factory) {
	if name == "" {
		panic(errutil.Explain(nil, "loadbalance: register with empty name"))
	}
	if f == nil {
		panic(errutil.Explain(nil, "loadbalance: register nil factory for %s", name))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic(errutil.Explain(nil, "loadbalance: strategy already registered: %s", name))
	}
	registry[name] = f
}

// New builds a new [Balancer] for the registered strategy name, tuned by cfg
// (the zero Config selects every strategy's defaults), or returns an error
// listing the available strategies when none matches.
func New(name string, cfg Config) (Balancer, error) {
	mu.RLock()
	f, ok := registry[name]
	if !ok {
		names := slices.Sorted(maps.Keys(registry))
		mu.RUnlock()
		return nil, errutil.Explain(nil, "loadbalance: no strategy registered as %q (registered: %v)", name, names)
	}
	mu.RUnlock()
	return f(cfg)
}
