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

// token_bucket.go is the token-bucket counter: the default rate limiter, a
// port of Guava's SmoothRateLimiter (both SmoothBursty and SmoothWarmingUp).
// Permits are granted at the FRONT of the pipeline while each charge's cost is
// appended to the BACK — a request pays for its predecessor, never for itself
// — and warm-up is Guava's stored-permit model: the bucket cools while idle
// and permit prices fall along a trapezoid as traffic drains it.

package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"

	"go-spring.org/stdlib/timeutil"
)

// tokenBucket is a minimal, dependency-free rate limiter. Fresh permits are
// minted at rate per second; up to maxPermits stored permits may sit in the
// bucket as a free stockpile. With a warm-up period set, the bucket starts
// cold (full) and stored permits above thresholdPermits cost extra wait on a
// trapezoid from coldInterval down to stableInterval — the Guava/Sentinel
// cold-start model, refilling at a pace that takes warmUp to rebuild the whole
// stockpile.
//
// reserveAt is the pipeline's tail, not the current time: it is when the queue
// of reserved charges runs dry, and it doubles as the regrowth reference — the
// stockpile only rebuilds while reserveAt lies in the past.
type tokenBucket struct {
	mu               sync.Mutex
	rate             float64   // fresh permits per second; the sustained pace
	stableInterval   float64   // seconds per fresh permit at the steady pace
	maxPermits       float64   // stockpile capacity; derived from warmUp when set
	thresholdPermits float64   // warm/cold boundary in stored permits (== maxPermits when warm-up is off)
	coldInterval     float64   // seconds per permit at the coldest point (== stableInterval when off)
	slope            float64   // seconds per permit of altitude above the threshold (0 when off)
	cooldownInterval float64   // seconds per stored permit while the stockpile rebuilds
	storedPermits    float64   // current stockpile, in [0, maxPermits]; never goes negative
	reserveAt        time.Time // when the pipeline runs dry; the wait queue lives here; zero until first use

	// policy is driven by [tokenBucket.Allow]: the first call materializes the
	// bucket from the rule, later ones reconfigure it in place. [Provider]
	// implementations return one unconfigured; that is the only way a bucket is
	// created.
	policy Policy
}

// Allow charges n units under policy, materializing the bucket on the first
// call and reconfiguring the existing state whenever the rule changes.
func (b *tokenBucket) Allow(ctx context.Context, policy Policy, n int) (bool, error) {
	if policy.RateLimit <= 0 || n <= 0 {
		return true, nil
	}

	// One clock read per call: configuration, the reservation and the wait all
	// measure from this instant.
	b.mu.Lock()
	now := time.Now()
	switch {
	case b.reserveAt.IsZero():
		b.configure(policy, now)
	case b.policy != policy:
		b.reconfigure(policy, now)
	}
	b.policy = policy
	b.mu.Unlock()

	// Charge outside mu: a bucket may queue for up to policy.MaxWait, and
	// holding the state lock while sleeping would serialize every caller.
	return b.charge(ctx, float64(n), policy.MaxWait, now), nil
}

// charge takes n permits, waiting up to maxWait for its turn. Whether the
// caller can afford the wait is not the bucket's concern: cancellation is left
// to Sleep, which refunds a reservation that never matures. now is the call's single clock read: the reservation
// and the wait both measure from it. Following Guava, a charge is granted at
// the current head of the pipeline — it never waits for its own cost — and the
// wait it owes is appended to the tail for whoever comes next. A false return
// means the head was further away than the deadline allowed, and nothing
// changed.
//
// The wait is reserved, not polled: the queue is reserveAt, so concurrent
// waiters line up on the time axis instead of waking periodically to race for
// the same refill. A charge whose sleep is cancelled refunds both the permits
// it consumed and the wait it queued.
func (b *tokenBucket) charge(ctx context.Context, n float64, maxWait time.Duration, now time.Time) bool {
	b.mu.Lock()
	b.resync(now)
	granted := b.reserveAt // this request rides the head of the queue
	if granted.After(now.Add(maxWait)) {
		b.mu.Unlock()
		return false
	}
	spend := math.Min(b.storedPermits, n)
	cost := b.storedCost(spend) + (n-spend)*b.stableInterval
	if b.reserveAt.Before(now) {
		b.reserveAt = now
	}
	b.reserveAt = b.reserveAt.Add(seconds(cost)) // the tail moves by what this charge owes
	b.storedPermits -= spend
	rate, maxPermits := b.rate, b.maxPermits // the geometry the reservation was made under
	b.mu.Unlock()

	if wait := granted.Sub(now); wait > 0 {
		if !timeutil.Sleep(ctx, wait) {
			// Cancelled or timed out before the turn matured: refund it.
			b.refund(now, cost, spend, rate, maxPermits)
			return false
		}
	}
	return true
}

