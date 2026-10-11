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
// own budget ([rateState]) and the [ClusterLimiter] seam (where a cross-replica
// budget lives). The counting algorithms live in go-spring.org/cloud/ratelimit, a
// leaf package any process-local ClusterLimiter store can build on. The stage's policy knobs
// (rate-limit/burst/algorithm/window/rate-limit-max-wait) live with every other
// stage's in client_policy.go / server_policy.go; the seam that builds an
// [chain.Executor] over them is driver.go, and the stage runs inside
// client_executor.go.
//
// Rate limiting is a stage of a protected call, not an API of its own: the call
// either gets its budget or comes back as [chain.ErrRateLimited]. By default the stage
// counts in a budget the executor holds for its service, exactly like the other
// stages hold their state — one budget per service, since one executor serves
// one service and the manager builds one executor per label.
//
// One thing goes beyond that default, and it is a [ClusterLimiter] store's property:
// HOW WIDE a budget reaches. Supply one over a shared backend (Redis, typically)
// and the limit covers every replica. With no store, the budget is this
// executor's alone.

package resilience

import (
	"context"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/ratelimit"
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

// RateSpec is the rate-limit stage's own vocabulary: the knobs a budget is built
// from. Both directions assemble it from their policy (see
// [ClientPolicy.RateSpec] / [ServerPolicy.RateSpec]), so the stage — and any
// [ClusterLimiter] store behind it — never sees a direction and needs no second
// implementation for inbound. A zero RateLimit means unlimited.
type RateSpec struct {
	// RateLimit is the sustained budget in units per second. 0 disables the
	// stage.
	RateLimit float64

	// Burst is how many units may exceed RateLimit momentarily; 0 means the
	// stage's default (a small multiple of RateLimit).
	Burst int

	// Algorithm picks the counting scheme; empty means [TokenBucket], the other
	// value is [SlidingWindow].
	Algorithm Algorithm

	// Window is the rolling interval [SlidingWindow] counts over; 0 means one
	// second. Ignored by [TokenBucket].
	Window time.Duration

	// MaxWait is how long an over-limit call may WAIT for a unit before being
	// rejected; 0 keeps the reject-immediately behavior. Ignored when RateLimit is
	// 0.
	MaxWait time.Duration

	// WarmUp is the cold-start ramp-in period: the budget grows from a small
	// fraction of full to full over this duration instead of admitting the full
	// rate from the first instant. 0 disables warm-up. Ignored by
	// [SlidingWindow].
	WarmUp time.Duration

	// ColdFactor shapes the ramp-in: admission starts at roughly
	// RateLimit/ColdFactor and rises to full by the end of WarmUp. 0 means the
	// stage's default (3).
	ColdFactor int
}

// ClusterLimiter is the rate-limit stage's cluster-wide seam: the store a rate
// limit lives in when it must hold across every replica, not just this process.
// Handing one to a driver changes one thing about the executors it builds, and
// only that: their budgets live in the store instead of the executor, so a limit
// over a shared backend (Redis, typically) holds across replicas, with each call
// charged under the service the executor is bound to. Everything else about the
// stage stays the executor's own.
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
type ClusterLimiter interface {
	// Allow consumes n units of scope's budget under spec. It returns
	// [chain.ErrRateLimited] when the budget is exhausted (after waiting up to
	// [RateSpec.MaxWait]) and the store's own error when the counters could not be
	// read or updated. A zero [RateSpec.RateLimit] means unlimited, and n <= 0
	// always passes.
	Allow(ctx context.Context, scope string, spec RateSpec, n int) error
}

// allowRate charges one unit of the rate-limit stage for scope — an executor's
// service, or an inbound executor's route — and reports [chain.ErrRateLimited] when
// there is none. It spends the supplied store when the driver was handed one
// (that is what gives the budget a reach beyond this executor) and otherwise the
// budget the caller holds (rate). The store is charged under scope: an executor
// covers one service or route, so keys belong to whoever needs them, not here.
func allowRate(ctx context.Context, rate rateState, spec RateSpec, scope string, limiter ClusterLimiter) error {
	if limiter != nil {
		return limiter.Allow(ctx, scope, spec, 1)
	}
	if spec.RateLimit <= 0 {
		return nil
	}
	if rate.window != nil {
		if rate.window.AllowN(1) {
			return nil
		}
		return chain.ErrRateLimited
	}
	if rate.bucket.WaitN(ctx, 1, spec.MaxWait) {
		return nil
	}
	return chain.ErrRateLimited
}

// rateState is the budget ONE executor or inbound executor holds for its ONE
// service or route, built from a [RateSpec]. The zero value means "no rate limit
// configured", which the stage treats as unlimited.
type rateState struct {
	bucket *ratelimit.TokenBucket
	window *ratelimit.SlidingWindow
}

// newRateState builds the local budget spec asks for.
func newRateState(spec RateSpec) rateState {
	if spec.RateLimit <= 0 {
		return rateState{}
	}
	if spec.Algorithm == SlidingWindow {
		win := spec.Window
		if win <= 0 {
			win = time.Second
		}
		limit := spec.RateLimit * win.Seconds()
		if limit < 1 {
			limit = 1
		}
		return rateState{window: ratelimit.NewSlidingWindow(limit, win)}
	}
	burst := spec.Burst
	if burst <= 0 {
		// A small burst keeps steady traffic from being clipped by timing jitter
		// while still bounding spikes.
		if burst = int(spec.RateLimit); burst < 1 {
			burst = 1
		}
	}
	return rateState{bucket: ratelimit.NewTokenBucket(spec.RateLimit, burst).WithWarmUp(spec.WarmUp, spec.ColdFactor)}
}
