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
	"go-spring.org/cloud/chain"
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
// translating the neutral Policy knobs into sentinel's own rule shapes through
// the shared helpers in rules.go (the same translation the inbound engine
// uses). The breaker rule is selected by [resilience.ClientPolicy.BreakerStrategy]
// so the driver matches the builtin's semantics.
func (e *sentinelExecutor) ensureRules() error {
	service := e.service
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return nil
	}

	if e.policy.RateLimit > 0 {
		rule := flowRule(service, flowKnobs{
			rateLimit:  e.policy.RateLimit,
			window:     e.policy.Window,
			maxWait:    e.policy.RateLimitMaxWait,
			warmUp:     e.policy.RateLimitWarmUpDuration,
			coldFactor: e.policy.WarmUpColdFactor,
		})
		if _, err := flow.LoadRulesOfResource(service, []*flow.Rule{rule}); err != nil {
			return errutil.Explain(err, "resilience: load flow rule for %q", service)
		}
	}

	if e.policy.BreakerActive() {
		rule := breakerRule(service, breakerKnobs{
			strategy:       e.policy.ResolvedBreakerStrategy(),
			errorThreshold: e.policy.ErrorThreshold,
			rateThreshold:  e.policy.ErrorRateThreshold,
			slowThreshold:  e.policy.SlowCallDurationThreshold,
			slowRate:       e.policy.SlowCallRateThreshold,
			minRequests:    e.policy.MinRequests,
			window:         e.policy.BreakerWindow,
			open:           e.policy.OpenDuration,
			probeN:         e.policy.HalfOpenRequests,
		})
		if _, err := circuitbreaker.LoadRulesOfResource(service, []*circuitbreaker.Rule{rule}); err != nil {
			return errutil.Explain(err, "resilience: load breaker rule for %q", service)
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
		if _, err := isolation.LoadRulesOfResource(isoResource, []*isolation.Rule{
			isolationRule(isoResource, e.policy.MaxConcurrent),
		}); err != nil {
			return errutil.Explain(err, "resilience: load isolation rule for %q", isoResource)
		}
	}

	e.loaded = true
	return nil
}

func (e *sentinelExecutor) Execute(ctx context.Context, call chain.Operation, fn func(context.Context) error) error {
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

	// eff is the retry node this call runs under — the policy node when the
	// governance center pushed a schedule (it wins whole), otherwise the call's own
	// node — resolved by the SAME merge the builtin engine uses, so the two
	// engines cannot disagree on retry. The zero node means a single attempt.
	eff := resilience.EffectiveRetry(e.policy.Retry, call.Retry)
	attempts := eff.MaxRetries + 1
	// A non-idempotent call runs once whatever the schedule says — the retry would
	// repeat a side effect, not an attempt. The builtin engine has always honored
	// this; the sentinel engine reads the same merged node so it does too now.
	if attempts > 1 && eff.NonIdempotent {
		attempts = 1
	}
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
		if !resilience.ShouldRetry(err) {
			break
		}
		if i == attempts-1 {
			break
		}
		if !timeutil.Sleep(budgetCtx, resilience.Backoff(eff, i)) {
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
		return fmt.Errorf("%w: %s", chain.ErrCircuitOpen, b.Error())
	case base.BlockTypeIsolation:
		return fmt.Errorf("%w: %s", chain.ErrBulkheadFull, b.Error())
	default:
		return fmt.Errorf("%w: %s", chain.ErrRateLimited, b.Error())
	}
}
