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

package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// The bundled "default" driver: a self-contained implementation with no
// third-party dependencies, so the framework is usable out of the box and in
// tests. Production deployments select the recommended sentinel-golang driver
// (separate module, registers itself as "sentinel" on blank import) purely by
// changing the driver name — the [Executor] seam and every adapter stay put.
func init() { RegisterDriver("default", builtinDriver{}) }

type builtinDriver struct{}

func (builtinDriver) NewExecutor(p Policy) (Executor, error) {
	if p.RateLimit < 0 {
		return nil, fmt.Errorf("resilience: negative rate limit %v", p.RateLimit)
	}
	return &builtinExecutor{policy: p, states: map[string]*resourceState{}}, nil
}

// builtinExecutor keeps per-resource limiter and breaker state so that one
// misbehaving downstream does not trip protection for the others.
type builtinExecutor struct {
	policy Policy
	mu     sync.Mutex
	states map[string]*resourceState
}

type resourceState struct {
	bucket  *tokenBucket
	breaker *circuitBreaker
}

func (e *builtinExecutor) state(resource string) *resourceState {
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
	if e.policy.ErrorThreshold > 0 {
		open := e.policy.OpenDuration
		if open <= 0 {
			open = 5 * time.Second
		}
		s.breaker = &circuitBreaker{threshold: e.policy.ErrorThreshold, openFor: open}
	}
	e.states[resource] = s
	return s
}

func (e *builtinExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	s := e.state(resource)

	attempts := e.policy.MaxRetries + 1
	var err error
	for range attempts {
		if s.bucket != nil && !s.bucket.allow() {
			return ErrRateLimited
		}
		if s.breaker != nil && !s.breaker.allow() {
			return ErrCircuitOpen
		}

		err = e.runOnce(ctx, fn)

		if s.breaker != nil {
			s.breaker.record(err == nil)
		}
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return err
}

// runOnce applies the per-attempt timeout, if any, around fn.
func (e *builtinExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.Timeout <= 0 {
		return fn(ctx)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, e.policy.Timeout)
	defer cancel()
	return fn(attemptCtx)
}

func (e *builtinExecutor) Close() error { return nil }

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

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// circuitBreaker is a consecutive-failure breaker with a half-open trial. It
// opens after threshold consecutive failures and, once openFor has elapsed,
// admits a single trial request whose outcome closes or re-opens it.
type circuitBreaker struct {
	threshold int
	openFor   time.Duration

	mu       sync.Mutex
	failures int
	openedAt time.Time
	halfOpen bool
}

// allow reports whether a request may proceed given the current breaker state.
func (c *circuitBreaker) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openedAt.IsZero() {
		return true // closed
	}
	if time.Since(c.openedAt) < c.openFor {
		return false // open, cooling down
	}
	// Cool-down elapsed: admit one trial request (half-open).
	c.halfOpen = true
	return true
}

// record folds an attempt's outcome back into the breaker state.
func (c *circuitBreaker) record(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if success {
		c.failures = 0
		c.openedAt = time.Time{}
		c.halfOpen = false
		return
	}
	if c.halfOpen {
		// Trial failed: re-open the cool-down window.
		c.halfOpen = false
		c.openedAt = time.Now()
		return
	}
	c.failures++
	if c.failures >= c.threshold {
		c.openedAt = time.Now()
	}
}
