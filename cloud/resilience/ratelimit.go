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

// ratelimit.go is the rate-limit stage of the resilience package: the executor's
// own budget ([rateState]), the [Counters] seam (where a cross-replica budget
// lives), and the counting algorithms. The stage's policy knobs
// (rate-limit/burst/algorithm/window/rate-limit-max-wait) live with every other
// stage's in policy.go; the seam that builds an [ClientExecutor] over them is
// executor.go, and the stage runs inside executor_default.go.
//
// Rate limiting is a stage of a protected call, not an API of its own: the call
// either gets its budget or comes back as [ErrRateLimited]. By default the stage
// counts in a budget the executor holds for its service, exactly like the other
// stages hold their state — one budget per service, since one executor serves
// one service and the manager builds one executor per label.
//
// One thing goes beyond that default, and it is a [Counters] store's property:
// HOW WIDE a budget reaches. Supply one over a shared backend (Redis, typically)
// and the limit covers every replica. With no store, the budget is this
// executor's alone.

package resilience

import (
	"context"
	"sync"
	"time"

	"go-spring.org/stdlib/timeutil"
)

// Algorithm names the counting strategy a rate limit asks for.
type Algorithm string

const (
	// TokenBucket refills tokens continuously at [ClientPolicy.RateLimit] up to
	// [ClientPolicy.Burst]; it smooths bursts and is the default.
	TokenBucket Algorithm = "token-bucket"
	// SlidingWindow counts events over a rolling [ClientPolicy.Window], giving a hard
	// cap on events per window with less burst tolerance than a token bucket.
	SlidingWindow Algorithm = "sliding-window"
)

// Counters is the rate-limit stage's counting seam. Handing one to a driver
// changes one thing about the executors it builds, and only that: their budgets
// live in the store instead of the executor, so a limit over a shared backend
// holds across replicas, with each call charged under the service the executor
// is bound to. Everything else about the stage stays the executor's own.
//
// A client never builds one: the container provides it if some backend starter
// contributed it, and the default driver passes it to the executors it builds
// (see [NewDefaultDriver]).
//
// Implementations must be safe for concurrent use and must keep each scope's
// counters apart: keeping one budget per scope is what makes a limit cover every
// caller that spends it. An implementation may also see only one policy per
// scope — alternating policies on one scope would ask it to restart that
// scope's budget on every call.
type Counters interface {
	// Allow consumes n units of scope's budget under p. It returns
	// [ErrRateLimited] when the budget is exhausted (after waiting up to
	// [ClientPolicy.RateLimitMaxWait]) and the store's own error when the counters
	// could not be read or updated. A zero [ClientPolicy.RateLimit] means unlimited,
	// and n <= 0 always passes.
	Allow(ctx context.Context, scope string, p ClientPolicy, n int) error
}

// allow charges one unit of the rate-limit stage for this executor's service and
// reports [ErrRateLimited] when there is none. It spends the supplied store when
// the driver was handed one — that is what gives the budget a reach beyond this
// executor — and otherwise the budget this executor holds (rate), built for the
// current policy. The store is charged under the executor's service: an executor
// covers one service, so keys belong to whoever needs them, not here.
func (e *defaultExecutor) allow(ctx context.Context, rate rateState, p ClientPolicy) error {
	if e.counters != nil {
		return e.counters.Allow(ctx, e.service, p, 1)
	}
	if p.RateLimit <= 0 {
		return nil
	}
	if rate.window != nil {
		if rate.window.allowN(1) {
			return nil
		}
		return ErrRateLimited
	}
	if rate.bucket.waitN(ctx, 1, p.RateLimitMaxWait) {
		return nil
	}
	return ErrRateLimited
}

// rateState is the budget ONE executor holds for its ONE service. It is rebuilt
// whenever the policy changes (see [defaultExecutor.adopt]), so a hot-reloaded
// rate starts its budget over exactly as a freshly built executor would, and the
// hot path never diffs policies. The zero value means "no rate limit
// configured", which the stage treats as unlimited.
type rateState struct {
	bucket *tokenBucket
	window *slidingWindow
}

// newRateState builds the local budget p asks for.
func newRateState(p ClientPolicy) rateState {
	if p.RateLimit <= 0 {
		return rateState{}
	}
	if p.Algorithm == SlidingWindow {
		win := p.Window
		if win <= 0 {
			win = time.Second
		}
		limit := p.RateLimit * win.Seconds()
		if limit < 1 {
			limit = 1
		}
		return rateState{window: &slidingWindow{limit: limit, window: win, curStart: time.Now()}}
	}
	burst := p.Burst
	if burst <= 0 {
		// A small burst keeps steady traffic from being clipped by timing jitter
		// while still bounding spikes.
		if burst = int(p.RateLimit); burst < 1 {
			burst = 1
		}
	}
	return rateState{bucket: newTokenBucket(p.RateLimit, burst)}
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

// waitN consumes n tokens, waiting up to maxWait (and no longer than ctx allows)
// when the bucket is momentarily empty. It is the queueing form of allowN:
// maxWait 0 reduces to the immediate allow/reject decision. A false return means
// no token was obtained (deadline exceeded or ctx done), and no token is
// consumed in that case.
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
