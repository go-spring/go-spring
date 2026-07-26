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
	"sort"
	"sync"
	"time"
)

// RateLimiter is a first-class, standalone throttle: it answers "may this unit
// of work against key run right now?" and consumes budget when it says yes. It
// is separate from [Executor] on purpose — the executor bundles limiting with
// breaking/retry/timeout for outbound calls, whereas a RateLimiter is the bare
// flow-control primitive you place anywhere (an inbound HTTP middleware, a
// per-tenant quota, a background job pacer).
//
// The seam is what makes distributed limiting pluggable: the bundled builtin
// driver keeps counters in-process (per-replica limiting), while a Redis-backed
// driver enforces a single global budget shared across replicas — same
// interface, selected by driver name. Implementations must be safe for
// concurrent use.
type RateLimiter interface {
	// Allow reports whether one unit of work against key may proceed now,
	// consuming one token when it returns true. key scopes independent budgets
	// (per tenant, per route, ...); pass a constant for a single global budget.
	Allow(ctx context.Context, key string) (bool, error)

	// AllowN is [RateLimiter.Allow] for n units at once. It is all-or-nothing: it
	// consumes n tokens and returns true, or consumes none and returns false.
	AllowN(ctx context.Context, key string, n int) (bool, error)

	// Close releases any background resources (connections, pumps). It is safe to
	// call more than once.
	Close() error
}

// Algorithm names the counting strategy a [LimitPolicy] asks for.
type Algorithm string

const (
	// TokenBucket refills tokens continuously at Rate up to Burst; it smooths
	// bursts and is the default.
	TokenBucket Algorithm = "token-bucket"
	// SlidingWindow counts events over a rolling Window, giving a hard cap on
	// events per window with less burst tolerance than a token bucket.
	SlidingWindow Algorithm = "sliding-window"
)

// LimitPolicy is a backend-neutral description of a rate limit. Each
// [LimiterDriver] maps it onto its own primitives (the builtin driver reads it
// directly; a Redis driver translates it into a Lua token-bucket script). A
// zero policy (Rate 0) is an unlimited pass-through.
type LimitPolicy struct {
	// Rate is the sustained permitted throughput in operations per second. 0
	// disables limiting (every call is allowed).
	Rate float64

	// Burst is the maximum momentary excess over Rate for [TokenBucket]. It
	// defaults to a small multiple of Rate when unset and is ignored by
	// [SlidingWindow].
	Burst int

	// Algorithm selects the counting strategy; empty means [TokenBucket].
	Algorithm Algorithm

	// Window is the rolling interval for [SlidingWindow]; it defaults to one
	// second and is ignored by [TokenBucket]. The window cap is Rate*Window.
	Window time.Duration
}

// LimiterDriver builds a [RateLimiter] from a [LimitPolicy]. Backends implement
// it and register under a name via [RegisterLimiter].
type LimiterDriver interface {
	NewRateLimiter(LimitPolicy) (RateLimiter, error)
}

var (
	limiterMu       sync.RWMutex
	limiterRegistry = map[string]LimiterDriver{}
)

// RegisterLimiter makes a [LimiterDriver] available under name. It panics if
// name is empty, d is nil, or name is already registered, mirroring the
// driver-registry idiom used elsewhere ([RegisterDriver], discovery.Register) so
// duplicate wiring fails loudly at init.
func RegisterLimiter(name string, d LimiterDriver) {
	if name == "" {
		panic("resilience: register limiter with empty name")
	}
	if d == nil {
		panic("resilience: register nil limiter driver for " + name)
	}
	limiterMu.Lock()
	defer limiterMu.Unlock()
	if _, ok := limiterRegistry[name]; ok {
		panic("resilience: limiter driver already registered: " + name)
	}
	limiterRegistry[name] = d
}

// GetLimiter returns the [LimiterDriver] registered under name.
func GetLimiter(name string) (LimiterDriver, bool) {
	limiterMu.RLock()
	defer limiterMu.RUnlock()
	d, ok := limiterRegistry[name]
	return d, ok
}

// MustGetLimiter returns the [LimiterDriver] registered under name, or an error
// listing the available drivers when none matches.
func MustGetLimiter(name string) (LimiterDriver, error) {
	if d, ok := GetLimiter(name); ok {
		return d, nil
	}
	limiterMu.RLock()
	names := make([]string, 0, len(limiterRegistry))
	for k := range limiterRegistry {
		names = append(names, k)
	}
	limiterMu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("resilience: no limiter driver registered as %q (registered: %v)", name, names)
}

// The bundled "default" limiter driver: in-process counters, no third-party
// dependencies. It limits each replica independently — select a Redis-backed
// driver for a single budget shared across replicas.
func init() { RegisterLimiter("default", builtinLimiterDriver{}) }

type builtinLimiterDriver struct{}

func (builtinLimiterDriver) NewRateLimiter(p LimitPolicy) (RateLimiter, error) {
	if p.Rate < 0 {
		return nil, fmt.Errorf("resilience: negative rate %v", p.Rate)
	}
	return &builtinRateLimiter{policy: p, states: map[string]any{}}, nil
}

// builtinRateLimiter keeps per-key counter state so independent budgets do not
// interfere. The concrete state type depends on the configured algorithm.
type builtinRateLimiter struct {
	policy LimitPolicy
	mu     sync.Mutex
	states map[string]any
}

func (l *builtinRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

func (l *builtinRateLimiter) AllowN(_ context.Context, key string, n int) (bool, error) {
	if l.policy.Rate == 0 { // unlimited
		return true, nil
	}
	if n <= 0 {
		return true, nil
	}
	switch l.policy.Algorithm {
	case SlidingWindow:
		return l.window(key).allowN(n), nil
	default:
		return l.bucket(key).allowN(float64(n)), nil
	}
}

func (l *builtinRateLimiter) bucket(key string) *tokenBucket {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s, ok := l.states[key].(*tokenBucket); ok {
		return s
	}
	burst := l.policy.Burst
	if burst <= 0 {
		if burst = int(l.policy.Rate); burst < 1 {
			burst = 1
		}
	}
	b := newTokenBucket(l.policy.Rate, burst)
	l.states[key] = b
	return b
}

func (l *builtinRateLimiter) window(key string) *slidingWindow {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s, ok := l.states[key].(*slidingWindow); ok {
		return s
	}
	win := l.policy.Window
	if win <= 0 {
		win = time.Second
	}
	limit := l.policy.Rate * win.Seconds()
	if limit < 1 {
		limit = 1
	}
	w := &slidingWindow{limit: limit, window: win, curStart: time.Now()}
	l.states[key] = w
	return w
}

// allowN consumes n tokens if at least n are available. It extends the bucket
// used by the executor (allow consumes exactly one) for multi-unit checks.
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

func (l *builtinRateLimiter) Close() error { return nil }
