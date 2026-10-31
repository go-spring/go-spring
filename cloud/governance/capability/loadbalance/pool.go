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

	"go-spring.org/cloud/governance/capability/discovery"
)

// Pool is the INNER layer of client-side load balancing: ONE immutable
// generation of a complete endpoint-selection chain — a [discovery.Resolver]
// source, the filter chain, a [Balancer] and its [Tracker] — built once and
// never retuned. It is what a [Chain] assembles (see buildGeneration) and swaps
// whole on a governance refresh; request paths reach it THROUGH the chain, via
// [Chain.Pick] and [Chain.Complete]. Building one directly with [NewPool] is
// the standalone case, for callers with no governance manager behind them.
//
// The endpoint source of a Pool is a [discovery.Resolver] — a plain
// `func(ctx context.Context) ([]Endpoint, error)`, typically bound to one
// service name via [discovery.NewResolver] — re-read live on every Pick. A
// source error (a registry hiccup, an unknown service) propagates out of
// [Pool.Pick] instead of being treated as "no endpoints".
// On every [Pool.Pick] it takes the live endpoint snapshot from its
// [discovery.Resolver] (kept fresh by discovery Watch), filters it, and hands
// the survivors to a [Balancer] to choose one.
//
// The filter chain has four stages: the selection's dynamic selector (metadata
// terms from the governance document), static eligibility (Disabled or
// unhealthy per the discovery endpoint flags), suspension ([Tracker], for
// instances still registered but failing), and soft drain (zero weight at the
// naming service). Pick owns the fallbacks: a filter that would empty the set is
// skipped, except Disabled and the selector — a disabled endpoint never receives
// traffic, and a narrowing rule must not silently widen, even when it leaves
// nothing.
type Pool struct {
	// src is the endpoint source, re-read on every Pick. It is the chain's BASE
	// source: a selector narrows its output inside Pick instead of wrapping it.
	src discovery.Resolver

	// bal is the strategy in force, fixed at construction: a changed selection
	// replaces the pool rather than retuning it.
	bal Balancer

	// tracker is the pool's own outlier-suspension state, built from the
	// selection's thresholds.
	tracker *Tracker

	// sel is the selection this pool was built from, fixed at construction. It
	// travels with the pool under the chain's atomic pointer, so a reader never
	// pairs a pool with another generation's selection. Zero for a standalone
	// pool built directly with [NewPool], which has no selection document.
	sel Selection
}

// NewPool builds a [Pool] over src using balancer bal and tracker tracker, and
// records sel as the selection it was built from. The collaborators are ones
// the caller has already built: the pool creates nothing and assumes neither
// src nor bal nor tracker is nil.
//
// src is the BASE source, the same in every generation of a chain: the
// selection's selector is a filtering stage inside [Pool.Pick], not a wrapper
// around the source. That is what keeps the source a stable way for the manager
// to rebuild a chain (see [Manager.adopt]).
//
// Build the tracker with [NewTracker]; a zero [TrackerConfig] is the disabled
// state ("no suspension"), stated at the call site. Ownership passes to the
// pool: a tracker is one generation's runtime state, so sharing one between
// pools would couple suspension decisions that a rebuild exists to separate.
//
// A standalone caller — one with no governance document behind it — passes
// Selection{}, which [Pool.Selection] reports as the zero value.
func NewPool(src discovery.Resolver, bal Balancer, tracker *Tracker, sel Selection) *Pool {
	return &Pool{src: src, bal: bal, tracker: tracker, sel: sel}
}

// Selection returns the selection this pool was built from. It is the chain's
// change-test record and its inspection surface, so "what is serving" and "what
// to compare the next push against" are the same value. Zero for a pool built
// directly with [NewPool].
func (p *Pool) Selection() Selection { return p.sel }

// Pick selects one live, healthy, non-suspended endpoint via the configured
// balancer. The caller must invoke [Pool.Complete] with the picked endpoint
// when the request finishes: it advances least-conn accounting and feeds the
// suspension tracker, so skipping it defeats health suspension.
//
// ctx is the request's context, threaded to the endpoint source. The source is
// usually a cache read, but a backend may refresh on demand, and that read owes
// the request its deadline, cancellation and trace.
//
// It returns [ErrNoEndpoints] when the source has no endpoints, or every
// endpoint is disabled.
func (p *Pool) Pick(ctx context.Context, info PickInfo) (discovery.Endpoint, error) {
	eps, err := p.src(ctx)
	if err != nil {
		return discovery.Endpoint{}, err
	}

	// The selection's selector: the dynamic half of endpoint narrowing, applied
	// here rather than baked into the source, so the source stays the chain's
	// stable target across generations. It is a HARD filter — no fallback when it
	// matches nothing, because silently widening would turn a canary into
	// full traffic.
	if len(p.sel.Selector) > 0 {
		eps = discovery.FilterByMetadata(eps, p.sel.Selector)
	}
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}

	// Static eligible: the shared Endpoint contract filter (never Disabled,
	// prefer Healthy). It returns empty only when every endpoint is disabled —
	// an explicit exclusion that must not be overridden by any fallback below.
	candidates := eligible(eps)
	if len(candidates) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}

	// The filters below are pure: when one would empty the set, the pre-filter
	// set stays — black-holing the pool is worse than probing degraded
	// instances.

	// Suspension: drop instances the tracker has suspended for repeated
	// failures.
	if active := p.tracker.Admissible(candidates); len(active) > 0 {
		candidates = active
	}

	// Soft drain: drop instances whose weight was set to zero at the naming
	// service (the runtime traffic-drain signal).
	if live := excludeDrained(candidates); len(live) > 0 {
		candidates = live
	}

	return p.bal.Pick(candidates, info)
}

// Complete settles the request that Pick issued to ep: it advances the
// balancer's own accounting and records the outcome with the suspension
// tracker, so every strategy — not just least-conn — feeds suspension.
// Invoke it exactly once per picked endpoint, after the request ends.
//
// ctx is the context of that request — the one whose outcome this is — and is
// threaded through to the balancer and the tracker so their logs and
// instrumentation carry the request's trace and values. It is read, never
// canceled, by this call.
func (p *Pool) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	p.bal.Complete(ctx, ep, err)
	p.tracker.Record(ctx, ep.Addr, err == nil)
}

// eligible returns the endpoints allowed to receive traffic under the
// [discovery.Endpoint] contract: prefer !Disabled && Healthy, degrade to
// !Disabled when none are healthy, never Disabled. It is a pure filter: empty
// only when every endpoint is disabled — an explicit exclusion the caller must
// not override with a fallback.
func eligible(eps []discovery.Endpoint) []discovery.Endpoint {
	out := make([]discovery.Endpoint, 0, len(eps))
	for _, ep := range eps {
		if !ep.Disabled && ep.Healthy {
			out = append(out, ep)
		}
	}
	if len(out) == 0 {
		for _, ep := range eps {
			if !ep.Disabled {
				out = append(out, ep)
			}
		}
	}
	return out
}

// excludeDrained drops endpoints with an explicit zero weight (the drain
// signal). Negative weights are kept — misconfiguration should not silently
// remove an instance; the weighted balancer still treats them as default.
// It is a pure filter: when every endpoint is drained it returns nil and the
// fallback belongs to the caller. Callers may legitimately see all-zero
// snapshots, where registrants predate the weight contract and store 0 for
// "default".
func excludeDrained(eps []discovery.Endpoint) []discovery.Endpoint {
	var kept []discovery.Endpoint
	for _, ep := range eps {
		if ep.Weight != 0 {
			kept = append(kept, ep)
		}
	}
	return kept
}
