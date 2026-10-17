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

package guard

import (
	"context"
	"errors"
	"time"

	"go-spring.org/cloud/ratelimit"
)

// RateSpec is the rate-limit stage's own vocabulary. A [ClusterLimiter] store
// receives it so a shared backend counts the same request the local budget
// would have.
type RateSpec struct {
	// RateLimit is the sustained budget in units per second. 0 disables the
	// stage.
	RateLimit float64

	// Burst is how many units may exceed RateLimit momentarily.
	Burst int

	// Algorithm is the counting scheme ("sliding-window" or empty for the token
	// bucket).
	Algorithm string

	// Window is the sliding-window interval; 0 means one second.
	Window time.Duration

	// MaxWait is how long an over-limit request may wait before being refused.
	MaxWait time.Duration

	// WarmUp is the cold-start ramp-in period.
	WarmUp time.Duration

	// ColdFactor shapes the ramp-in.
	ColdFactor int
}

// ClusterLimiter is the rate-limit stage's cross-replica seam: a store (Redis,
// typically) that holds one budget across every replica. When a Guard is built
// with one, the store is charged instead of the instance-local budget; the
// Guard's own state still holds the bulkhead and breaker.
//
// Allow returns [ErrRateLimited] when the budget is exhausted, and any other
// error when the store itself could not be read or updated. The engine treats a
// store failure as "allow": an unreachable counter backend must not become an
// outage.
type ClusterLimiter interface {
	Allow(ctx context.Context, scope string, spec RateSpec, n int) error
}

// RateLimiter is the rate-limit stage as an independent object. It is the same
// implementation [Guard] uses; a caller assembling a custom middleware can take
// one directly.
//
// scope is the identity a [ClusterLimiter] counts under (the server instance's
// Service); limiter may be nil for an instance-local budget.
type RateLimiter struct {
	st rateState
	p  Policy
}

// NewRateLimiter builds the rate-limit stage for p. A zero p.RateLimit leaves
// the stage unlimited.
func NewRateLimiter(p Policy, scope string, limiter ClusterLimiter) *RateLimiter {
	return &RateLimiter{st: newRateState(p, limiter, scope), p: p}
}

// Allow charges one request against the budget. A nil limiter or a zero
// p.RateLimit admits everything.
func (l *RateLimiter) Allow(ctx context.Context) bool {
	if l == nil {
		return true
	}
	return l.st.allow(ctx, l.p)
}

// rateState is the budget ONE server instance holds. Its zero value means "no
// rate limit configured", which the stage treats as unlimited.
type rateState struct {
	bucket *ratelimit.TokenBucket
	window *ratelimit.SlidingWindow

	// limiter, when non-nil, is charged instead of the local budget; scope is
	// the instance's identity so every replica of the same label shares it.
	limiter ClusterLimiter
	scope   string
	spec    RateSpec
}

// newRateState builds the budget p asks for. limiter is optional: nil keeps the
// count in this instance.
func newRateState(p Policy, limiter ClusterLimiter, scope string) rateState {
	r := rateState{limiter: limiter, scope: scope, spec: rateSpecFor(p)}
	if p.RateLimit <= 0 {
		return r
	}
	if p.Algorithm == "sliding-window" {
		win := p.Window
		if win <= 0 {
			win = time.Second
		}
		limit := p.RateLimit * win.Seconds()
		if limit < 1 {
			limit = 1
		}
		r.window = ratelimit.NewSlidingWindow(limit, win)
		return r
	}
	burst := p.Burst
	if burst <= 0 {
		burst = int(p.RateLimit)
		if burst < 1 {
			burst = 1
		}
	}
	r.bucket = ratelimit.NewTokenBucket(p.RateLimit, burst).
		WithWarmUp(p.RateLimitWarmUpDuration, p.WarmUpColdFactor)
	return r
}

// rateSpecFor is the policy's view of the rate-limit stage.
func rateSpecFor(p Policy) RateSpec {
	return RateSpec{
		RateLimit:  p.RateLimit,
		Burst:      p.Burst,
		Algorithm:  p.Algorithm,
		Window:     p.Window,
		MaxWait:    p.RateLimitMaxWait,
		WarmUp:     p.RateLimitWarmUpDuration,
		ColdFactor: p.WarmUpColdFactor,
	}
}

// allow charges one request against the budget. When the budget is exhausted
// and p asks for a bounded wait, the wait is bounded by both MaxWait and the
// request's context.
func (r rateState) allow(ctx context.Context, p Policy) bool {
	if p.RateLimit <= 0 {
		return true
	}
	if r.limiter != nil {
		// A store failure is not an inbound decision: allow the request rather
		// than turning an unreachable counter backend into an outage.
		return !errors.Is(r.limiter.Allow(ctx, r.scope, r.spec, 1), ErrRateLimited)
	}
	if r.window != nil {
		return r.window.AllowN(1)
	}
	return r.bucket.WaitN(ctx, 1, p.RateLimitMaxWait)
}