// seconds converts a wait in seconds to a [time.Duration].
func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// storedCost is the wait owed for consuming take stored permits: free when
// warm-up is off; along the trapezoid (averaging the prices at both ends)
// above thresholdPermits and at the stable price below it. As in Guava, the
// trapezoid applies to any positive altitude above the threshold, including a
// fraction of a permit. Callers hold mu and pass take against the CURRENT
// stockpile.
func (b *tokenBucket) storedCost(take float64) float64 {
	if b.slope == 0 { // no warm-up: stored permits are free
		return 0
	}
	var seconds float64
	if above := b.storedPermits - b.thresholdPermits; above > 0.0 {
		cold := math.Min(above, take)
		length := b.permitsToTime(above) + b.permitsToTime(above-cold)
		seconds = cold * length / 2
		take -= cold
	}
	return seconds + take*b.stableInterval
}

// permitsToTime is the price in seconds of the stored permit altitude seats
// above the threshold. Callers hold mu.
func (b *tokenBucket) permitsToTime(above float64) float64 {
	return above*b.slope + b.stableInterval
}

// configure is the unconfigured → configured transition: the bucket takes the
// policy's pace and shape, and starts empty — or, with warm-up, full and cold.
// Callers hold mu.
func (b *tokenBucket) configure(policy Policy, now time.Time) {
	b.rate = policy.RateLimit
	b.stableInterval = 1 / b.rate
	b.reshape(policy)
	b.storedPermits = 0
	if policy.WarmUp > 0 {
		b.storedPermits = b.maxPermits // a warming bucket starts cold
	}
	b.reserveAt = now
}

// reconfigure applies a changed rule to the existing state: the stockpile
// advances under its old geometry, then keeps its level (scaled to the new
// capacity) while the pace and shape change. Pending queue debt is rescaled by
// the pace ratio, so charges that queue AFTER the change line up under the new
// pace; charges already sleeping keep their original turn and are not woken.
// Callers hold mu.
func (b *tokenBucket) reconfigure(policy Policy, now time.Time) {
	head := now
	if b.reserveAt.After(head) {
		head = b.reserveAt // resync up to the queue head
	}
	b.resync(head)
	oldMax, oldRate := b.maxPermits, b.rate
	b.rate = policy.RateLimit
	b.stableInterval = 1 / b.rate
	b.reshape(policy)
	b.storedPermits = math.Min(b.storedPermits*b.maxPermits/oldMax, b.maxPermits)
	if pending := b.reserveAt.Sub(now); pending > 0 {
		b.reserveAt = now.Add(seconds(pending.Seconds() * oldRate / b.rate))
	}
}

// resync advances the stockpile to now: while the pipeline has been idle,
// stored permits regrow at one per cooldownInterval seconds, up to maxPermits.
// Callers hold mu.
func (b *tokenBucket) resync(now time.Time) {
	if delta := now.Sub(b.reserveAt); delta > 0 {
		b.storedPermits = math.Min(b.maxPermits, b.storedPermits+delta.Seconds()/b.cooldownInterval)
		b.reserveAt = now
	}
}

// reshape computes the stockpile geometry from the policy: burst-sized and
// free without warm-up; with warm-up, maxPermits and thresholdPermits are
// derived from the ramp and coldFactor (Guava's construction) and burst is
// ignored. Callers hold mu.
func (b *tokenBucket) reshape(policy Policy) {
	b.slope = 0
	b.coldInterval = b.stableInterval
	if policy.WarmUp <= 0 {
		b.maxPermits = float64(burstFor(policy))
		b.thresholdPermits = b.maxPermits
		b.cooldownInterval = b.stableInterval
		return
	}
	coldFactor := policy.ColdFactor
	if coldFactor <= 1 {
		coldFactor = 3
	}
	warmUp := policy.WarmUp.Seconds()
	b.coldInterval = b.stableInterval * float64(coldFactor)
	b.thresholdPermits = 0.5 * warmUp / b.stableInterval
	b.maxPermits = b.thresholdPermits + 2*warmUp/(b.stableInterval+b.coldInterval)
	b.slope = (b.coldInterval - b.stableInterval) / (b.maxPermits - b.thresholdPermits)
	b.cooldownInterval = warmUp / b.maxPermits
}

// refund returns a reservation that never matured: the stockpile gets its
// permits back and the queue loses the wait this charge appended. Later
// reservations keep their (now closer) turns. now is the call's single clock
// read, so the refund is applied as of the request's own instant. rate and
// maxPermits are the geometry the reservation was made under: if the rule
// changed while the caller slept, the debt is rescaled to the new geometry
// first — mirroring what reconfigure itself did to the state.
func (b *tokenBucket) refund(now time.Time, cost, spend, rate, maxPermits float64) {
	cost *= rate / b.rate
	spend *= maxPermits / b.maxPermits

	b.mu.Lock()
	defer b.mu.Unlock()
	b.resync(now)
	b.storedPermits = math.Min(b.storedPermits+spend, b.maxPermits)
	if b.reserveAt.After(now) {
		if rolled := b.reserveAt.Add(-seconds(cost)); rolled.After(now) {
			b.reserveAt = rolled
		} else {
			b.reserveAt = now
		}
	}
}
