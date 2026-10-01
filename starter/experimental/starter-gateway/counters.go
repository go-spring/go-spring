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

// counters.go is the gateway's own in-memory [resilience.Counters]. The
// rate-limit filter keys budgets by route or route|client-ip — an unbounded
// key space a process-local store must bound itself (hence the eviction) — and
// it reaches only this process: a store over a shared backend, contributed as
// the container's Counters bean, covers every replica instead, and the wiring
// prefers it when present.

package StarterGateway

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/timeutil"
)

// maxScopes bounds how many keys the store tracks before it starts evicting
// idle ones; the gateway keys budgets by things possibly unbounded (client
// IPs), so without a cap those keys would grow the map for the process's life.
const (
	maxScopes = 4096
	idleScope = 10 * time.Minute
)

// newMemoryCounters returns the process-local store the rate-limit filter
// falls back to when the wiring provided no shared one.
func newMemoryCounters() resilience.Counters {
	return &memoryCounters{scopes: map[string]*scopeState{}}
}

// memoryCounters keeps per-key counter state so independent budgets do not
// interfere. The concrete state type depends on the configured algorithm.
type memoryCounters struct {
	mu     sync.Mutex
	scopes map[string]*scopeState
}

// scopeState is one key's counters plus the config they were built for, so a
// policy change (a hot-reloaded rate) starts that key's budget over.
type scopeState struct {
	spec   rateSpec
	bucket *tokenBucket
	window *slidingWindow
	used   time.Time
}

// rateSpec is the part of a rate-limit policy the counters depend on.
type rateSpec struct {
	rate      float64
	burst     int
	window    time.Duration
	algorithm resilience.Algorithm
}

func (c *memoryCounters) Allow(ctx context.Context, scope string, p resilience.ClientPolicy, n int) error {
	if p.RateLimit <= 0 || n <= 0 { // no budget configured
		return nil
	}
	s := c.state(scope, p)
	if s.window != nil {
		if s.window.allowN(n) {
			return nil
		}
		return resilience.ErrRateLimited
	}
	if s.bucket.waitN(ctx, float64(n), p.RateLimitMaxWait) {
		return nil
	}
	return resilience.ErrRateLimited
}

// state returns scope's counters, rebuilding them when the policy's rate-limit
// configuration changed since they were built.
func (c *memoryCounters) state(scope string, p resilience.ClientPolicy) *scopeState {
	spec := rateSpec{rate: p.RateLimit, burst: p.Burst, window: p.Window, algorithm: p.Algorithm}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.scopes[scope]; ok && s.spec == spec {
		s.used = now
		return s
	}
	s := &scopeState{spec: spec, used: now}
	if p.Algorithm == resilience.SlidingWindow {
		win := p.Window
		if win <= 0 {
			win = time.Second
		}
		limit := p.RateLimit * win.Seconds()
		if limit < 1 {
			limit = 1
		}
		s.window = &slidingWindow{limit: limit, window: win, curStart: now}
	} else {
		burst := p.Burst
		if burst <= 0 {
			// A small burst keeps steady traffic from being clipped by timing
			// jitter while still bounding spikes.
			if burst = int(p.RateLimit); burst < 1 {
				burst = 1
			}
		}
		s.bucket = newTokenBucket(p.RateLimit, burst)
	}
	c.scopes[scope] = s
	if len(c.scopes) > maxScopes {
		c.evictLocked(now)
	}
	return s
}

// evictLocked drops the keys that have gone idle, and then — if the store is
// still over its cap, which only an unbounded key space can cause — arbitrary
// ones until it is under. Re-creating an evicted key starts a fresh bucket.
func (c *memoryCounters) evictLocked(now time.Time) {
	for k, s := range c.scopes {
		if now.Sub(s.used) > idleScope {
			delete(c.scopes, k)
		}
	}
	for k := range c.scopes {
		if len(c.scopes) <= maxScopes {
			return
		}
		delete(c.scopes, k)
	}
}

// tokenBucket is a minimal, dependency-free rate limiter. Tokens refill
// continuously at rate per second up to burst.
type tokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		rate:   rate,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// allowN consumes n tokens if at least n are available.
func (b *tokenBucket) allowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}

// waitN consumes n tokens, waiting up to maxWait (and no longer than ctx
// allows) when the bucket is momentarily empty. It is the queueing form of
// allowN: maxWait 0 reduces to the immediate allow/reject decision. A false
// return means no token was obtained (deadline exceeded or ctx done), and no
// token is consumed in that case.
func (b *tokenBucket) waitN(ctx context.Context, n float64, maxWait time.Duration) bool {
	if maxWait <= 0 {
		return b.allowN(n) // no queueing budget: immediate allow/reject
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
		if b.tokens > b.burst {
			b.tokens = b.burst
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

// slidingWindow approximates a rolling-window counter with the standard
// weighted two-window estimate: it blends the previous window's count by the
// fraction of it still overlapping the current instant. This bounds events per
// window without storing a timestamp per event.
type slidingWindow struct {
	mu        sync.Mutex
	limit     float64
	window    time.Duration
	curStart  time.Time
	curCount  float64
	prevCount float64
}

func (w *slidingWindow) allowN(n int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(w.curStart)
	if elapsed >= w.window {
		// Roll forward: an adjacent window becomes prev; a gap of two or more
		// windows means the old counts have fully aged out.
		if elapsed >= 2*w.window {
			w.prevCount = 0
		} else {
			w.prevCount = w.curCount
		}
		w.curCount = 0
		w.curStart = now
		elapsed = 0
	}
	weight := float64(w.window-elapsed) / float64(w.window)
	estimate := w.prevCount*weight + w.curCount
	if estimate+float64(n) > w.limit {
		return false
	}
	w.curCount += float64(n)
	return true
}
