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
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
)

// The endpoint source of a Pool is a [discovery.Resolver] — a plain
// `func() ([]Endpoint, error)`, typically bound to one service name via
// [discovery.NewResolver] — re-read live on every Pick. A source error (a
// registry hiccup, an unknown service) propagates out of [Pool.Pick] instead
// of being treated as "no endpoints".
// Pool is the runtime that ties client-side load balancing together. On every
// [Pool.Pick] it takes the live endpoint snapshot from its
// [discovery.Resolver] (kept fresh by discovery Watch), filters it, and hands
// the survivors to a [Balancer] to choose one.
//
// The filter chain has three stages: static admission (Disabled or unhealthy
// per the discovery endpoint flags), suspension ([Tracker], for instances
// still registered but failing), and soft drain (zero weight at the naming
// service). Pick owns the fallbacks: a filter that would empty the set is
// skipped, except Disabled — a disabled endpoint never receives traffic even
// when it is the only one left.
type Pool struct {
	src     discovery.Resolver
	bal     atomic.Pointer[Balancer]
	tracker *Tracker
	sel     atomic.Pointer[Selection]

	// selMu serializes the read-modify-write of sel in [Pool.ApplySelection], so
	// two concurrent policy applications cannot drop each other's strategy name.
	// It guards nothing on the hot path: Pick and Complete never touch it.
	selMu sync.Mutex
}

// PoolOption configures a [Pool].
type PoolOption func(*Pool)

// WithTrackerConfig sets the pool's initial outlier-suspension config. The
// pool builds and owns its [Tracker]; callers that need the tracker itself
// (inspection, or a non-Pool adapter such as the gRPC balancer) reach it
// through [Pool.Tracker] or build one with [NewTracker] directly.
func WithTrackerConfig(cfg TrackerConfig) PoolOption {
	return func(p *Pool) { p.tracker = NewTracker(cfg) }
}

// NewPool builds a [Pool] over src using balancer bal. The candidate set
// follows the naming service in real time; bal is any strategy from this
// package.
//
// The pool owns a [Tracker], starting disabled (threshold 0 — fully
// transparent) so every pool is governable: a governance rule that sets
// outlier thresholds takes effect without extra wiring. Set initial thresholds
// with [WithTrackerConfig].
func NewPool(src discovery.Resolver, bal Balancer, opts ...PoolOption) *Pool {
	p := &Pool{src: src, tracker: NewTracker(TrackerConfig{})}
	p.bal.Store(&bal)
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ApplySelection applies a resolved endpoint-selection policy in place: the
// balancer strategy named by s (with its parameters) and the suspension
// thresholds. It is the whole-selection entry point a governance subscriber
// calls, and it takes the package's own [Selection] value so the policy model
// stays out of this package's API.
//
// An empty s.Balancer leaves the current strategy alone. A strategy name that
// fails to build — unknown, or a parameter aimed at another strategy — is
// IGNORED rather than fatal: the pool degrades to the last good strategy
// instead of taking the client down, and the build error is RETURNED for the
// caller to surface (the governance [Source] contract has no error channel —
// "everything you push, you vouch for" — so the pusher's own logging is the
// rejection's only trace). The suspension half is always applied, so a rule
// that only retunes thresholds still works.
//
// This is the sink [Pool.BindSelection] points at a managed policy; the values
// it accepted are readable back through [Pool.Selection].
func (p *Pool) ApplySelection(s Selection) error {
	p.selMu.Lock()
	sel := p.Selection()
	var buildErr error
	if s.Balancer != "" {
		params := s.BalancerConfig()
		bal, err := New(s.Balancer, params)
		switch {
		case err != nil:
			buildErr = err
		default:
			// In-place swap, lock-free on the hot path. The old strategy's own
			// state (least_conn in-flight counts, a hash ring, p2c's latency
			// model) is not carried across — a switch starts it over.
			p.bal.Store(&bal)
			sel.Balancer = s.Balancer
			sel.BalancerReplicas = s.BalancerReplicas
			sel.BalancerZoneKey = s.BalancerZoneKey
			sel.BalancerDelegate = s.BalancerDelegate
		}
	}
	sel.OutlierThreshold = s.OutlierThreshold
	sel.OutlierSuspendFor = s.OutlierSuspendFor
	p.sel.Store(&sel)
	p.selMu.Unlock()

	// The tracker retunes in place, keeping the per-endpoint failure state so a
	// mid-flight threshold change does not forget what the pool already observed.
	p.tracker.SetConfig(TrackerConfig{Threshold: s.OutlierThreshold, SuspendFor: s.OutlierSuspendFor})
	return buildErr
}

// Tracker returns the pool's suspension tracker. Every pool owns one (a fully
// transparent disabled tracker unless [WithTrackerConfig] set thresholds). It
// is an inspection and test helper.
func (p *Pool) Tracker() *Tracker { return p.tracker }

// Pick selects one live, healthy, non-suspended endpoint via the configured
// balancer. The caller must invoke [Pool.Complete] with the picked endpoint
// when the request finishes: it advances least-conn accounting and feeds the
// suspension tracker, so skipping it defeats health suspension.
//
// It returns [ErrNoAvailable] when the source has no endpoints, or every
// endpoint is disabled.
func (p *Pool) Pick(info PickInfo) (discovery.Endpoint, error) {
	eps, err := p.src()
	if err != nil {
		return discovery.Endpoint{}, err
	}
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
	}

	// Static admission: the shared Endpoint contract filter (never Disabled,
	// prefer Healthy). It returns empty only when every endpoint is disabled —
	// an explicit exclusion that must not be overridden by any fallback below.
	candidates := admission(eps)
	if len(candidates) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
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

	return (*p.bal.Load()).Pick(candidates, info)
}

// Complete settles the request that Pick issued to ep: it advances the
// balancer's own accounting and records the outcome with the suspension
// tracker, so every strategy — not just least-conn — feeds suspension.
// Invoke it exactly once per picked endpoint, after the request ends.
func (p *Pool) Complete(ep discovery.Endpoint, err error) {
	(*p.bal.Load()).Complete(ep, err)
	p.tracker.Record(ep.Addr, err == nil)
}

// admission returns the endpoints allowed to receive traffic under the
// [discovery.Endpoint] contract: prefer !Disabled && Healthy, degrade to
// !Disabled when none are healthy, never Disabled. It is a pure filter: empty
// only when every endpoint is disabled — an explicit exclusion the caller must
// not override with a fallback.
func admission(eps []discovery.Endpoint) []discovery.Endpoint {
	out := eps[:0:0]
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
