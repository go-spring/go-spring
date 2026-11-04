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

package ratelimit

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// TestTokenBucketBurstAndRefill pins the Guava pipeline: a charge is granted
// at the head (never waiting for itself) and owes its cost to the next
// caller; the stockpile regrows only while the pipeline is idle.
func TestTokenBucketBurstAndRefill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 100, Burst: 2}
		assert.That(t, allow(b, p, 1)).True()  // granted at the head; owes 10ms
		assert.That(t, allow(b, p, 1)).False() // the head is now 10ms away
		time.Sleep(20 * time.Millisecond)
		assert.That(t, allow(b, p, 1)).True() // debt expired, stockpile regrew
	})
}

// TestTokenBucketWarmUpStartsCold pins Guava's WarmUp model: the bucket
// starts full and cold, and the first cold permit costs the trapezoid average
// of its two ends — charged, as always, to the next request. With rate 2/s,
// warm-up 10s and coldFactor 3: threshold=10, max=20, slope=0.1s, so the
// first stored permit above the threshold costs (1.5+1.4)/2 = 1.45s.
func TestTokenBucketWarmUpStartsCold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 2, WarmUp: 10 * time.Second, ColdFactor: 3, MaxWait: 2 * time.Second}
		start := time.Now()
		assert.That(t, allow(b, p, 1)).True() // granted at the head, owes 1.45s
		assert.That(t, time.Since(start)).Equal(time.Duration(0))
		assert.That(t, allow(b, p, 1)).True() // waits out the predecessor's cost
		assert.That(t, time.Since(start)).Equal(1450 * time.Millisecond)
	})
}

// TestTokenBucketWarmUpChargesFractionalAltitude pins the trapezoid at its
// lower edge: with half a permit above the threshold, half of the charge is
// priced on the cold curve and half at the steady interval. Guava applies the
// trapezoid to any positive altitude, not just to whole permits.
func TestTokenBucketWarmUpChargesFractionalAltitude(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		// rate 2/s, warm-up 10s, coldFactor 3: threshold 10, max 20,
		// slope 0.1s, cooldown 0.5s, stable 0.5s.
		p := Policy{RateLimit: 2, WarmUp: 10 * time.Second, ColdFactor: 3, MaxWait: time.Minute}

		// Spend 15 stored permits, leaving 5; this charge owes 12.5s.
		assert.That(t, allow(b, p, 15)).True()

		// Let the queue drain, then refill 5 permits to the threshold (2.5s)
		// and half a permit more (0.25s): stored = threshold + 0.5.
		time.Sleep(15250 * time.Millisecond)

		// The next charge is granted at the head and owes 0.5125s: 0.5 permit
		// on the trapezoid (0.2625s) plus 0.5 at the steady price (0.25s).
		start := time.Now()
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, allow(b, p, 1)).True() // waits out the predecessor's cost
		elapsed := time.Since(start)
		assert.Number(t, elapsed.Milliseconds()).GreaterThan(int64(500)).LessThan(int64(520))
	})
}

func TestTokenBucketWaitsForToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 100, Burst: 1, MaxWait: time.Second} // one token every 10ms
		start := time.Now()
		assert.That(t, allow(b, p, 1)).True() // burst spent; the next token must be waited for
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, time.Since(start)).Equal(10 * time.Millisecond)
	})
}

// TestTokenBucketChargesTheNextRequest pins Guava's pay-for-previous rule: a
// charge is granted at the head of the queue no matter its size — only its
// successor feels the wait. Nothing is "too big to ever hold".
func TestTokenBucketChargesTheNextRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 1, Burst: 2}
		start := time.Now()
		assert.That(t, allow(b, p, 3)).True() // granted immediately, owes 3s
		assert.That(t, time.Since(start)).Equal(time.Duration(0))
		assert.That(t, allow(b, p, 1)).False() // the head is now 3s away
	})
}

