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

// executor.go holds the runtime protection seam: the [Executor] interface (the
// single seam every client adapter calls), the [Fallback] degradation helper,
// and the bundled "default" executor — the implementation the "default" driver
// (driver.go) builds from a [Policy]. It keeps per-resource limiter/breaker/
// bulkhead state and threads each call through them with retry + timeout. The
// breaker and rate-limiter primitives it composes live in breaker.go and
// ratelimit.go.

package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Executor runs operations under a [Policy]. It is the single seam every client
// adapter calls; implementations must be safe for concurrent use.
type Executor interface {
	// Execute runs fn under the policy, scoping rate-limiter and circuit-breaker
	// state to resource (typically a downstream service name). It returns
	// [ErrRateLimited] or [ErrCircuitOpen] when the call is rejected before fn
	// runs, or fn's own (final) error otherwise. The context passed to fn may be
	// a per-attempt timeout derived from ctx.
	Execute(ctx context.Context, resource string, fn func(context.Context) error) error

	// Close releases any background resources held by the executor (e.g. metric
	// pumps in a production driver). It is safe to call more than once.
	Close() error

	// Refresh adopts p as the new policy at runtime — hot-reloading
	// rate/breaker/bulkhead/retry thresholds without rebuilding the bean.
	// Refreshing resets per-resource protection state (breaker counters, token
	// buckets, bulkhead slots): a new policy starts clean, which is the
	// intended semantic of "the threshold changed". Every driver implements
	// this so adapters driven by a config binding can call it directly
	// when the bound policy changes.
	Refresh(p Policy) error
}

// Fallback runs fn through exec and, when the operation is rejected (rate
// limited, circuit open, bulkhead full) or fails after all retries, invokes
// degrade to produce a graceful result instead of surfacing the error. It is
// the degradation stage of the framework and composes with any [Executor]
// regardless of driver: degrade receives the triggering error so it can serve
// cached data for [ErrCircuitOpen] yet propagate a genuine bug, for example.
//
// degrade's own error (or nil) becomes the final result. When exec is nil the
// call is a transparent pass-through: fn runs once and its error, if any, still
// reaches degrade, so wiring stays a no-op until a policy is configured.
func Fallback(ctx context.Context, exec Executor, resource string,
	fn func(context.Context) error, degrade func(context.Context, error) error) error {
	var err error
	if exec == nil {
		err = fn(ctx)
	} else {
		err = exec.Execute(ctx, resource, fn)
	}
	if err == nil {
		return nil
	}
	return degrade(ctx, err)
}

// defaultExecutor keeps per-resource limiter and breaker state so that one
// misbehaving downstream does not trip protection for the others.
type defaultExecutor struct {
	policy   Policy
	mu       sync.Mutex
	states   map[string]*resourceState
	listener BreakerEventListener
}

// SetBreakerEventListener attaches l so every per-resource breaker built by this
// executor (including ones created later for new resources) emits state
// transitions. It satisfies [BreakerEventListenerSetter].
//
// It must be called before the executor is shared — that is, before its first
// [defaultExecutor.Execute], which is when the per-resource breakers are built.
// [WrapExecutor] attaches the listener while the executor is still being
// constructed inside [resolve], so the executor never escapes unobserved.
func (e *defaultExecutor) SetBreakerEventListener(l BreakerEventListener) {
	e.listener = l
}

