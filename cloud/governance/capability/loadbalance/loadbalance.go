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

// Built-in strategy names, exported by the built-in provider. They appear in
// service configs and gRPC LB config, so they are stable. A new built-in is
// admitted only after a survey of industry practice and a real consumer; a
// deployment that wants an implementation variant with equivalent semantics
// (maglev, rendezvous hashing; peak-ewma beside p2c) contributes its own
// [Provider] under its own name instead of growing this list.
const (
	RoundRobin     = "round_robin"
	LeastConn      = "least_conn"
	ConsistentHash = "consistent_hash"
	Weighted       = "weighted"
	Random         = "random"
	P2C            = "p2c"
)

// ErrNoEndpoints is returned by a [Balancer] or [Pool] when the candidate set is
// empty — no endpoints were discovered, none survived the selector, or every
// endpoint was excluded as disabled or drained.
var ErrNoEndpoints = errors.New("loadbalance: no available endpoints")

// PickInfo carries the per-request inputs a [Balancer] may route on. Every field
// is optional, and a strategy reads only the fields it understands: round-robin
// ignores all of them.
//
// A field belongs here only when a strategy consumes it. The leading candidates
// for future fields are Subset (canary metadata routing, the biggest gap) and
// Attempt (retry avoidance); both wait for a consumer.
type PickInfo struct {
	// HashKey selects the instance for hash-based strategies (consistent hash).
	// Requests sharing a HashKey land on the same instance while the topology is
	// stable. Strategies that do not hash ignore it.
	HashKey string

	// Zone is the caller's locality (region/zone/unit). The zone-aware filter
	// prefers endpoints whose Metadata advertises the same zone and spills over to
	// remote ones only when no local endpoint is available; strategies themselves
	// ignore it.
	Zone string
}

// Balancer selects one endpoint from a live candidate set for each request. The
// candidate slice is supplied on every call — the caller owns discovery and the
// surrounding filters — so a Balancer only needs to keep selection state such as
// a round-robin cursor or a hash ring cache. Binding the set at construction
// would force a rebuild on every topology change and reset that state, and a
// push model would duplicate the discovery snapshot per balancer.
//
// State must be keyed by endpoint address (never by index or order) so an
// arbitrary topology change adapts: a removed address is cleaned up or drains,
// and a returning address resumes with its state intact. Implementations must be
// safe for concurrent use.
type Balancer interface {
	// Pick returns one endpoint from eps for the given info. eps is the already
	// filtered eligible set; Pick must not mutate it and must not retain it
	// beyond the call. It returns [ErrNoEndpoints] when eps is empty.
	Pick(eps []discovery.Endpoint, info PickInfo) (discovery.Endpoint, error)

	// Complete marks the request that Pick issued to ep as finished. ctx is the
	// request's own context: it carries that request's trace and values and is
	// read for logging and instrumentation only — a strategy must not cancel it.
	// err is the request's final error, nil on success.
	//
	// Strategies that keep per-request state settle it here (least-conn decrements
	// its in-flight count); stateless strategies declare a no-op, so callers never
	// nil-check. Keeping settlement in a method rather than a per-pick closure
	// keeps the hot path allocation-free.
	//
	// Callers must invoke it exactly once per picked endpoint, after the request
	// ends. A repeated call cannot be made harmless while other requests are in
	// flight, because stateful strategies count per endpoint rather than per
	// request.
	Complete(ctx context.Context, ep discovery.Endpoint, err error)
}
