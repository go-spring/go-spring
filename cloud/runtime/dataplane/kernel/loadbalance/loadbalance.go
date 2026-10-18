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
// of [go-spring.org/cloud/runtime/dataplane/kernel/discovery].
//
// Discovery answers "which instances exist right now?"; this package answers
// the next question — "given that live set, which one do I send this request
// to?". It is deliberately split into two concerns:
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
// A strategy names itself in a selection rule and is resolved out of one
// process-wide registry ([Register]): the built-ins each register from their own
// file's init, and a deployment adds its own the same way — the package it links
// registers. The core fixes only the [Balancer]/[Factory]/[Params] contract, so a
// new strategy — with parameters of its own — is added entirely outside this
// package.
//
// The package has zero third-party dependencies; RPC-framework adapters (gRPC
// balancer.Builder, kitex loadbalance.Loadbalancer, ...) live in their starters
// and translate a Balancer into the framework's own picker interface.
package loadbalance

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/stdlib/errutil"
)

// Names of the built-in strategies, each registered by its own file's init. The
// names appear in service configs and gRPC LB config, so they are stable. New
// built-in strategies are admitted only after a survey of industry practice and
// a real consumer; a deployment that wants an implementation variant with
// equivalent semantics (maglev, rendezvous hashing; peak-ewma beside p2c)
// registers it under its own name instead of growing this list.
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

	// Complete marks the request that Pick issued to ep as finished. ctx is the
	// context of the request that triggered this completion — it carries that
	// request's trace and values and is read for logging and instrumentation
	// only; the strategy must not cancel it. err is the request's final error,
	// nil on success. Strategies that keep per-request state settle it here
	// (least-conn decrements its in-flight count); stateless strategies declare
	// a no-op so callers never nil-check. The two-method shape keeps the hot
	// path free of per-request closures. Callers must invoke it exactly once per
	// picked endpoint, after the request ends (least_conn tolerates a repeat —
	// its count deletes at zero).
	Complete(ctx context.Context, ep discovery.Endpoint, err error)
}

// Factory builds a fresh, independent [Balancer] for one target. It is the
// single extension point of this package: a strategy makes itself available by
// calling [Register] from its own file's init — the registered name IS the
// strategy name a selection rule cites — so adding a strategy, built-in or
// third-party, never changes this file again.
//
// p carries the strategy's own parameters, flat and interpreted by the strategy
// alone; a factory ends with [Params.Done], so a parameter aimed at another
// strategy fails construction instead of being silently dropped.
//
// A factory holds no per-target state — the registry keeps ONE value and calls
// it per target — so anything stateful belongs on the Balancer it returns, not
// on the factory. It is a plain function type rather than an interface with a
// Build method: a strategy has exactly one entry point and no state, so a named
// type per strategy would buy nothing.
type Factory func(p *Params) (Balancer, error)

// strategies is the one table of strategies a process has, keyed by the name a
// selection rule cites. The built-ins and anything a linked package registers
// share it, which is what lets a composing factory (zone_aware's delegate) or an
// RPC adapter resolve a name without being handed a directory.
var (
	mu         sync.RWMutex
	strategies = map[string]Factory{}
)

// Register makes f available under name. It is called from a strategy file's
// init, so it has no error channel: an empty name, a nil factory, or a name
// registered twice are programming errors and panic. A duplicate means two
// packages claim one name — a collision a deployment would otherwise meet only
// as "my rule silently routed the other way".
func Register(name string, f Factory) {
	switch {
	case name == "":
		panic(errutil.Explain(nil, "loadbalance: register with empty name"))
	case f == nil:
		panic(errutil.Explain(nil, "loadbalance: register nil factory for %s", name))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := strategies[name]; ok {
		panic(errutil.Explain(nil, "loadbalance: strategy already registered: %s", name))
	}
	strategies[name] = f
}

// New builds the strategy named name. An unknown name, or a params bag the
// strategy rejects, is an error — callers degrade it to "keep the current
// strategy" (see [Pool]), so a bad rule never takes a client down. A nil p is
// the empty bag, so a caller with no parameters need not wrap one.
func New(name string, p *Params) (Balancer, error) {
	mu.RLock()
	f, ok := strategies[name]
	mu.RUnlock()
	if !ok {
		return nil, errutil.Explain(nil, "loadbalance: no strategy registered as %q (registered: %v)", name, Names())
	}
	if p == nil {
		p = NewParams(nil)
	}
	return f(p)
}

// Validate reports whether every strategy named by s could be built, WITHOUT
// building any of them into a chain: each non-empty [Selection.Balancer] — the
// default's and every rule's — goes through the same [New] a chain rebuild uses,
// so an unregistered name and a params bag the strategy rejects are both caught
// here rather than at the next rebuild.
//
// It is the check a caller runs BEFORE adopting s, so the answer cannot drift
// from what adoption would do: same registry, same resolution path.
//
// An empty Balancer is not an error — it names the base strategy. Unlike the
// per-chain degrade in [Manager.Apply] (which keeps a bad rule's resource on its
// previous strategy), this is all-or-nothing: it answers "is the whole settings
// consistent with what this process has registered?".
//
// A package-level function rather than a [Manager] method: the registry is
// process-wide (see [Register]), so there is no per-manager directory to consult.
// It takes no lock beyond the registry's own, so it is safe for concurrent use.
func Validate(s Settings) error {
	if err := validateSelection(s.Default); err != nil {
		return err
	}
	for _, r := range s.Rules {
		if err := validateSelection(r.Selection); err != nil {
			return err
		}
	}
	return nil
}

// validateSelection resolves one selection's strategy, discarding the instance:
// it exists only to surface the error [New] would report.
func validateSelection(sel Selection) error {
	if sel.Balancer == "" {
		return nil // the base strategy; nothing to resolve
	}
	_, err := New(sel.Balancer, NewParams(sel.Params))
	return err
}

// Names lists the registered strategy names, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return slices.Sorted(maps.Keys(strategies))
}
