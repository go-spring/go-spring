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

// server_executor_default.go is the bundled driver's INBOUND engine: the executor a
// route admits requests by. It runs the protection stages an inbound call has a
// meaning for — rate limit, circuit breaker, bulkhead, handling budget — and no
// retry, because a handler that already produced side effects cannot be replayed.
//
// It is a type of its own rather than the outbound engine restricted to a
// [ServerPolicy]: the two directions are separate decisions, and one engine shared
// between them would have to express one direction's model in terms of the
// other's. What IS shared are the organs — the breaker
// (executor_default_breaker.go), the rate-limit budget (ratelimit.go) and the
// [serviceState] holding both — and they take their knobs in their own vocabulary
// (see [breakerSpec], [RateSpec]) so that no direction reaches them.

package resilience

import (
	"context"
	"errors"
	"go-spring.org/cloud/chain"
	"sync"
	"time"

	"go-spring.org/log"
)

// serverExecutor protects ONE route — the one it was built for — so that a
// route under pressure does not trip inbound for any other. Being bound to its
// route is what makes every stage's state single: one breaker, one bulkhead, one
// rate budget (or, when the driver was handed a store, that store's budget for
// this route — which is what carries a limit past the process when the store is
// over a shared backend).
type serverExecutor struct {
	service  string
	policy   ServerPolicy
	counters Counters

	// mu guards state, rate and built, which the first call establishes as a set.
	mu       sync.Mutex
	built    bool
	state    serviceState
	rate     rateState
	listener BreakerEventListener
}

// newServerExecutor builds the executor for one inbound route. Only the policy
// is established here; the state a call runs on (breaker, bulkhead, rate budget)
// is built on first use by [serverExecutor.snapshot], because it must see the
// breaker listener — and [WrapServerExecutor] attaches that after this returns.
func newServerExecutor(service string, p ServerPolicy, counters Counters) *serverExecutor {
	return &serverExecutor{service: service, policy: p, counters: counters}
}

// snapshot returns the state and rate budget a request should run on, building
// them on first use. Deferring the build to the first call is load-bearing:
// [WrapServerExecutor] attaches the breaker listener while the executor is still
// private (see [BreakerEventListenerSetter]), so the breaker must be created
// against a listener that is already in place.
func (e *serverExecutor) snapshot() (serviceState, rateState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.built {
		e.state = e.buildState()
		e.rate = newRateState(e.policy.RateSpec())
		e.built = true
	}
	return e.state, e.rate
}

// buildState builds the stages whose state belongs to this route alone, against
// its policy and listener. Callers must hold mu.
func (e *serverExecutor) buildState() serviceState {
	state := serviceState{}
	spec := e.policy.breakerSpec()
	if spec.active() {
		open := e.policy.OpenDuration
		if open <= 0 {
			open = 5 * time.Second
		}
		state.breaker = newCircuitBreaker(spec, open, e.service, e.listener)
	}
	if e.policy.MaxConcurrent > 0 {
		// A buffered channel is a non-blocking counting semaphore: a full buffer
		// means the route is at capacity, so excess requests are rejected rather
		// than queued.
		state.sem = make(chan struct{}, e.policy.MaxConcurrent)
	}
	return state
}

// SetBreakerEventListener attaches l so this route's breaker emits state
// transitions. It satisfies [BreakerEventListenerSetter] and must be called before
// the executor's first [serverExecutor.Execute], which is when the breaker is
// built — [WrapServerExecutor] attaches the listener while the executor is still
// being constructed inside the manager, so it never escapes unobserved.
func (e *serverExecutor) SetBreakerEventListener(l BreakerEventListener) {
	e.listener = l
}

// Execute admits one request under the route's policy, or rejects it with
// [chain.ErrRateLimited], [chain.ErrCircuitOpen] or [chain.ErrBulkheadFull] before fn runs. Unlike
// the outbound engine it runs the handler at most once — inbound has no retry
// stage — so the handling budget bounds the call itself rather than one attempt of
// several.
//
// The per-call recorder is not populated here: the inbound emitter carries its own
// duration and status (see [wrappedServerExecutor.Execute]), and a single attempt
// has no retry cost to decompose.
func (e *serverExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	s, rate := e.snapshot()

	// The bulkhead bounds concurrent in-flight requests to the route; one slot is
	// held for the whole call and released when it returns.
	if s.sem != nil {
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		default:
			return chain.ErrBulkheadFull
		}
	}

	// The rate-limit stage charges the request. Where the count goes depends on
	// what this executor was built with: a store, when the driver was handed one
	// (shared with every other executor spending it, and with every replica when
	// that store is over a shared backend), or otherwise a budget of its own,
	// charging this route. A store failure is not an inbound decision: the
	// request proceeds rather than turning an unreachable counter backend into an
	// outage.
	if lerr := allowRate(ctx, rate, e.policy.RateSpec(), e.service, e.counters); lerr != nil {
		if !errors.Is(lerr, chain.ErrRateLimited) {
			log.Warn(ctx, log.TagAppDef, log.Err(lerr),
				log.Msg("resilience: rate-limit counters unavailable, allowing request"))
		} else {
			return chain.ErrRateLimited
		}
	}

	if s.breaker != nil && !s.breaker.allow() {
		return chain.ErrCircuitOpen
	}

	start := time.Now()
	err := e.runOnce(ctx, fn)
	if s.breaker != nil {
		s.breaker.record(err == nil, time.Since(start))
	}
	return err
}

// runOnce applies the handling budget, if any, around fn. Inbound has one attempt,
// so this budget is the whole call's time, not a per-attempt slice.
func (e *serverExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.AttemptTimeout <= 0 {
		return fn(ctx)
	}
	budgetCtx, cancel := context.WithTimeout(ctx, e.policy.AttemptTimeout)
	defer cancel()
	return fn(budgetCtx)
}

func (e *serverExecutor) Close() error { return nil }
