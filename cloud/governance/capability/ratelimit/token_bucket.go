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

// token_bucket.go is the token-bucket counter: the default rate limiter,
// refilling continuously and (optionally) ramping in from a cold start.

package ratelimit

import (
	"context"
	"sync"
	"time"

	"go-spring.org/stdlib/timeutil"
)

// tokenBucket is a minimal, dependency-free rate limiter. Tokens refill
// continuously at rate per second up to burst. With a warm-up period set, the
// bucket starts cold and its CAPACITY ramps from burst/coldFactor up to burst
// over the period — the cold-start approximation of sentinel's WarmUp flow
// rule: refilling still runs at the full rate, so throughput is bounded by a
// growing ceiling rather than a growing refill.
type tokenBucket struct {
	mu         sync.Mutex
	rate       float64   // tokens added per second
	burst      float64   // capacity, and the initial token count
	tokens     float64   // current balance; negative while reservations queue
	lastRefill time.Time // the instant tokens was last advanced

	// warm-up state; warmUp <= 0 disables the ramp and cap stays at burst.
	warmUp       time.Duration // how long the ramp-in takes
	coldCapacity float64       // capacity at t=0 (burst/coldFactor)
	rampStarted  time.Time     // when the ramp began

	// policy and configured are driven by [tokenBucket.Allow]: the first call
	// materializes the bucket from the rule, later ones reconfigure it in place.
	// [Provider] implementations return one unconfigured; that is the only way a
	// bucket is created.
	policy     Policy
	configured bool
}

// Allow charges n units under policy, materializing the bucket on the first
// call and reconfiguring it in place whenever the rule changes.
func (b *tokenBucket) Allow(ctx context.Context, policy Policy, n int) (bool, error) {
	if policy.RateLimit <= 0 || n <= 0 {
		return true, nil
	}

	b.mu.Lock()
	switch {
	case !b.configured:
		b.configure(policy, time.Now())
	case b.policy != policy:
		b.reconfigure(policy, time.Now())
	}
	b.policy = policy
	b.mu.Unlock()

	// Charge outside mu: a bucket may queue for up to policy.MaxWait, and
	// holding the state lock while sleeping would serialize every caller.
	return b.charge(ctx, float64(n), policy.MaxWait), nil
}

// charge consumes n tokens, waiting up to maxWait (and no longer than ctx
// allows) when the bucket is momentarily empty. maxWait 0 is the immediate
// allow/reject decision.
//
// The wait is reserved, not polled: the tokens are deducted up front (going
// negative as a reservation), so concurrent waiters queue behind each other instead of
// waking periodically to race for the same refill. A false return means no
// token was obtained (deadline exceeded or ctx done), and the reservation is
// refunded. Requests for more than the bucket can ever hold (n > burst) are
// refused immediately.
func (b *tokenBucket) charge(ctx context.Context, n float64, maxWait time.Duration) bool {
	if n <= 0 {
		return true
	}
	b.mu.Lock()
	now := time.Now()
	b.refill(now)
	if b.tokens >= n {
		b.tokens -= n
		b.mu.Unlock()
		return true
	}
	if maxWait <= 0 || b.rate <= 0 || n > b.burst {
		b.mu.Unlock()
		return false
	}
	wait := b.waitTime(n, now)
	deadline := now.Add(maxWait)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if now.Add(wait).After(deadline) {
		b.mu.Unlock()
		return false
	}
	b.tokens -= n // reserve now: later waiters queue behind this one
	b.mu.Unlock()

	if wait <= 0 {
		return true
	}
	if !timeutil.Sleep(ctx, wait) {
		// Cancelled or timed out before the reservation matured: refund it.
		b.refund(n)
		return false
	}
	return true
}

