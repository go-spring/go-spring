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
	"go-spring.org/cloud/discovery"
)

// EndpointSource supplies the current live endpoint snapshot. A
// [discovery.Resolver] satisfies it directly (its Endpoints method tracks the
// backend via Watch), so a Pool reuses that machinery instead of re-implementing
// discovery.
type EndpointSource interface {
	Endpoints() []discovery.Endpoint
}

// Pool is the runtime that ties client-side load balancing together. On every
// [Pool.Pick] it takes the live endpoint snapshot from an [EndpointSource]
// (kept fresh by discovery Watch), drops endpoints the naming service marks
// disabled or unhealthy and that the suspension [Tracker] has suspended, and hands
// the survivors to a [Balancer] to choose one.
//
// This is where the two halves of health removal meet: discovery Watch handles
// instances coming and going, while the Tracker handles instances that are still
// registered but failing. Each filter falls back to its input rather than
// emptying the set — except Disabled instances, which are excluded outright (an
// operator-or-provider-disabled instance must never receive traffic).
type Pool struct {
	src     EndpointSource
	bal     Balancer
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
// *discovery.Resolver so the candidate set follows the naming service in real
// time; bal is any strategy from this package.
func NewPool(src EndpointSource, bal Balancer, opts ...PoolOption) *Pool {
	p := &Pool{src: src, bal: bal}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Pick selects one live, healthy, non-suspended endpoint via the configured
// balancer. The caller must invoke [Pool.Complete] with the picked endpoint
// when the request finishes: it advances least-conn accounting and feeds the
// suspension tracker, so skipping it defeats health suspension.
//
// It returns [ErrNoAvailable] when the source has no endpoints, or every
// endpoint is disabled.
func (p *Pool) Pick(info PickInfo) (discovery.Endpoint, error) {
	eps := p.src.Endpoints()
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
	}

	// Discovery-driven static admission ([discovery.Allows]: Endpoint
	// contract filter), distinct from the dynamic suspension filter below
	// ([Tracker.Allows]: circuit-breaker-style suspension).
	candidates := discovery.Allows(eps)
	if len(candidates) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
	}

	// Suspension filtering: drop instances the tracker has suspended for repeated
	// failures. Allows falls back to its input if everything is suspended.
	candidates = p.tracker.Allows(candidates)

	// Soft drain: drop instances whose weight was set to zero at the naming
	// service (the runtime traffic-drain signal). Falls back to its input when
	// every endpoint is zero-weighted — an unnormalized snapshot (registrants
	// predating the weight contract store 0 for "default") must not blackhole
	// the pool, it just degrades to an even split.
	if drained := excludeDrained(candidates); len(drained) > 0 {
		candidates = drained
	}

	return p.bal.Pick(candidates, info)
}

// Complete settles the request that Pick issued to ep: it advances the
// balancer's own accounting and records the outcome with the suspension tracker
// (when attached), so every strategy — not just least-conn — feeds suspension.
// Invoke it exactly once per picked endpoint, after the request ends.
func (p *Pool) Complete(ep discovery.Endpoint, err error) {
	p.bal.Complete(ep, err)
	if p.tracker != nil {
		p.tracker.Record(ep.Addr, err == nil)
	}
}

// excludeDrained drops endpoints with an explicit zero weight (the drain
// signal). Negative weights are kept — misconfiguration should not silently
// remove an instance; the weighted balancer still treats them as default.
// It returns nil when every endpoint is drained, so the caller can fall back.
func excludeDrained(eps []discovery.Endpoint) []discovery.Endpoint {
	var kept []discovery.Endpoint
	for _, ep := range eps {
		if ep.Weight != 0 {
			kept = append(kept, ep)
		}
	}
	return kept
}
