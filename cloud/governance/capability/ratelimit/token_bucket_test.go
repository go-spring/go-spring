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

func TestTokenBucketBurstAndRefill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 100, Burst: 2}
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, allow(b, p, 1)).False() // burst spent, refill not elapsed yet
		time.Sleep(20 * time.Millisecond)
		assert.That(t, allow(b, p, 1)).True() // refilled
	})
}

// TestTokenBucketWarmUpStartsCold pins the cold-start behavior: a warm-up
// bucket begins at burst/coldFactor tokens, not full burst.
func TestTokenBucketWarmUpStartsCold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 0.0001, Burst: 9, WarmUp: time.Hour, ColdFactor: 3}
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, allow(b, p, 1)).True()
		assert.That(t, allow(b, p, 1)).True() // 3 = 9/3 cold tokens spent
		assert.That(t, allow(b, p, 1)).False()
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

// TestTokenBucketRejectsOversizedRequest pins fail-fast behavior: no
// amount of waiting can ever accumulate more than burst tokens.
func TestTokenBucketRejectsOversizedRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &tokenBucket{}
		p := Policy{RateLimit: 1, Burst: 2, MaxWait: time.Hour}
		start := time.Now()
		assert.That(t, allow(b, p, 3)).False()
		assert.That(t, time.Since(start)).Equal(time.Duration(0)) // no wait attempted
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
