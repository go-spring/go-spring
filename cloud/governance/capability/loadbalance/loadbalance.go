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

package loadbalance

import (
	"context"
	"errors"

	"go-spring.org/cloud/discovery"
)

// Names of the built-in strategies, exported by the built-in provider. The names
// appear in service configs and gRPC LB config, so they are stable. New built-in
// strategies are admitted only after a survey of industry practice and a real
// consumer; a deployment that wants an implementation variant with equivalent
// semantics (maglev, rendezvous hashing; peak-ewma beside p2c) contributes its
// own [Provider] under its own name instead of growing this list.
const (
	RoundRobin     = "round_robin"
	LeastConn      = "least_conn"
	ConsistentHash = "consistent_hash"
	Weighted       = "weighted"
	ZoneAware      = "zone_aware"
	Random         = "random"
	P2C            = "p2c"
)

// ErrNoEndpoints is returned by a [Balancer] or [Pool] when there is no eligible
// endpoint to pick — the candidate set is empty after discovery and suspension
// filtering.
var ErrNoEndpoints = errors.New("loadbalance: no available endpoints")

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
	// filtered eligible set; Pick must not mutate it. It returns [ErrNoEndpoints]
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
