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

// budget.go is the direction-neutral rate-limit stage above the counting
// algorithms in this package: the stage's vocabulary ([Spec], [Algorithm], the
// [ClusterLimiter] seam) and the one budget object both call directions hold.
//
// Outbound calls (go-spring.org/cloud/runtime/dataplane/outbound/resilience) and inbound requests
// (go-spring.org/cloud/runtime/dataplane/inbound/admission) keep separate policies, but a request is charged
// against the same budget here. Both directions also accept the same
// [ClusterLimiter] store, so a Redis-backed limit covers outbound calls and
// inbound requests without an adapter between two identical interfaces.

package ratelimit

import (
	"context"
	"time"
)

// Algorithm names the counting strategy a rate limit asks for. The empty value
// means [AlgorithmTokenBucket].
type Algorithm string

const (
	// AlgorithmTokenBucket refills tokens continuously at [Spec.RateLimit] up to
	// [Spec.Burst]; it smooths bursts and is the default.
	AlgorithmTokenBucket Algorithm = "token-bucket"

	// AlgorithmSlidingWindow counts events over a rolling [Spec.Window], giving
	// a hard cap on events per window with less burst tolerance than a token
	// bucket.
	AlgorithmSlidingWindow Algorithm = "sliding-window"
)

// Spec is the rate-limit stage's own vocabulary: the knobs a budget is built
// from. Each direction assembles it from its policy, so the stage — and any
// [ClusterLimiter] store behind it — never sees a direction. A zero RateLimit
// means unlimited.
type Spec struct {
	// RateLimit is the sustained budget in units per second. 0 disables the
	// stage.
	RateLimit float64

	// Burst is how many units may exceed RateLimit momentarily; 0 means a
	// small multiple of RateLimit.
	Burst int

	// Algorithm picks the counting scheme; empty means [AlgorithmTokenBucket].
	Algorithm Algorithm

	// Window is the rolling interval [AlgorithmSlidingWindow] counts over; 0
	// means one second. Ignored by the token bucket.
	Window time.Duration

	// MaxWait is how long an over-limit charge may WAIT for a unit before being
	// refused; 0 refuses immediately.
	MaxWait time.Duration

	// WarmUp is the cold-start ramp-in period; 0 disables warm-up. Ignored by
	// the sliding window.
	WarmUp time.Duration

	// ColdFactor shapes the ramp-in: admission starts at roughly
	// RateLimit/ColdFactor and rises to full by the end of WarmUp. 0 means the
	// default (3).
	ColdFactor int
}

// ClusterLimiter is the rate-limit stage's cross-replica seam: a store (Redis,
// typically) that holds one budget across every replica. When a caller supplies
// one, the store is charged instead of the process-local budget.
//
// Allow reports whether n units were admitted. admitted=false with a nil error
// means the budget is exhausted; a non-nil error means the store itself could
// not be read or updated. The two are deliberately separate: a store failure is
// not a budget decision, so each direction decides what to do with it (inbound
// guard allows the request; an outbound call surfaces the error).
//
// Implementations must be safe for concurrent use and must keep each scope's
// counters apart.
type ClusterLimiter interface {
	Allow(ctx context.Context, scope string, spec Spec, n int) (admitted bool, err error)
}

// Budget is the process-local budget ONE protected unit holds, built from a
// [Spec]. Its zero value means "no rate limit configured", which admits
// everything.
type Budget struct {
	bucket  *TokenBucket
	window  *SlidingWindow
	maxWait time.Duration
}

// NewBudget builds the local budget spec asks for.
func NewBudget(spec Spec) Budget {
	if spec.RateLimit <= 0 {
		return Budget{}
	}
	if spec.Algorithm == AlgorithmSlidingWindow {
		win := spec.Window
		if win <= 0 {
			win = time.Second
		}
		limit := max(spec.RateLimit*win.Seconds(), 1)
		return Budget{window: NewSlidingWindow(limit, win)}
	}
	burst := spec.Burst
	if burst <= 0 {
		burst = max(int(spec.RateLimit), 1)
	}
	return Budget{
		bucket: NewTokenBucket(spec.RateLimit, burst).
			WithWarmUp(spec.WarmUp, spec.ColdFactor),
		maxWait: spec.MaxWait,
	}
}

// Allow charges n units against the local budget, waiting up to the spec's
// MaxWait (bounded by ctx) when it is exhausted.
func (b Budget) Allow(ctx context.Context, n int) bool {
	if n <= 0 {
		return true
	}
	if b.window != nil {
		return b.window.AllowN(n)
	}
	if b.bucket != nil {
		return b.bucket.WaitN(ctx, float64(n), b.maxWait)
	}
	return true
}

// Charge spends n units of spec's budget for scope: the supplied store when
// there is one (that is what extends the budget past this process), otherwise
// the local budget. It reports whether the units were admitted; a non-nil error
// means the store failed, which is not an admission decision.
func Charge(ctx context.Context, limiter ClusterLimiter, scope string, spec Spec, budget Budget, n int) (bool, error) {
	if limiter != nil {
		return limiter.Allow(ctx, scope, spec, n)
	}
	return budget.Allow(ctx, n), nil
}
