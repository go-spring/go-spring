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

// counters.go is the gateway's own in-memory [ratelimit.ClusterLimiter]. The
// rate-limit filter keys budgets by route or route|client-ip — an unbounded
// key space a process-local store must bound itself (hence the eviction) — and
// it reaches only this process: a store over a shared backend, contributed as
// the container's ClusterLimiter bean, covers every replica instead, and the wiring
// prefers it when present.

package StarterGateway

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
)

// maxScopes bounds how many keys the store tracks before it starts evicting
// idle ones; the gateway keys budgets by things possibly unbounded (client
// IPs), so without a cap those keys would grow the map for the process's life.
const (
	maxScopes = 4096
	idleScope = 10 * time.Minute
)

// newMemoryLimiter returns the process-local store the rate-limit filter
// falls back to when the wiring provided no shared one.
func newMemoryLimiter() ratelimit.ClusterLimiter {
	return &memoryLimiter{scopes: map[string]*scopeState{}}
}

// memoryLimiter keeps per-key counter state so independent budgets do not
// interfere. The concrete state type depends on the configured algorithm.
type memoryLimiter struct {
	mu     sync.Mutex
	scopes map[string]*scopeState
}

// scopeState is one key's counters plus the config they were built for, so a
// policy change (a hot-reloaded rate) starts that key's budget over.
type scopeState struct {
	spec   ratelimit.Spec
	bucket *ratelimit.TokenBucket
	window *ratelimit.SlidingWindow
	used   time.Time
}

func (c *memoryLimiter) Allow(ctx context.Context, scope string, spec ratelimit.Spec, n int) (bool, error) {
	if spec.RateLimit <= 0 || n <= 0 { // no budget configured
		return true, nil
	}
	s := c.state(scope, spec)
	if s.window != nil {
		return s.window.AllowN(n), nil
	}
	return s.bucket.WaitN(ctx, float64(n), spec.MaxWait), nil
}

// state returns scope's counters, rebuilding them when the rate-limit
// configuration changed since they were built.
func (c *memoryLimiter) state(scope string, spec ratelimit.Spec) *scopeState {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.scopes[scope]; ok && s.spec == spec {
		s.used = now
		return s
	}
	s := &scopeState{spec: spec, used: now}
	if spec.Algorithm == ratelimit.AlgorithmSlidingWindow {
		win := spec.Window
		if win <= 0 {
			win = time.Second
		}
		limit := spec.RateLimit * win.Seconds()
		if limit < 1 {
			limit = 1
		}
		s.window = ratelimit.NewSlidingWindow(limit, win)
	} else {
		burst := spec.Burst
		if burst <= 0 {
			// A small burst keeps steady traffic from being clipped by timing
			// jitter while still bounding spikes.
			if burst = int(spec.RateLimit); burst < 1 {
				burst = 1
			}
		}
		s.bucket = ratelimit.NewTokenBucket(spec.RateLimit, burst).WithWarmUp(spec.WarmUp, spec.ColdFactor)
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
func (c *memoryLimiter) evictLocked(now time.Time) {
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
