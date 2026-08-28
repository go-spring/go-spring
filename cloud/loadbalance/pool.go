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
// disabled or unhealthy and that the ejection [Tracker] has evicted, and hands
// the survivors to a [Balancer] to choose one.
//
// This is where the two halves of health eviction meet: discovery Watch handles
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

// WithTracker attaches an outlier-ejection [Tracker] to the pool. Without it,
// eviction is disabled and only the discovery endpoint flags filter endpoints.
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

// Pick selects one live, healthy, non-evicted endpoint via the configured
// balancer. The returned [Result.Done] must be invoked when the request
// finishes: it advances least-conn accounting and feeds the ejection tracker,
// so skipping it defeats health eviction.
//
// It returns [ErrNoAvailable] when the source has no endpoints, or every
// endpoint is disabled.
func (p *Pool) Pick(info PickInfo) (Result, error) {
	eps := p.src.Endpoints()
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
	}

	// Discovery-driven static eligibility ([discovery.Eligible]: Endpoint
	// contract filter), distinct from the dynamic ejection filter below
	// ([Tracker.Eligible]: circuit-breaker-style eviction).
	candidates := discovery.Eligible(eps)
	if len(candidates) == 0 {
		return Result{}, ErrNoAvailable
	}

	// Ejection filtering: drop instances the tracker has evicted for repeated
	// failures. Eligible falls back to its input if everything is evicted.
	candidates = p.tracker.Eligible(candidates)

	// Soft drain: drop instances whose weight was set to zero at the naming
	// service (the runtime traffic-drain signal). Falls back to its input when
	// every endpoint is zero-weighted — an unnormalized snapshot (registrants
	// predating the weight contract store 0 for "default") must not blackhole
	// the pool, it just degrades to an even split.
	if drained := excludeDrained(candidates); len(drained) > 0 {
		candidates = drained
	}

	res, err := p.bal.Pick(candidates, info)
	if err != nil {
		return Result{}, err
	}

	// Wrap the balancer's Done so the tracker sees every outcome even for
	// strategies (round-robin, hash, weighted) that supply no Done of their own.
	addr := res.Endpoint.Addr
	inner := res.Done
	res.Done = func(di DoneInfo) {
		if inner != nil {
			inner(di)
		}
		if p.tracker != nil {
			p.tracker.Record(addr, di.Err == nil)
		}
	}
	return res, nil
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
