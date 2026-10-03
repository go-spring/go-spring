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

// server_executor.go is the sentinel driver's INBOUND engine: it maps a
// [resilience.ServerPolicy] onto sentinel-golang rules for one route. It is a type
// of its own rather than [sentinelExecutor] restricted to an inbound model — the
// two directions are separate decisions, and one engine shared between them would
// have to express one direction's model in terms of the other's. What the two
// engines DO share is sentinel itself (its rules, its breaker listener routing) and
// the mapping helpers around it.
//
// The stages are the ones inbound has a meaning for: a flow rule, a circuit
// breaker and an isolation (bulkhead) rule, plus the handling budget as the call's
// own bound. There is no retry — a handler that already produced side effects
// cannot be replayed — and no across-retry budget, since there are no retries to
// cap.

package StarterGovernanceSentinel

import (
	"context"
	"sync"
	"sync/atomic"

	"go-spring.org/stdlib/errutil"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/isolation"

	"go-spring.org/cloud/resilience"
)

// sentinelServerExecutor maps the inbound model onto sentinel-golang rules for
// ONE route. sentinel keys everything by resource name, and this executor IS one
// resource — the route it was built for, whose rules are loaded lazily on the first
// call; the handling budget is applied around sentinel's entry check, since
// sentinel itself does not model it.
type sentinelServerExecutor struct {
	policy  resilience.ServerPolicy
	service string

	mu       sync.Mutex
	loaded   bool
	listener atomic.Pointer[resilience.BreakerEventListener]
}

// newSentinelServerExecutor builds the inbound engine for one route. The
// rules themselves are registered on the first call (see
// [sentinelServerExecutor.ensureRules]).
func newSentinelServerExecutor(service string, p resilience.ServerPolicy) (*sentinelServerExecutor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	return &sentinelServerExecutor{service: service, policy: p}, nil
}

// SetBreakerEventListener attaches l and routes sentinel breaker state changes for
// the resource this executor serves (via ensureRules) to it. Satisfies
// [resilience.BreakerEventListenerSetter]; observe-resilience uses it.
func (e *sentinelServerExecutor) SetBreakerEventListener(l resilience.BreakerEventListener) {
	ensureRouteListener()
	e.listener.Store(&l)
}

// ensureRules loads this route's flow, circuit-breaker and isolation rules once,
// translating the inbound knobs into sentinel's own rule shapes. The breaker rule
// is selected by [resilience.ServerPolicy.BreakerStrategy] so the driver matches the
// bundled engine's semantics, and both strategies set ProbeNum=1 for an
// exactly-one-trial half-open.
func (e *sentinelServerExecutor) ensureRules() error {
	service := e.service
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return nil
	}

	if e.policy.RateLimit > 0 {
		if _, err := flow.LoadRulesOfResource(service, []*flow.Rule{{
			Resource:               service,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			Threshold:              e.policy.RateLimit,
			StatIntervalInMs:       1000,
		}}); err != nil {
			return errutil.Explain(err, "resilience: load flow rule for %q", service)
		}
	}

	if e.policy.BreakerActive() {
		if err := e.loadBreakerRule(service); err != nil {
			return err
		}
		if l := e.listener.Load(); l != nil {
			registerBreakerRoute(service, *l)
		}
	}

	if e.policy.MaxConcurrent > 0 {
		// The isolation rule lives under the bulkhead-suffixed resource name so it is
		// acquired once for the whole call (see Execute below), the same
		// bulkhead-scope contract the outbound engine upholds.
		isoResource := service + isoSuffix
		if _, err := isolation.LoadRulesOfResource(isoResource, []*isolation.Rule{{
			Resource:   isoResource,
			MetricType: isolation.Concurrency,
			Threshold:  uint32(e.policy.MaxConcurrent),
		}}); err != nil {
			return errutil.Explain(err, "resilience: load isolation rule for %q", isoResource)
		}
	}

	e.loaded = true
	return nil
}

// loadBreakerRule registers a circuit-breaker rule under service, choosing
// sentinel's strategy from [resilience.ServerPolicy.BreakerStrategy]. Both
// strategies share one stat window and a single half-open probe so they align with
// the bundled engine rather than silently diverging.
func (e *sentinelServerExecutor) loadBreakerRule(service string) error {
	openMs := uint32(e.policy.OpenDuration.Milliseconds())
	if openMs == 0 {
		openMs = 5000
	}
	winMs := uint32(e.policy.BreakerWindow.Milliseconds())
	if winMs == 0 {
		winMs = 1000
	}

	var rule *circuitbreaker.Rule
	switch e.policy.ResolvedBreakerStrategy() {
	case resilience.BreakerErrorRate:
		minReq := uint64(e.policy.MinRequests)
		if minReq == 0 {
			minReq = 1
		}
		rule = &circuitbreaker.Rule{
			Resource:         service,
			Strategy:         circuitbreaker.ErrorRatio,
			RetryTimeoutMs:   openMs,
			MinRequestAmount: minReq,
			StatIntervalMs:   winMs,
			Threshold:        e.policy.ErrorRateThreshold,
			ProbeNum:         1,
		}
	default: // BreakerConsecutive
		rule = &circuitbreaker.Rule{
			Resource:         service,
			Strategy:         circuitbreaker.ErrorCount,
			RetryTimeoutMs:   openMs,
			MinRequestAmount: 1,
			StatIntervalMs:   winMs,
			Threshold:        float64(e.policy.ErrorThreshold),
			ProbeNum:         1,
		}
	}
	if _, err := circuitbreaker.LoadRulesOfResource(service, []*circuitbreaker.Rule{rule}); err != nil {
		return errutil.Explain(err, "resilience: load breaker rule for %q", service)
	}
	return nil
}

// Execute admits one request under the route's policy, or rejects it with
// [chain.ErrRateLimited], [chain.ErrCircuitOpen] or
// [chain.ErrBulkheadFull] before fn runs. The handler runs at most once: there
// is no retry stage, so the handling budget bounds the call itself.
func (e *sentinelServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := e.ensureRules(); err != nil {
		return err
	}

	// Bulkhead: one Entry under the suffixed resource, held for the whole call via
	// defer.
	if e.policy.MaxConcurrent > 0 {
		isoEntry, blockErr := sentinel.Entry(e.service+isoSuffix, sentinel.WithTrafficType(base.Inbound))
		if blockErr != nil {
			return mapBlockError(blockErr)
		}
		defer isoEntry.Exit()
	}

	// The per-call Entry drives sentinel's flow and circuit-breaking rules.
	entry, blockErr := sentinel.Entry(e.service, sentinel.WithTrafficType(base.Inbound))
	if blockErr != nil {
		return mapBlockError(blockErr)
	}
	err := e.runOnce(ctx, fn)
	if err != nil {
		sentinel.TraceError(entry, err)
	}
	entry.Exit()
	return err
}

// runOnce applies the handling budget, if any, around fn. Inbound has one attempt,
// so this budget is the whole call's time, not a per-attempt slice.
func (e *sentinelServerExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.AttemptTimeout <= 0 {
		return fn(ctx)
	}
	budgetCtx, cancel := context.WithTimeout(ctx, e.policy.AttemptTimeout)
	defer cancel()
	return fn(budgetCtx)
}

func (e *sentinelServerExecutor) Close() error { return nil }
