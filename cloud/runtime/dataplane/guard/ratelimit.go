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
// [Policy] onto the direction-neutral [go-spring.org/cloud/runtime/dataplane/ratelimit.Spec] and
// holds the instance's local budget. The algorithms, the [RateSpec] vocabulary
// and the [ClusterLimiter] seam live in the shared leaf package, because
// outbound resilience charges the same ones.

package guard

import (
	"context"

	"go-spring.org/cloud/runtime/dataplane/ratelimit"
)

// RateSpec is the rate-limit stage's own vocabulary. A [ClusterLimiter] store
// receives it so a shared backend counts the same request the local budget
// would have.
type RateSpec = ratelimit.Spec

// ClusterLimiter is the rate-limit stage's cross-replica seam. A store failure
// is treated as "allow": an unreachable counter backend must not become an
// outage.
type ClusterLimiter = ratelimit.ClusterLimiter

// RateLimiter is the rate-limit stage as an independent object. It is the same
// implementation [Guard] uses; a caller assembling a custom middleware can take
// one directly.
//
// scope is the identity a [ClusterLimiter] counts under (the server instance's
// Service); limiter may be nil for an instance-local budget.
type RateLimiter struct {
	budget  ratelimit.Budget
	limiter ClusterLimiter
	scope   string
	spec    RateSpec
	p       Policy
}

// NewRateLimiter builds the rate-limit stage for p. A zero p.RateLimit leaves
// the stage unlimited.
func NewRateLimiter(p Policy, scope string, limiter ClusterLimiter) *RateLimiter {
	spec := rateSpecFor(p)
	return &RateLimiter{
		budget:  ratelimit.NewBudget(spec),
		limiter: limiter,
		scope:   scope,
		spec:    spec,
		p:       p,
	}
}

// Allow charges one request against the budget. A nil limiter or a zero
// p.RateLimit admits everything.
func (l *RateLimiter) Allow(ctx context.Context) bool {
	if l == nil || l.p.RateLimit <= 0 {
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

// rateSpecFor is the policy's view of the rate-limit stage.
func rateSpecFor(p Policy) RateSpec {
	return RateSpec{
		RateLimit:  p.RateLimit,
		Burst:      p.Burst,
		Algorithm:  ratelimit.Algorithm(p.Algorithm),
		Window:     p.Window,
		MaxWait:    p.RateLimitMaxWait,
		WarmUp:     p.RateLimitWarmUpDuration,
		ColdFactor: p.WarmUpColdFactor,
	}
}
