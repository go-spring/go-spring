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

package StarterGovernanceSentinel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/timeutil"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/isolation"

	"go-spring.org/cloud/resilience"
)

// sentinelExecutor maps a backend-neutral resilience.ClientPolicy onto sentinel-golang
// rules. sentinel keys everything by resource name, and this executor IS one
// resource — the service it was built for, whose rules are loaded lazily on the
// first call; retry and per-attempt timeout are applied
// around sentinel's entry check, since sentinel itself models neither.
type sentinelExecutor struct {
	policy  resilience.ClientPolicy
	service string

	mu       sync.Mutex
	loaded   bool
	listener atomic.Pointer[resilience.BreakerEventListener]
}

// SetBreakerEventListener attaches l and routes sentinel breaker state changes
// for the resource this executor serves (via ensureRules) to it. Satisfies
// [resilience.BreakerEventListenerSetter]; observe-resilience uses it.
func (e *sentinelExecutor) SetBreakerEventListener(l resilience.BreakerEventListener) {
	ensureRouteListener()
	e.listener.Store(&l)
}

// isoSuffix namespaces the bulkhead (isolation) resource name so an Entry that holds
// the concurrency slot across all retries does not collide with the per-attempt
// Entry sentinel uses for flow + circuit breaking. sentinel evaluates every rule
// type registered under a resource on each Entry for that resource, so the two
// concerns must live under distinct resource names to be acquired independently.
const isoSuffix = "$bulkhead"

func newSentinelExecutor(service string, p resilience.ClientPolicy) (*sentinelExecutor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	return &sentinelExecutor{service: service, policy: p}, nil
}

// ensureRules loads this executor's flow, circuit-breaker and isolation rules once,
// translating the neutral Policy knobs into sentinel's own rule shapes. The
// breaker rule is selected by [resilience.ClientPolicy.BreakerStrategy] so the driver
// matches the builtin's semantics, and both strategies set ProbeNum=1 for an
// exactly-one-trial half-open (aligning with the builtin's single-permit gate).
func (e *sentinelExecutor) ensureRules() error {
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
		// Route sentinel breaker state changes for this service to the
		// attached listener (if any), so observe-resilience gets the events.
		if l := e.listener.Load(); l != nil {
			registerBreakerRoute(service, *l)
		}
	}

	if e.policy.MaxConcurrent > 0 {
		// The isolation rule lives under the bulkhead-suffixed resource name so it is
		// acquired once for the whole Execute (see Execute below) and held across
		// retries, matching the builtin's bulkhead-scope contract.
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
// sentinel's strategy from [resilience.ClientPolicy.BreakerStrategy]. Both strategies
// share the same stat window and a single half-open probe so they align with
// the builtin driver rather than silently diverging.
func (e *sentinelExecutor) loadBreakerRule(service string) error {
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

// Refresh adopts p as the new policy and clears the loaded flag, so
// the next Execute reloads flow/breaker/isolation rules under the new thresholds.
// It is the [resilience.ClientExecutor.Refresh] implementation for the sentinel driver.
//
// sentinel's LoadRulesOfResource replaces a service's existing rules, so an
// already-loaded service gets fresh rules (and a reset breaker stat window) on
// its next Execute. This mirrors the default driver's "discard state, rebuild on
// next call" lazy semantic. Already-registered breaker route listeners are
// re-registered when ensureRules re-runs.
func (e *sentinelExecutor) Refresh(p resilience.ClientPolicy) error {
	if p.RateLimit < 0 {
		return errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policy = p
	e.loaded = false
	return nil
}

func (e *sentinelExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := e.ensureRules(); err != nil {
		return err
	}

	// Bulkhead: one Entry under the suffixed resource, held for the whole
	// Execute (retries included) via defer. This restores the bulkhead-scope
	// invariant the builtin upholds.
	if e.policy.MaxConcurrent > 0 {
		isoEntry, blockErr := sentinel.Entry(e.service+isoSuffix, sentinel.WithTrafficType(base.Outbound))
		if blockErr != nil {
			return mapBlockError(blockErr)
		}
		defer isoEntry.Exit()
	}

	// MaxDuration caps the whole call across retries, mirroring the builtin.
	budgetCtx := ctx
	if e.policy.MaxDuration > 0 {
		var cancel context.CancelFunc
		budgetCtx, cancel = context.WithTimeout(ctx, e.policy.MaxDuration)
		defer cancel()
	}

	attempts := e.policy.MaxRetries + 1
	var err error
	for i := range attempts {
		// Per-attempt Entry drives sentinel's flow and circuit-breaking rules.
		entry, blockErr := sentinel.Entry(e.service, sentinel.WithTrafficType(base.Outbound))
		if blockErr != nil {
			return mapBlockError(blockErr)
		}

		err = e.runOnce(budgetCtx, fn)
		if err != nil {
			sentinel.TraceError(entry, err)
		}
		entry.Exit()

		if err == nil {
			return nil
		}
		if budgetCtx.Err() != nil {
			break
		}
		if !e.policy.ShouldRetry(err) {
			break
		}
		if i == attempts-1 {
			break
		}
		if !timeutil.Sleep(budgetCtx, e.policy.Backoff(i)) {
			break
		}
	}
	return err
}

// runOnce applies the per-attempt timeout, if any, around fn. The ctx it
// receives is already bounded by the Execute-level MaxDuration budget.
func (e *sentinelExecutor) runOnce(ctx context.Context, fn func(context.Context) error) error {
	if e.policy.AttemptTimeout <= 0 {
		return fn(ctx)
	}
	attemptCtx, cancel := context.WithTimeout(ctx, e.policy.AttemptTimeout)
	defer cancel()
	return fn(attemptCtx)
}

func (e *sentinelExecutor) Close() error { return nil }

// mapBlockError translates sentinel's block reason into the framework's neutral
// sentinel errors so callers depend only on
// go-spring.org/cloud/resilience.
func mapBlockError(b *base.BlockError) error {
	switch b.BlockType() {
	case base.BlockTypeCircuitBreaking:
		return fmt.Errorf("%w: %s", resilience.ErrCircuitOpen, b.Error())
	case base.BlockTypeIsolation:
		return fmt.Errorf("%w: %s", resilience.ErrBulkheadFull, b.Error())
	default:
		return fmt.Errorf("%w: %s", resilience.ErrRateLimited, b.Error())
	}
}
