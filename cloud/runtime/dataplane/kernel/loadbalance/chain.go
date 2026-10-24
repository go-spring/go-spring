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
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/matcher"
	"go-spring.org/log"
)

// Chain is the OUTER layer of client-side load balancing: the egress a request
// path talks to, and the object that follows the governance selection for its
// label. It owns a whole endpoint-selection chain — the [Balancer], its
// [Tracker] and the inner [Pool] that binds them to a discovery source — as an
// immutable generation, REBUILT FROM SCRATCH whenever the selection its label
// resolves to moves to a new generation. Callers hold the chain, not the pool:
// [Chain.Pick], [Chain.Complete], [Chain.Tracker] and [Chain.Selection] are the
// outward surface, each delegating to the generation in force.
//
// A changed selection means a new balancer, a new tracker and a new pool,
// sharing nothing with the generation it replaces. After a config change nothing
// from the old configuration is still in force — which is the property a Chain
// exists for, and also its price: in-flight counts, a hash ring and observed
// failures all start over. The alternative would be to retune the pool in place,
// which is exactly what a Chain is built to avoid: a change must not leave any
// of the configuration it replaced still deciding traffic.
//
// # The chain pulls; the manager does not push
//
// [Manager.Apply] publishes a generation and stops there. The chain holds a
// [SelectionSource] — the manager — and asks it on every request what its label
// resolves to; when the generation it hears is not the one its pool was built
// from, it rebuilds. So the chain is the STABLE SHELL and the [Pool] is the
// replaceable state behind it, exactly as the resilience executor is a stable
// shell over a replaceable [execState]: a request path talks to the shell and
// never to the pool underneath.
//
// The label IS the key: several chains may share a label (two clients on one
// service name, a rebuilt one), and each follows the label's generation
// independently.
//
// A Chain is safe for concurrent use. A request path reads the generation in
// force on every call; a rebuild swaps it atomically, so a Pick never sees a
// half-built generation. Only the first request after a config change pays for
// the rebuild — every other one finds the generation already current.
type Chain struct {
	// src is the chain's endpoint source, fixed for the chain's whole life: a
	// rebuild re-reads the same source, it does not re-resolve it. A chain whose
	// TARGET changes — another service, another discovery backend — is a new
	// chain, and that is the caller's decision, because only the caller knows
	// what its upstream points at.
	src discovery.Resolver

	// label is the governance label this chain follows, and never changes.
	label string

	// source is the manager, and the chain's answer to "has my label's selection
	// moved?". Fixed at construction — the chain is built by the manager it
	// follows.
	source SelectionSource

	// cur is the generation in force: the pool a request pays attention to. It is
	// read on every request and replaced by a rebuild, so it is an atomic pointer
	// — neither side ever blocks on the other.
	cur atomic.Pointer[Pool]

	// wantGen is the source generation this chain has moved to, and it is the
	// whole of the request path's cost: a request reads it, compares, and goes on
	// its way. It is set as soon as a generation is owed — INCLUDING when the
	// build then fails, so a bad rule costs one attempt, not one per request until
	// the selection changes. It is a pointer so that "no generation moved to yet"
	// is distinguishable from generation zero, which is a real value.
	wantGen atomic.Pointer[uint64]

	// gen counts the rebuilds performed. It is an inspection value (see
	// [Chain.Generation]), never consulted by the request path.
	gen atomic.Uint64

	// buildMu serializes builds. It is taken only when a generation is owed —
	// never on the fast path, where wantGen already matches.
	buildMu sync.Mutex
}

// newChain assembles an unarmed chain: fields set, no generation built. Building
// the first generation is [Manager.Chain]'s job — it resolves the label's
// current selection and builds the generation that selection resolves to.
func newChain(src discovery.Resolver, label string, source SelectionSource) *Chain {
	return &Chain{src: src, label: label, source: source}
}

// Pick selects one live, healthy, non-suspended endpoint from the generation in
// force. It is the chain-level egress: a request path talks to the chain and
// never to the pool underneath. See [Pool.Pick] for the filtering and fallback
// semantics.
//
// Pick moves the chain to its label's current generation first, and then reads
// it once. A rebuild between a Pick and its [Chain.Complete] settles the request
// against the new generation — harmless, because a rebuild already discards the
// previous generation's least-conn and suspension state wholesale.
//
// ctx is the request's, and is threaded to the pool's endpoint source.
func (c *Chain) Pick(ctx context.Context, info PickInfo) (discovery.Endpoint, error) {
	return c.current().Pick(ctx, info)
}

