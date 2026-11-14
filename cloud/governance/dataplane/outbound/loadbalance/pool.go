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

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	balancer "go-spring.org/cloud/loadbalance"
)

// Pool is the INNER layer of client-side load balancing: ONE immutable
// generation of a complete endpoint-selection chain — a [discovery.Resolver]
// source, the filter chain, a [Balancer] and its [Tracker] — built once and
// never retuned. It is what a [Chain] assembles (see buildPool) and swaps whole
// on a governance refresh; request paths reach it THROUGH the chain, via
// [Chain.Pick] and the [chain.Peer] it returns. Building one directly with
// [NewPool] is the standalone case, for callers with no governance manager
// behind them.
//
// The endpoint source of a Pool is a [discovery.Resolver] — a plain
// `func(ctx context.Context) ([]Endpoint, error)`, typically bound to one
// service name via [discovery.NewResolver] — re-read live on every Pick. Its
// snapshot (kept fresh by discovery Watch) is filtered, and the survivors are
// handed to a [Balancer] to choose one. A source error (a registry hiccup, an
// unknown service) propagates out of [Pool.Pick] instead of being treated as
// "no endpoints".
//
// The filter chain has four stages, in order: the selection's metadata match
// (terms from the governance document), soft drain (zero weight at the naming
// service), the Disabled exclusion (an operator decree), and the [Tracker]'s
// servable-set decision — health preference, suspension, and the
// MinHealthyPercent panic over one shared denominator. Pick owns the fallbacks:
// drain falls back to the pre-drain set, and the tracker's decision falls back
// to the whole pool; Disabled and the metadata match are hard — a disabled
// endpoint never receives traffic, and a narrowing rule must not silently
// widen, even when it leaves nothing.
type Pool struct {
	// resolver is the endpoint source, re-read on every Pick. It is the chain's
	// BASE source: the selection's metadata match narrows its output inside Pick
	// instead of wrapping it.
	resolver discovery.Resolver

	// bal is the strategy in force, fixed at construction: a changed selection
	// replaces the pool rather than retuning it.
	bal balancer.Balancer

	// tracker is the pool's own outlier-suspension state, built from the
	// selection's thresholds.
	tracker *Tracker

	// sel is the selection this pool was built from, fixed at construction. It
	// travels with the pool under the chain's atomic pointer, so a reader never
	// pairs a pool with another generation's selection. Zero for a standalone
	// pool built directly with [NewPool], which has no selection document.
	sel Selection
}

// NewPool builds a [Pool] over resolver using balancer bal and tracker tracker,
// and records sel as the selection it was built from. The collaborators are
// ones the caller has already built: the pool creates nothing.
//
// resolver is the BASE source, the same in every generation of a chain: the
// selection's metadata match is a filtering stage inside [Pool.Pick], not a
// wrapper around the source. That is what keeps the source a stable way for the
// manager to rebuild a chain (see [Manager.Apply]).
//
// Build the tracker with [NewTracker]; a zero [TrackerConfig] is the disabled
// state ("no suspension"), stated at the call site. Ownership passes to the
// pool: a tracker is one generation's runtime state, so sharing one between
// pools would couple suspension decisions that a rebuild exists to separate.
// Health preference, suspension and the MinHealthyPercent floor are decided by
// the tracker from its config over the pool Pick hands it; the manager builds
// both from the same [Selection], so standalone callers should do likewise.
//
// A standalone caller — one with no governance document behind it — passes
// Selection{}, which [Pool.Selection] reports as the zero value.
//
// All four parts are required; the constructor does not guard them, because a
// pool is assembled by code that already owns them.
func NewPool(resolver discovery.Resolver, bal balancer.Balancer, tracker *Tracker, sel Selection) *Pool {
	return &Pool{resolver: resolver, bal: bal, tracker: tracker, sel: sel}
}

// Selection returns the selection this pool was built from. It is the chain's
// change-test record and its inspection surface, so "what is serving" and "what
// to compare the next push against" are the same value. Zero for a pool built
// directly with [NewPool].
func (p *Pool) Selection() Selection { return p.sel }

// PickResult is gone: [Pool.Pick] returns a [chain.Peer] whose complete
// closure is bound to this pool — the generation that made the pick — so a
// governance push landing between Pick and Complete cannot move the
// settlement to a generation that never saw the request. A retired
// generation drops the outcome (see [Tracker.Close]).

