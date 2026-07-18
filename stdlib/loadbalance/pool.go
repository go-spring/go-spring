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
	"go-spring.org/stdlib/discovery"
)

// EndpointSource supplies the current live endpoint snapshot. A
// [discovery.LiveDialer] satisfies it directly (its Endpoints method already
// tracks the backend via Watch), so a Pool reuses that machinery instead of
// re-implementing discovery.
type EndpointSource interface {
	Endpoints() []discovery.Endpoint
}

// Pool is the runtime that ties client-side load balancing together. On every
// [Pool.Pick] it takes the live endpoint snapshot from an [EndpointSource]
// (kept fresh by discovery Watch), drops endpoints that discovery marks
// unhealthy or that the ejection [Tracker] has evicted, and hands the survivors
// to a [Balancer] to choose one.
//
// This is where the two halves of health eviction (task 2.2) meet: discovery
// Watch handles instances coming and going, while the Tracker handles instances
// that are still registered but failing. Neither is allowed to black-hole all
// traffic — each filter falls back to its input when it would otherwise empty
// the set.
type Pool struct {
	src     EndpointSource
	bal     Balancer
	tracker *Tracker
}

// PoolOption configures a [Pool].
type PoolOption func(*Pool)

// WithTracker attaches an outlier-ejection [Tracker] to the pool. Without it,
// eviction is disabled and only discovery's own Healthy flag filters endpoints.
func WithTracker(t *Tracker) PoolOption {
	return func(p *Pool) { p.tracker = t }
}

// NewPool builds a [Pool] over src using balancer bal. src is typically a
// *discovery.LiveDialer so the candidate set follows the naming service in real
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
// It returns [ErrNoAvailable] only when the source has no endpoints at all.
func (p *Pool) Pick(info PickInfo) (Result, error) {
	eps := p.src.Endpoints()
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
	}

	// Discovery-driven filtering: prefer instances the naming service reports
	// healthy. If the backend does not track health (none marked healthy) fall
	// back to all, matching discovery.LiveDialer.Pick so discovery still works.
	candidates := healthy(eps)

	// Ejection filtering: drop instances the tracker has evicted for repeated
	// failures. Eligible falls back to its input if everything is evicted.
	candidates = p.tracker.Eligible(candidates)

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

// healthy returns the endpoints marked Healthy, or eps unchanged when none are
// (so backends that do not report health are not filtered to nothing).
func healthy(eps []discovery.Endpoint) []discovery.Endpoint {
	out := eps[:0:0]
	for _, ep := range eps {
		if ep.Healthy {
			out = append(out, ep)
		}
	}
	if len(out) == 0 {
		return eps
	}
	return out
}