// Complete settles the request that Pick issued to ep. Invoke it exactly once
// per picked endpoint, after the request ends. See [Pool.Complete].
func (c *Chain) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	c.current().Complete(ctx, ep, err)
}

// Tracker returns the suspension tracker of the generation in force. It is an
// inspection and test helper.
func (c *Chain) Tracker() *Tracker { return c.current().Tracker() }

// Selection returns the selection the generation in force was built from. It is
// an inspection and test helper. See [Pool.Selection].
func (c *Chain) Selection() Selection { return c.current().Selection() }

// Label returns the governance label the chain follows. It is an inspection
// helper.
func (c *Chain) Label() string { return c.label }

// Generation returns how many generations the chain has built, the first being
// 1. Each rebuild makes it grow by one, so a test can tell a real rebuild from a
// push that did not move this chain. It is an inspection and test helper, and
// like the other ones it moves the chain first — a chain follows the config on
// the request path, so a rebuild is only ever observed after something asks for
// the generation in force.
//
// Generations that a push moves past before any request arrives are COALESCED:
// the chain moves straight to the current one, so the count says how many times
// the chain rebuilt, not how many times the config changed.
func (c *Chain) Generation() uint64 {
	c.sync(context.Background())
	return c.gen.Load()
}

// current returns the generation in force, having first moved the chain onto
// whatever its label resolves to now. The move is a no-op — an atomic load and an
// integer comparison — on every request but the first after a config change.
func (c *Chain) current() *Pool {
	c.sync(context.Background())
	return c.cur.Load()
}

// sync brings the chain to the generation its label now resolves to. It is the
// whole of the chain's refresh process, and the chain owns it: the manager only
// answers what a label's selection IS and which generation that is (see
// [SelectionSource]).
//
// A strategy the selection names that fails to build is not fatal: the current
// generation stays in force, so a bad rule degrades a chain to its last good
// configuration instead of taking its client down — the rule the governance
// source's contract forces, since it has no error channel. The generation is
// CONSUMED even then, so the failure costs one build, not one per request.
func (c *Chain) sync(ctx context.Context) {
	sel, gen := c.source.ResolveSelection(c.label)

	// The fast path, and the whole of a request's cost once the chain is current:
	// one atomic compare.
	if w := c.wantGen.Load(); w != nil && *w == gen {
		return
	}

	// A build is owed. It runs without blocking the request path's way: only the
	// goroutine that wins buildMu builds, and the others return to the generation
	// in force rather than waiting on a strategy factory they did not ask for.
	c.buildMu.Lock()
	defer c.buildMu.Unlock()
	if w := c.wantGen.Load(); w != nil && *w == gen {
		return // another request moved the chain while this one waited
	}
	// Consume the generation BEFORE building, so a refused build is not retried on
	// every request.
	c.wantGen.Store(&gen)

	p, err := c.build(sel)
	if err != nil {
		log.Warn(ctx, log.TagAppDef,
			"loadbalance: endpoint selection rejected, keeping the resource's current strategy",
			log.String("label", c.label),
			log.Err(err))
		return
	}
	// Store only if the generation this pool was built for is still the one
	// wanted: a config change that landed while the build ran has already
	// consumed a newer generation, and this pool is its predecessor.
	if w := c.wantGen.Load(); w != nil && *w == gen {
		c.cur.Store(p)
		c.gen.Add(1)
	}
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
	// A selector on the selection narrows the endpoint set before the strategy
	// sees it. It is applied to the source's output rather than pushed into the
	// resolver, so the dynamic rule composes with whatever static narrowing the
	// resolver already does (both must hold) and needs no re-resolution of the
	// chain's target.
	src := c.src
	if len(s.Selector) > 0 {
		src = selectorSource(c.src, s.Selector)
	}
	p := NewPool(src, bal, WithTrackerConfig(TrackerConfig{
		Threshold:  s.OutlierThreshold,
		SuspendFor: s.OutlierSuspendFor,
	}))
	// Record the resolved selection as the policy in force, so a chain's
	// generation reads back what it was built from (see [Pool.Selection]).
	p.recordSelection(s)
	return p, nil
}

// selectorSource returns a resolver that narrows the endpoints src returns to
// those whose metadata satisfies s. The selector is captured by value, so a
// generation's filter is immutable for that generation's life: a pushed change
// builds a new generation instead of mutating one in place.
func selectorSource(src discovery.Resolver, s matcher.Selector) discovery.Resolver {
	return func(ctx context.Context) ([]discovery.Endpoint, error) {
		eps, err := src(ctx)
		if err != nil {
			return nil, err
		}
		return discovery.Narrow(eps, discovery.Query{Selector: s}), nil
	}
}