// TestTokenBucketRefundsOnCancel pins that an aborted reservation does
// not leak tokens: once time passes, the refunded token is available.
func TestTokenBucketRefundsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 1, Burst: 1, MaxWait: time.Second}
		assert.That(t, allow(b, p, 1)).True()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		admitted, err := b.Allow(ctx, p, 1)
		assert.Error(t, err).Nil()
		assert.That(t, admitted).False()

		time.Sleep(time.Second)
		assert.That(t, allow(b, p, 1)).True() // the cancelled reservation was refunded
	})
}

// TestTokenBucketKeepsStateAcrossPolicyChange pins the model: the rule travels
// with every call, so changing it updates the counter instead of refilling it.
func TestTokenBucketKeepsStateAcrossPolicyChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}

		policy := Policy{RateLimit: 1, Burst: 1}
		allowed, err := b.Allow(context.Background(), policy, 1)
		assert.Error(t, err).Nil()
		assert.That(t, allowed).True()
		allowed, _ = b.Allow(context.Background(), policy, 1)
		assert.That(t, allowed).False() // the only token is spent

		// A burst change must not refill the bucket.
		policy.Burst = 10
		allowed, _ = b.Allow(context.Background(), policy, 1)
		assert.That(t, allowed).False()

		// The new rate does reach the existing state. The next call applies it
		// (still refused: the elapsed time so far was earned at the old rate),
		// and 100ms later the bucket has refilled at 100/s.
		policy.RateLimit = 100
		allowed, _ = b.Allow(context.Background(), policy, 1)
		assert.That(t, allowed).False()
		time.Sleep(100 * time.Millisecond)
		allowed, _ = b.Allow(context.Background(), policy, 1)
		assert.That(t, allowed).True()
	})
}

// TestTokenBucketUnlimitedPolicy pins that a zero rate disables the stage: every
// charge is admitted.
func TestTokenBucketUnlimitedPolicy(t *testing.T) {
	b := &tokenBucket{}
	allowed, err := b.Allow(context.Background(), Policy{}, 1)
	assert.Error(t, err).Nil()
	assert.That(t, allowed).True()
	allowed, _ = b.Allow(context.Background(), Policy{}, 100)
	assert.That(t, allowed).True()
}

// TestTokenBucketNonPositiveCharge pins that a non-positive batch is a no-op:
// it is admitted, and neither a permit nor queue debt is consumed.
func TestTokenBucketNonPositiveCharge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 1, Burst: 1}
		assert.That(t, allow(b, p, 0)).True()
		assert.That(t, allow(b, p, -1)).True()
		assert.That(t, allow(b, p, 1)).True() // the only permit is still there
		assert.That(t, allow(b, p, 1)).False()
	})
}

// TestTokenBucketMaxWaitRejects pins the refusal side of queueing: a head
// beyond the deadline is refused without moving the queue, so a later charge
// with a longer budget still finds the same head.
func TestTokenBucketMaxWaitRejects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 1, Burst: 1}
		assert.That(t, allow(b, p, 1)).True() // owes the next charge 1s

		short := p
		short.MaxWait = 500 * time.Millisecond
		start := time.Now()
		assert.That(t, allow(b, short, 1)).False()
		assert.That(t, time.Since(start)).Equal(time.Duration(0)) // refused, not queued

		p.MaxWait = 2 * time.Second
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, time.Since(start)).Equal(time.Second) // the head never moved
	})
}

// TestTokenBucketRateDropStretchesQueue pins the pace-ratio rescale of pending
// debt: halving the rate doubles the wait that charges queueing afterwards
// inherit.
func TestTokenBucketRateDropStretchesQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 10, Burst: 10, MaxWait: 2 * time.Second}
		start := time.Now()
		assert.That(t, allow(b, p, 1)).True() // granted now, tail 100ms
		assert.That(t, allow(b, p, 1)).True() // waits to 100ms, tail 200ms

		slow := p
		slow.RateLimit = 5 // the 100ms of pending debt doubles to 200ms
		assert.That(t, allow(b, slow, 1)).True()
		assert.That(t, time.Since(start)).Equal(300 * time.Millisecond)
	})
}