// waitTime is how long n tokens take to become available, accounting for both
// the refill rate and the growing warm-up ceiling. Callers hold mu and have
// already refilled.
func (b *tokenBucket) waitTime(n float64, now time.Time) time.Duration {
	var seconds float64
	if b.rate > 0 && b.tokens < n {
		seconds = (n - b.tokens) / b.rate
	}
	if b.warmUp > 0 {
		if ceiling := b.capacityAt(now); ceiling < n {
			var readyAt time.Duration
			if n >= b.burst || b.burst <= b.coldCapacity {
				readyAt = b.warmUp
			} else {
				progress := (n - b.coldCapacity) / (b.burst - b.coldCapacity)
				readyAt = time.Duration(progress * float64(b.warmUp))
			}
			if remaining := readyAt - now.Sub(b.rampStarted); remaining.Seconds() > seconds {
				seconds = remaining.Seconds()
			}
		}
	}
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// refund returns a reservation that never matured, leaving the bucket exactly
// where a refill would have put it.
func (b *tokenBucket) refund(n float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.refill(now)
	b.tokens += n
	if ceiling := b.capacityAt(now); b.tokens > ceiling {
		b.tokens = ceiling
	}
}

// configure is the unconfigured → configured transition: the bucket takes
// the policy's rate and full initial capacity, and starts its warm-up ramp when
// the policy asks for one. Callers hold mu.
func (b *tokenBucket) configure(policy Policy, now time.Time) {
	b.rate = policy.RateLimit
	b.burst = float64(burstFor(policy))
	b.lastRefill = now
	b.tokens = b.burst
	b.startWarmUp(policy.WarmUp, policy.ColdFactor, now)
	b.configured = true
}

// startWarmUp enters the cold-start ramp: capacity begins at
// burst/coldFactor and grows to burst over warmUp. A non-positive warmUp leaves
// the bucket at full capacity. Callers hold mu.
func (b *tokenBucket) startWarmUp(warmUp time.Duration, coldFactor int, now time.Time) {
	b.warmUp = 0
	b.coldCapacity = 0
	if warmUp <= 0 {
		return
	}
	if coldFactor <= 1 {
		coldFactor = 3
	}
	b.warmUp = warmUp
	b.coldCapacity = b.burst / float64(coldFactor)
	b.tokens = b.coldCapacity
	b.rampStarted = now
}

// reconfigure applies a changed rule to the existing tokens: the bucket
// advances to now under its old parameters, then takes the new rate, burst and
// warm-up settings. Counts are preserved; only a burst decrease clamps them.
// Callers hold mu.
func (b *tokenBucket) reconfigure(policy Policy, now time.Time) {
	b.refill(now)
	b.rate = policy.RateLimit
	b.burst = float64(burstFor(policy))
	if policy.WarmUp > 0 {
		coldFactor := policy.ColdFactor
		if coldFactor <= 1 {
			coldFactor = 3
		}
		if b.warmUp <= 0 {
			b.rampStarted = now // a newly enabled warm-up starts now
		}
		b.warmUp = policy.WarmUp
		b.coldCapacity = b.burst / float64(coldFactor)
	} else {
		b.warmUp = 0
		b.coldCapacity = 0
	}
	if ceiling := b.capacityAt(now); b.tokens > ceiling {
		b.tokens = ceiling
	}
}

// capacityAt returns the bucket's capacity at now — full burst, or the ramped-in
// ceiling while a warm-up period is in progress. Callers hold mu.
func (b *tokenBucket) capacityAt(now time.Time) float64 {
	if b.warmUp <= 0 {
		return b.burst
	}
	elapsed := now.Sub(b.rampStarted)
	if elapsed >= b.warmUp {
		return b.burst
	}
	progress := float64(elapsed) / float64(b.warmUp)
	return b.coldCapacity + (b.burst-b.coldCapacity)*progress
}

// refill advances the bucket to now and caps it at the current ceiling.
// Callers hold mu. tokens may remain negative: charge reservations are taken
// against future refills.
func (b *tokenBucket) refill(now time.Time) {
	if b.rate > 0 {
		b.tokens += now.Sub(b.lastRefill).Seconds() * b.rate
	}
	if ceiling := b.capacityAt(now); b.tokens > ceiling {
		b.tokens = ceiling
	}
	b.lastRefill = now
}
