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
	"maps"
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
)

// Chain is the OUTER layer of client-side load balancing: the egress a request
// path talks to, and the receiver of dynamic governance refreshes. It owns a
// whole endpoint-selection chain — the [Balancer], its [Tracker] and the inner
// [Pool] that binds them to a discovery source — as an immutable generation,
// REBUILT FROM SCRATCH whenever the governance selection resolved for its label
// changes. Callers hold the chain, not the pool: [Chain.Pick], [Chain.Complete],
// [Chain.Tracker] and [Chain.Selection] are the outward surface, each delegating
// to the generation in force.
//
// A changed selection means a new balancer, a new tracker and a new pool,
// sharing nothing with the generation it replaces. After a config change nothing
// from the old configuration is still in force — which is the property a Chain
// exists for, and also its price: in-flight counts, a hash ring and observed
// failures all start over. The alternative would be to retune the pool in place,
// which is exactly what a Chain is built to avoid: a change must not leave any
// of the configuration it replaced still deciding traffic.
//
// The chain owns its own refresh: [Manager.Apply] resolves what the chain's
// label's selection IS and hands it over, and [Chain.refresh] decides whether it
// moved — a push that does not change this label's selection costs the chain
// nothing.
//
// The label IS the key: a chain registers itself as a subscriber of its label
// (see [Manager.Chain]), so one label may legitimately back several chains
// (two clients sharing a service name, a rebuilt one) — each subscribes
// independently, and a label whose selection moves rebuilds every chain under it.
//
// A Chain is safe for concurrent use. A request path reads the generation in
// force on every call; a rebuild swaps it atomically, so a rebuild never blocks
// a Pick and a Pick never sees a half-built generation.
type Chain struct {
	// src is the chain's endpoint source, fixed for the chain's whole life: a
	// rebuild re-reads the same source, it does not re-resolve it. A chain whose
	// TARGET changes — another service, another discovery backend — is a new
	// chain, and that is the caller's decision, because only the caller knows
	// what its upstream points at.
	src discovery.Resolver

	// label is the governance label this chain subscribes to, and never changes.
	label string

	// cur is the generation in force: the pool a request pays attention to. It is
	// read on every request and replaced by a rebuild, so it is an atomic pointer
	// — neither side ever blocks on the other.
	cur atomic.Pointer[Pool]

	// mu serializes rebuilds so two pushes cannot interleave a build and leak a
	// generation. It guards nothing on the request path. It also guards last,
	// the selection the current generation was built from.
	mu   sync.Mutex
	gen  uint64
	last Selection
}

// newChain assembles an unarmed chain: fields set, no generation built. Building
// the first generation is [Manager.Chain]'s job — it resolves the label's
// current selection and builds the generation that selection resolves to.
func newChain(src discovery.Resolver, label string) *Chain {
	return &Chain{src: src, label: label}
}

// Pick selects one live, healthy, non-suspended endpoint from the generation in
// force. It is the chain-level egress: a request path talks to the chain and
// never to the pool underneath. See [Pool.Pick] for the filtering and fallback
// semantics.
//
// Pick reads the current generation once. A rebuild between a Pick and its
// [Chain.Complete] settles the request against the new generation — harmless,
// because a rebuild already discards the previous generation's least-conn and
// suspension state wholesale.
func (c *Chain) Pick(info PickInfo) (discovery.Endpoint, error) {
	return c.cur.Load().Pick(info)
}

// Complete settles the request that Pick issued to ep. Invoke it exactly once
// per picked endpoint, after the request ends. See [Pool.Complete].
func (c *Chain) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	c.cur.Load().Complete(ctx, ep, err)
}

// Tracker returns the suspension tracker of the generation in force. It is an
// inspection and test helper.
func (c *Chain) Tracker() *Tracker { return c.cur.Load().Tracker() }

// Selection returns the selection the generation in force was built from. It is
// an inspection and test helper. See [Pool.Selection].
func (c *Chain) Selection() Selection { return c.cur.Load().Selection() }

// Label returns the governance label the chain follows. It is an inspection
// helper.
func (c *Chain) Label() string { return c.label }

// Generation returns how many generations the chain has built, the first being
// 1. Each rebuild makes it grow by one, so a test can tell a real rebuild from a
// push that did not move this chain. It is an inspection and test helper.
func (c *Chain) Generation() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen
}

// refresh moves the chain to s: a no-op when s is the selection the current
// generation was already built from (a push that does not move this label costs
// the chain nothing), and otherwise a rebuild from scratch. The chain — not the
// manager — owns this decision, which IS the refresh process: the manager only
// resolves what a label's selection is and hands it over (see [Manager.Apply]).
//
// A strategy the selection names that fails to build is an error and the current
// generation stays in force, so a bad rule degrades a chain to its last good
// configuration instead of taking its client down — the rule the governance
// source's contract forces, since it has no error channel. A refused selection
// is still CONSUMED: last moves to it, so it is not retried until it changes.
func (c *Chain) refresh(s Selection) error {
	c.mu.Lock()
	if c.gen > 0 && selectionEqual(c.last, s) {
		c.mu.Unlock()
		return nil
	}
	c.last = s // consumed even on a refused build: no retry until it changes
	c.mu.Unlock()

	// Assemble outside the lock: building runs a strategy factory, which is
	// caller-supplied code, and a request path must not wait on it to read the
	// current generation.
	p, err := c.build(s)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.gen++
	c.cur.Store(p)
	c.mu.Unlock()
	return nil
}

// build assembles one INNER-LAYER generation from s: the strategy the selection
// names — or the base strategy, round-robin, when the document names none —
// plus a fresh tracker carrying the selection's suspension thresholds, bound
// together by a new [Pool].
//
// The resolved selection is the COMPLETE truth for this label, which is what
// makes the empty strategy name mean "the base strategy" here where in-place
// application reads it as "keep what is in force" ([Selection.Balancer]). The
// base is round-robin. Under a rebuild there is nothing to keep, and the
// fallback must be stated rather than inherited: an earlier generation is gone
// by design.
func (c *Chain) build(s Selection) (*Pool, error) {
	var bal Balancer = NewRoundRobin()
	if s.Balancer != "" {
		b, err := New(s.Balancer, NewParams(s.Params))
		if err != nil {
			return nil, err
		}
		bal = b
	}
	p := NewPool(c.src, bal, WithTrackerConfig(TrackerConfig{
		Threshold:  s.OutlierThreshold,
		SuspendFor: s.OutlierSuspendFor,
	}))
	// Record the resolved selection as the policy in force, so a chain's
	// generation reads back what it was built from (see [Pool.Selection]).
	p.recordSelection(s)
	return p, nil
}

// selectionEqual reports whether two resolved selections are equivalent for the
// purpose of refresh change detection. [Selection] carries a parameter map and
// so is not comparable with ==; every field is compared explicitly instead.
func selectionEqual(a, b Selection) bool {
	return a.Balancer == b.Balancer &&
		maps.Equal(a.Params, b.Params) &&
		a.OutlierThreshold == b.OutlierThreshold &&
		a.OutlierSuspendFor == b.OutlierSuspendFor
}
