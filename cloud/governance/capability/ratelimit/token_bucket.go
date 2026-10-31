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

// TokenBucket is a minimal, dependency-free rate limiter. Tokens refill
// continuously at rate per second up to burst. With a warm-up period set, the
// bucket starts cold and its CAPACITY ramps from burst/coldFactor up to burst
// over the period — the cold-start approximation of sentinel's WarmUp flow
// rule: refilling still runs at the full rate, so throughput is bounded by a
// growing ceiling rather than a growing refill.
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time

	// warm-up state; warmup <= 0 disables the ramp and cap stays at burst.
	warmup  time.Duration
	cold    float64 // capacity at t=0 (burst/coldFactor)
	started time.Time
}

// NewTokenBucket returns a full bucket that refills at rate per second up to
// burst.
func NewTokenBucket(rate float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:   rate,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// WithWarmUp turns the bucket into a cold-start one: capacity ramps from
// burst/coldFactor to burst over warmup, starting now, and the initial tokens
// are the cold capacity rather than full burst.
func (b *TokenBucket) WithWarmUp(warmup time.Duration, coldFactor int) *TokenBucket {
	if warmup <= 0 {
		return b
	}
	if coldFactor <= 1 {
		coldFactor = 3
	}
	b.warmup = warmup
	b.cold = b.burst / float64(coldFactor)
	b.tokens = b.cold
	b.started = time.Now()
	return b
}

// cap returns the bucket's capacity at now — full burst, or the ramped-in
// ceiling while a warm-up period is in progress. Callers hold mu.
func (b *TokenBucket) cap(now time.Time) float64 {
	if b.warmup <= 0 {
		return b.burst
	}
	elapsed := now.Sub(b.started)
	if elapsed >= b.warmup {
		return b.burst
	}
	progress := float64(elapsed) / float64(b.warmup)
	return b.cold + (b.burst-b.cold)*progress
}

// Usage reports how much of the bucket's current capacity is spent, without
// changing the bucket: the same refill and warm-up arithmetic as [AllowN],
// projected at now and discarded.
func (b *TokenBucket) Usage() Usage {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	ceiling := b.cap(now)
	tokens := b.tokens + now.Sub(b.last).Seconds()*b.rate
	if tokens > ceiling {
		tokens = ceiling
	}
	if tokens < 0 {
		tokens = 0
	}
	return Usage{Used: ceiling - tokens, Limit: ceiling}
}

// AllowN consumes n tokens if at least n are available.
func (b *TokenBucket) AllowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if ceiling := b.cap(now); b.tokens > ceiling {
		b.tokens = ceiling
	}
	b.last = now
	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}

// WaitN consumes n tokens, waiting up to maxWait (and no longer than ctx
// allows) when the bucket is momentarily empty. It is the queueing form of
// AllowN: maxWait 0 reduces to the immediate allow/reject decision. A false
// return means no token was obtained (deadline exceeded or ctx done), and no
// token is consumed in that case.
func (b *TokenBucket) WaitN(ctx context.Context, n float64, maxWait time.Duration) bool {
	if maxWait <= 0 {
		return b.AllowN(n) // no queueing budget: immediate allow/reject
	}
	deadline := time.Now().Add(maxWait)
	if d, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || d.Before(deadline) {
			deadline = d
		}
	}
	for {
		b.mu.Lock()
		now := time.Now()
		b.tokens += now.Sub(b.last).Seconds() * b.rate
		if ceiling := b.cap(now); b.tokens > ceiling {
			b.tokens = ceiling
		}
		b.last = now
		if b.tokens >= n {
			b.tokens -= n
			b.mu.Unlock()
			return true
		}
		// Tokens missing and how long until n of them refill.
		need := time.Duration((n - b.tokens) / b.rate * float64(time.Second))
		b.mu.Unlock()
		if now.Add(need).After(deadline) {
			return false
		}
		sleep := need
		if sleep > 10*time.Millisecond {
			sleep = 10 * time.Millisecond // re-check often; refill is continuous
		}
		if !timeutil.Sleep(ctx, sleep) {
			return false
		}
	}
}