// Pick selects one live, healthy, non-suspended endpoint via the configured
// balancer. exclude lists the addresses the calling chain's earlier rounds
// already failed against — the failed-list semantics a failover retry needs —
// and Pick avoids them. A single-endpoint set ignores the exclusion (retrying
// the only instance beats failing the call); a wider set with every candidate
// excluded has nothing left and returns [balancer.ErrNoEndpoints]. The
// chain-level caller passes the exclusion list [chain.Recorder.FailedAddrs]
// produces; a caller with no failover context passes nil.
//
// The caller must invoke [chain.Peer.Complete] on the returned peer when the
// request finishes: it advances the balancer's accounting and feeds the
// suspension tracker, so skipping it defeats health suspension.
//
// ctx is the request's context, threaded to the endpoint source. The source is
// usually a cache read, but a backend may refresh on demand, and that read owes
// the request its deadline, cancellation and trace.
//
// It returns [balancer.ErrNoEndpoints] when the source has no endpoints, the
// metadata match selects none, or every endpoint is disabled.
func (p *Pool) Pick(ctx context.Context, info balancer.PickInfo, exclude []string) (chain.Peer, error) {
	eps, err := p.resolver(ctx)
	if err != nil {
		return chain.Peer{}, err
	}
	// Discovery churn must not leave suspension state behind for addresses that
	// are gone; prune against the raw set before any filter narrows it.
	p.tracker.RemoveStale(ctx, eps)

	// The selection's metadata match: the dynamic half of endpoint narrowing,
	// applied here rather than baked into the source, so the source stays the
	// chain's stable target across generations. It is a HARD filter — no fallback
	// when it matches nothing, because silently widening would turn a canary into
	// full traffic.
	if len(p.sel.MetadataMatch) > 0 {
		eps = discovery.FilterByMetadata(eps, p.sel.MetadataMatch)
	}
	if len(eps) == 0 {
		return chain.Peer{}, balancer.ErrNoEndpoints
	}

	// Soft drain: drop instances whose weight was set to zero at the naming
	// service (the runtime traffic-drain signal). It runs before the tracker's
	// health decision: drain is an operator decree, so an undrained unhealthy
	// instance is a better candidate than a drained healthy one. The
	// all-drained fallback keeps the pre-drain (post-metadata) set, so an
	// all-zero snapshot never black-holes the pool.
	if live := excludeDrained(eps); len(live) > 0 {
		eps = live
	}

	// Eligible pool: never Disabled. It is the denominator the MinHealthyPercent
	// floor and panic measure against, and the set the tracker arbitrates over.
	pool := nonDisabled(eps)
	if len(pool) == 0 {
		return chain.Peer{}, balancer.ErrNoEndpoints
	}

	// The tracker decides the servable subset from the pool: health preference,
	// suspension and the panic floor share one threshold and one denominator,
	// so they are decided in one place. See [Tracker.Available].
	candidates := p.tracker.Available(ctx, pool)

	// Failover exclusion: drop the peers this call already failed against. A
	// one-candidate set keeps the excluded instance (retrying the only instance
	// beats failing the call); a wider set with nothing left reports it.
	if live := excludeAddrs(candidates, exclude); len(live) > 0 {
		candidates = live
	} else if len(candidates) > 1 {
		return chain.Peer{}, balancer.ErrNoEndpoints
	}

	ep, err := p.bal.Pick(candidates, info)
	if err != nil {
		return chain.Peer{}, err
	}
	return chain.NewPeer(ep, func(ctx context.Context, err error) {
		p.Complete(ctx, ep, err)
	}), nil
}

// Close releases the generation's instrumented state. The manager calls it on
// the pool a rebuild replaces; see [Tracker.Close] for why the count must come
// back. Outstanding picks may still settle on this pool — their
// suspension outcomes are dropped as a retired generation's — but it must not
// be picked from again.
func (p *Pool) Close() { p.tracker.Close() }

// Complete settles the request that Pick issued to ep: it advances the
// balancer's own accounting and records the outcome with the suspension
// tracker, so every strategy — not just least-conn — feeds suspension.
// Invoke it exactly once per picked endpoint, after the request ends.
//
// Both consumers classify the outcome for themselves, so Complete hands each
// one the round's own error — the fact, not a classification of it.
//
// ctx is the context of that request — the one whose outcome this is — and is
// threaded through to the balancer and the tracker so their logs and
// instrumentation carry the request's trace and values. It is read, never
// canceled, by this call.
func (p *Pool) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	p.bal.Complete(ctx, ep, err)
	p.tracker.Record(ctx, ep, err)
}

// nonDisabled returns the endpoints not administratively removed. It is the
// eligible pool: never empty unless every endpoint is disabled (an explicit
// exclusion the caller must not override with a fallback), and the common
// denominator for the health preference, suspension and MinHealthyPercent
// panic. Health is deliberately NOT applied here: it is weighed against this
// same pool when the tracker decides the servable subset.
func nonDisabled(eps []discovery.Endpoint) []discovery.Endpoint {
	out := make([]discovery.Endpoint, 0, len(eps))
	for _, ep := range eps {
		if !ep.Disabled {
			out = append(out, ep)
		}
	}
	return out
}

// excludeAddrs drops endpoints whose address is in exclude. It is a pure
// filter: when every endpoint is excluded it returns nil and the fallback
// belongs to the caller.
func excludeAddrs(eps []discovery.Endpoint, exclude []string) []discovery.Endpoint {
	if len(exclude) == 0 {
		return eps
	}
	drop := make(map[string]struct{}, len(exclude))
	for _, a := range exclude {
		drop[a] = struct{}{}
	}
	var kept []discovery.Endpoint
	for _, ep := range eps {
		if _, bad := drop[ep.Addr]; !bad {
			kept = append(kept, ep)
		}
	}
	return kept
}

// excludeDrained drops endpoints whose weight is explicitly zero (the drain
// signal). Negative weights are kept — misconfiguration should not silently
// remove an instance, and the weighted balancer treats them as the default. It
// is a pure filter: when every endpoint is drained it returns nil and the
// fallback belongs to the caller. All-zero snapshots are legitimate (registrants
// predating the weight contract store 0 for "default"), which is why the caller
// falls back to the pre-filter set rather than black-holing the pool.
func excludeDrained(eps []discovery.Endpoint) []discovery.Endpoint {
	var kept []discovery.Endpoint
	for _, ep := range eps {
		if ep.Weight != 0 {
			kept = append(kept, ep)
		}
	}
	return kept
}
