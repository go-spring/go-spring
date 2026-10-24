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

// ratelimit.go is the inbound view of the shared rate-limit stage: it maps a
// [Policy] onto the direction-neutral [go-spring.org/cloud/runtime/dataplane/kernel/ratelimit.Spec] and
// holds the instance's local budget. The algorithms, the [ratelimit.Spec] vocabulary
// and the [ratelimit.ClusterLimiter] seam live in the shared leaf package, because
// outbound resilience charges the same ones.

package admission

import (
	"context"

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
)

// RateLimiter is the rate-limit stage as an independent object. It is the same
// implementation [Guard] uses; a caller assembling a custom middleware can take
// one directly.
//
// scope is the identity a [ratelimit.ClusterLimiter] counts under (the server
// instance's Service); limiter may be nil for an instance-local budget.
type RateLimiter struct {
	budget  ratelimit.Budget
	limiter ratelimit.ClusterLimiter
	scope   string
	spec    ratelimit.Spec
	p       Policy
}

// NewRateLimiter builds the rate-limit stage for p. A zero p.RateLimit leaves
// the stage unlimited.
func NewRateLimiter(p Policy, scope string, limiter ratelimit.ClusterLimiter) *RateLimiter {
	spec := rateSpecFor(p)
	return &RateLimiter{
		budget:  ratelimit.NewBudget(spec),
		limiter: limiter,
		scope:   scope,
		spec:    spec,
		p:       p,
	}
}

// Allow charges one request against the budget. A zero p.RateLimit admits
// everything.
func (l *RateLimiter) Allow(ctx context.Context) bool {
	if l.p.RateLimit <= 0 {
		return true
	}
	admitted, err := ratelimit.Charge(ctx, l.limiter, l.scope, l.spec, l.budget, 1)
	// A store failure is not an inbound decision: allow the request rather
	// than turning an unreachable counter backend into an outage.
	if err != nil {
		return true
	}
	return admitted
}

// Usage reports how much of the LOCAL budget is spent. The zero Usage means
// there is nothing to report: the stage is unlimited, or a ClusterLimiter is
// configured and the store, not the local budget, is charged.
func (l *RateLimiter) Usage() ratelimit.Usage {
	if l.limiter != nil || l.p.RateLimit <= 0 {
		return ratelimit.Usage{}
	}
	return l.budget.Usage()
}

// rateSpecFor is the policy's view of the rate-limit stage.
func rateSpecFor(p Policy) ratelimit.Spec {
	return ratelimit.Spec{
		RateLimit:  p.RateLimit,
		Burst:      p.Burst,
		Algorithm:  ratelimit.Algorithm(p.Algorithm),
		Window:     p.Window,
		MaxWait:    p.RateLimitMaxWait,
		WarmUp:     p.RateLimitWarmUpDuration,
		ColdFactor: p.WarmUpColdFactor,
	}
}
