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
	"sync/atomic"
	"time"

	"go-spring.org/cloud/discovery"
)

// EndpointSource supplies the current live endpoint snapshot, surfacing any
// error from the underlying discovery read (a registry hiccup, an unknown
// service) rather than hiding it. A [discovery.Resolver] — typically bound to one
// service name via [discovery.NewResolver] — has exactly this shape; wrap one as
// [SourceFunc] to feed a Pool. [Pool.Pick] propagates a source error instead of
// treating it as "no endpoints".
type EndpointSource interface {
	Endpoints() ([]discovery.Endpoint, error)
}

// SourceFunc adapts a plain reader (such as a [discovery.Resolver]) into an
// [EndpointSource], so a consumer that already holds a bound by-name read can
// hand it straight to [NewPool].
type SourceFunc func() ([]discovery.Endpoint, error)

// Endpoints implements [EndpointSource].
func (f SourceFunc) Endpoints() ([]discovery.Endpoint, error) { return f() }

// Pool is the runtime that ties client-side load balancing together. On every
// [Pool.Pick] it takes the live endpoint snapshot from an [EndpointSource]
// (kept fresh by discovery Watch), filters it, and hands the survivors to a
// [Balancer] to choose one.
//
// The filter chain has three stages: static admission (Disabled or unhealthy
// per the discovery endpoint flags), suspension ([Tracker], for instances
// still registered but failing), and soft drain (zero weight at the naming
// service). Pick owns the fallbacks: a filter that would empty the set is
// skipped, except Disabled — a disabled endpoint never receives traffic even
// when it is the only one left.
type Pool struct {
	src     EndpointSource
	bal     atomic.Pointer[Balancer]
	tracker *Tracker
}

// PoolOption configures a [Pool].
type PoolOption func(*Pool)

// WithTracker attaches an outlier-suspension [Tracker] to the pool. Without it,
// suspension is disabled and only the discovery endpoint flags filter endpoints.
func WithTracker(t *Tracker) PoolOption {
	return func(p *Pool) { p.tracker = t }
}

// NewPool builds a [Pool] over src using balancer bal. src is typically a
// discovery.Resolver so the candidate set follows the naming service in real
// time; bal is any strategy from this package.
func NewPool(src EndpointSource, bal Balancer, opts ...PoolOption) *Pool {
	p := &Pool{src: src}
	p.bal.Store(&bal)
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// SetBalancer replaces the load-balancing strategy in place. It is the hot-path
// counterpart of the balancer chosen at construction: a governance policy that
// names a strategy can switch it on a live pool.
//
// Be aware that a strategy carries its own state — least_conn its in-flight
// counts, consistent_hash its ring, p2c its latency model. Changing strategy
// starts that state over; it is not carried across. Safe for concurrent use
// with [Pool.Pick] and [Pool.Complete].
func (p *Pool) SetBalancer(bal Balancer) {
	p.bal.Store(&bal)
}

// SetTrackerConfig retunes outlier suspension in place. The per-endpoint
// failure state is kept (see [Tracker.SetConfig]), so tightening or relaxing
// the threshold mid-flight does not forget what the pool already observed.
// It is a no-op when the pool has no tracker attached — attach one with
// [WithTracker] to make suspension governable at all.
func (p *Pool) SetTrackerConfig(cfg TrackerConfig) {
	if p.tracker != nil {
		p.tracker.SetConfig(cfg)
	}
}

// ApplySelection applies a resolved endpoint-selection policy in place: the
// balancer strategy named by balancer and the suspension thresholds. It is the
// whole-selection entry point a governance subscriber calls, and it takes plain
// values so this package needs no dependency on the policy model.
//
// An empty balancer leaves the current strategy alone, and an unknown strategy
// name is IGNORED rather than fatal: the governance [Source] contract has no
// error channel ("everything you push, you vouch for"), so a bad rule degrades
// to the last good strategy instead of taking the client down. The suspension
// half is always applied, so a rule that only retunes thresholds still works.
func (p *Pool) ApplySelection(balancer string, threshold int, suspendFor time.Duration) {
	if balancer != "" {
		if bal, err := New(balancer); err == nil {
			p.SetBalancer(bal)
		}
	}
	p.SetTrackerConfig(TrackerConfig{Threshold: threshold, SuspendFor: suspendFor})
}

// Tracker returns the suspension tracker attached to the pool, or nil when
// none was attached. It is an inspection and test helper.
func (p *Pool) Tracker() *Tracker { return p.tracker }

// Pick selects one live, healthy, non-suspended endpoint via the configured
// balancer. The caller must invoke [Pool.Complete] with the picked endpoint
// when the request finishes: it advances least-conn accounting and feeds the
// suspension tracker, so skipping it defeats health suspension.
//
// It returns [ErrNoAvailable] when the source has no endpoints, or every
// endpoint is disabled.
func (p *Pool) Pick(info PickInfo) (discovery.Endpoint, error) {
	eps, err := p.src.Endpoints()
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
// balancer's own accounting and records the outcome with the suspension tracker
// (when attached), so every strategy — not just least-conn — feeds suspension.
// Invoke it exactly once per picked endpoint, after the request ends.
func (p *Pool) Complete(ep discovery.Endpoint, err error) {
	(*p.bal.Load()).Complete(ep, err)
	if p.tracker != nil {
		p.tracker.Record(ep.Addr, err == nil)
	}
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