// Refresh adopts p as the new policy and resets all per-resource state, so the
// next Execute rebuilds breakers/limiters/bulkheads against the new thresholds.
// It is the [Executor.Refresh] implementation for the default driver.
// Per-resource state is discarded: a fresh breaker/limiter starts from zero,
// which is the intended semantic of a threshold change (the old failure counts
// were counted under the old policy).
func (e *defaultExecutor) Refresh(p Policy) error {
	if p.RateLimit < 0 {
		return fmt.Errorf("resilience: negative rate limit %v", p.RateLimit)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policy = p
	e.states = map[string]*resourceState{}
	return nil
}

type resourceState struct {
	bucket  *tokenBucket
	breaker *circuitBreaker
	sem     chan struct{}
}

func (e *defaultExecutor) state(resource string) *resourceState {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.states[resource]
	if ok {
		return s
	}
	s = &resourceState{}
	if e.policy.RateLimit > 0 {
		burst := e.policy.Burst
		if burst <= 0 {
			// A small burst keeps steady traffic from being clipped by timing
			// jitter while still bounding spikes.
			if burst = int(e.policy.RateLimit); burst < 1 {
				burst = 1
			}
		}
		s.bucket = newTokenBucket(e.policy.RateLimit, burst)
	}
	if e.policy.BreakerActive() {
		open := e.policy.OpenDuration
		if open <= 0 {
			open = 5 * time.Second
		}
		s.breaker = newCircuitBreaker(e.policy, open, resource, e.listener)
	}
	if e.policy.MaxConcurrent > 0 {
		// A buffered channel is a non-blocking counting semaphore: a full buffer
		// means the bulkhead is at capacity, so excess calls are rejected rather
		// than queued.
		s.sem = make(chan struct{}, e.policy.MaxConcurrent)
	}
	e.states[resource] = s
	return s
}

func (e *defaultExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	s := e.state(resource)

	// The bulkhead bounds concurrent in-flight calls to the resource; one slot
	// is held for the whole Execute (retries included) and released when done.
	if s.sem != nil {
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		default:
			return ErrBulkheadFull
		}
	}

	// MaxDuration caps the whole call across retries. It only tightens the
	// caller's own context; per-attempt Timeout is applied inside runOnce
	// against budgetCtx, so the effective per-attempt bound is min(Timeout,
	// remaining MaxDuration).
	budgetCtx := ctx
	if e.policy.MaxDuration > 0 {
		var cancel context.CancelFunc
		budgetCtx, cancel = context.WithTimeout(ctx, e.policy.MaxDuration)
		defer cancel()
	}

	attempts := e.policy.MaxRetries + 1
	var err error
	ran := false // whether at least one attempt actually invoked fn this call
	for i := range attempts {
		if s.bucket != nil && !s.bucket.allowN(1) {
			return ErrRateLimited
		}
		if s.breaker != nil && !s.breaker.allow() {
			// Breaker rejected the attempt. If an earlier attempt already ran
			// this call — e.g. it won the half-open trial permit, failed, and a
			// retry now finds the gate spent — fold that outcome in once so a
			// won trial is never left unresolved (which would stick the breaker
			// half-open with a consumed permit). When nothing ran yet there is
			// no sample to record.
			if ran {
				s.breaker.record(err == nil)
			}
			return ErrCircuitOpen
		}

		err = e.runOnce(budgetCtx, fn)
		ran = true
		if err == nil {
			break
		}
		// A cancelled/Expired budget (caller cancel or MaxDuration) ends the
		// loop before the predicate is consulted: there is no budget left for
		// another attempt regardless of why the last one failed.
		if budgetCtx.Err() != nil {
			break
		}
		if !e.policy.ShouldRetry(err) {
			break
		}
		if i == attempts-1 {
			break // last attempt — no backoff sleep after it
		}
		if !SleepFor(budgetCtx, e.policy.Backoff(i)) {
			break // backoff interrupted by ctx cancellation
		}
	}
	// The breaker measures the outcome of one protected call, not each retry
	// attempt. Recording once per logical Execute — rather than once per
	// attempt inside the loop — stops a retrying client from amplifying a
	// single downstream failure into N breaker samples, which would trip the
	// circuit far faster than the configured ErrorThreshold / error-rate
	// implies (the "resilience on => breaker trips instantly" symptom). The
	// rate limiter above intentionally still charges per attempt, because each
	// attempt is a real downstream request the limiter exists to bound.
	if s.breaker != nil {
		s.breaker.record(err == nil)
	}
	return err
}

// runOnce applies the per-attempt timeout, if any, around fn. The ctx it
// receives is already bounded by the Execute-level MaxDuration budget.
func (e *defaultExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.Timeout <= 0 {
		return fn(ctx)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, e.policy.Timeout)
	defer cancel()
	return fn(attemptCtx)
}

func (e *defaultExecutor) Close() error { return nil }
