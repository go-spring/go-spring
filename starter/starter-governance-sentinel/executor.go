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
	breaker "go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"sync"

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/timeutil"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"

	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
)

// sentinelExecutor maps a backend-neutral policies onto sentinel-golang
// rules. sentinel keys everything by resource name, and this executor IS one
// resource — the service it was built for, whose rules are loaded lazily on the
// first call; retry and per-attempt timeout are applied
// around sentinel's entry check, since sentinel itself models neither.
type sentinelExecutor struct {
	service string

	// mu guards the three fields below: the resolved policy sentinel's rules were
	// built from, the generation that policy came from, and whether those rules
	// are in the SDK. They move together, so a call never runs against a policy
	// its rules do not match.
	mu       sync.Mutex
	resolved resilience.CallPolicy
	gen      uint64
	loaded   bool

	// loadedOnce records that sentinel has held rules for this resource at least
	// once, which is what decides whether a refused load is fatal (nothing is
	// governing the resource yet) or tolerated (the last good rules stay in
	// force).
	loadedOnce bool

	// mgr is the manager this executor follows the config through. sentinel keys
	// everything by resource name and keeps its statistics across a rule reload,
	// so a generation change is a RULE SWAP for this driver rather than a rebuild
	// of materialized state — the boundary that makes it the one backend that
	// cannot show the "state starts over" contract. It is set at construction and
	// never replaced.
	mgr *resilience.Manager
}

func newSentinelExecutor(service string, m *resilience.Manager) (*sentinelExecutor, error) {
	if m == nil {
		return nil, fmt.Errorf("sentinel: NewExecutor requires a non-nil Manager")
	}
	return &sentinelExecutor{service: service, mgr: m}, nil
}

// ensureRules loads this executor's flow, circuit-breaker and isolation rules once,
// translating the neutral Policy knobs into sentinel's own rule shapes through
// the shared helpers in rules.go (the same translation the inbound engine
// uses). The breaker rule is selected by [resilience.BreakerPolicy.BreakerStrategy]
// so the driver matches the builtin's semantics.
func (e *sentinelExecutor) ensureRules() error {
	service := e.service
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return nil
	}

	rp := e.resolved.Rate.Policy
	if rp.RateLimit > 0 {
		rule := flowRule(service, flowKnobs{
			rateLimit:  rp.RateLimit,
			window:     rp.Window,
			maxWait:    rp.RateLimitMaxWait,
			warmUp:     rp.RateLimitWarmUpDuration,
			coldFactor: rp.WarmUpColdFactor,
		})
		if _, err := flow.LoadRulesOfResource(service, []*flow.Rule{rule}); err != nil {
			return errutil.Explain(err, "resilience: load flow rule for %q", service)
		}
	}

	if e.breakerActive() {
		spec := e.resolved.Breaker.Policy.Spec()
		rule := breakerRule(service, breakerKnobs{
			strategy:       spec.Strategy,
			errorThreshold: spec.FailureThreshold,
			rateThreshold:  spec.FailureRate,
			slowThreshold:  spec.SlowThreshold,
			slowRate:       spec.SlowRate,
			minRequests:    spec.MinSamples,
			window:         spec.Window,
			open:           spec.OpenDuration,
			probeN:         spec.HalfOpenTrials,
		})
		if _, err := circuitbreaker.LoadRulesOfResource(service, []*circuitbreaker.Rule{rule}); err != nil {
			return errutil.Explain(err, "resilience: load breaker rule for %q", service)
		}
		// Keep this driver's own mirror of the breaker state so its metrics can
		// report it: sentinel exposes transitions only through a process-wide
		// listener, so the driver subscribes and records them itself.
		ensureRouteListener()
		registerBreakerRoute(service)
		ensureStateObserver()
	} else {
		// The switch can turn an active breaker off between generations: clear
		// the rule and stop mirroring its state, or the old rule keeps tripping.
		if _, err := circuitbreaker.LoadRulesOfResource(service, nil); err != nil {
			return errutil.Explain(err, "resilience: clear breaker rule for %q", service)
		}
		unregisterBreakerRoute(service)
	}

	e.loaded = true
	return nil
}

// breakerActive reports whether the resolved strategy's own thresholds are
// set, so the driver loads a breaker rule only when one can trip. This mirrors
// what the kernel's Spec.Validate demands; it lives here because "configured"
// is a policy question the driver answers before touching the kernel.
func (e *sentinelExecutor) breakerActive() bool {
	p := e.resolved.Breaker.Policy
	if !p.Enabled() {
		return false
	}
	spec := p.Spec()
	switch spec.Strategy {
	case breaker.StrategyErrorRate:
		return spec.FailureRate > 0 || (spec.SlowThreshold > 0 && spec.SlowRate > 0)
	case breaker.StrategySlowCall:
		return spec.SlowThreshold > 0 && spec.SlowRate > 0
	default: // Consecutive, ErrorCount
		return spec.FailureThreshold > 0
	}
}

// load brings this resource's rules up to date with the policy the manager now
// resolves, and returns the policy the call runs under. It is the whole of the
// driver's config-following: the manager pushes nothing, so the resource notices
// on its own next call that the generation moved and swaps sentinel's rules.
//
// Sentinels's rule loading is live — LoadRulesOfResource replaces the rule set
// while the resource keeps its statistics — so a generation change costs a rule
// swap, not a state reset. That is the honest boundary of this backend: the
// framework cannot reach inside the SDK to rebuild what it materialized, and
// sentinel would not want it to.
//
// A load the backend refuses is consumed rather than retried: the resource keeps
// the rules it already has, so one bad policy does not cost a rule load per
// call — and on the FIRST load there are no rules to keep, so the error is
// returned and the call fails, which is what a caller wants to hear when the
// resource is not being governed at all.
func (e *sentinelExecutor) load() (resilience.CallPolicy, error) {
	p := e.mgr.ResolveLabel(e.service)
	gen := p.Gen

	e.mu.Lock()
	if e.loaded && e.gen == gen {
		cur := e.resolved
		e.mu.Unlock()
		return cur, nil
	}
	e.resolved, e.gen, e.loaded = p, gen, false
	e.mu.Unlock()

	var err error
	if p.Rate.Policy.RateLimit < 0 {
		err = errutil.Explain(nil, "resilience: negative rate limit %v", p.Rate.Policy.RateLimit)
	} else {
		err = e.ensureRules()
	}
	return e.settled(p, err)
}

// settled records the outcome of one load attempt. A success marks the
// generation dealt with. A failure does too once the resource has held rules
// before — the last good ones stay in force and the failure is a warning. It does
// NOT while the resource has never held any: there is nothing governing it, so
// the generation stays unconsumed and every call keeps reporting the error until
// the config is fixed.
func (e *sentinelExecutor) settled(p resilience.CallPolicy, err error) (resilience.CallPolicy, error) {
	e.mu.Lock()
	if err == nil {
		e.loadedOnce = true
	}
	if err == nil || e.loadedOnce {
		e.loaded = true
	}
	tolerated := e.loadedOnce
	e.mu.Unlock()

	if err == nil || !tolerated {
		return p, err
	}
	log.Warn(context.Background(), log.TagAppDef,
		"resilience: sentinel refused a policy change, keeping the resource's loaded rules",
		log.String("service", e.service), log.Err(err))
	return p, nil
}

func (e *sentinelExecutor) Execute(ctx context.Context, call chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	// The policy this call runs under, brought up to date first: sentinel holds
	// the rules, so following the config means swapping them when the generation
	// moves — see [sentinelExecutor.load].
	policy, err := e.load()
	if err != nil {
		return err
	}

	// Timeout bounds the whole call — every round and the backoff between them —
	// as one deadline, mirroring the builtin.
	callCtx := ctx
	if policy.Limits.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, policy.Limits.Timeout)
		defer cancel()
	}

	// eff is the retry node this call runs under — the policy node when the
	// governance center pushed a schedule (it wins whole), otherwise the call's own
	// node — resolved by the SAME merge the builtin engine uses, so the two
	// engines cannot disagree on retry. The zero node means a single attempt.
	eff := resilience.EffectiveRetry(policy.Limits.Retry, call.Retry)
	attempts := eff.MaxRetries + 1
	// A non-idempotent call runs once whatever the schedule says — the retry would
	// repeat a side effect, not an attempt. The builtin engine has always honored
	// this; the sentinel engine reads the same merged node so it does too now.
	if attempts > 1 && eff.NonIdempotent {
		attempts = 1
	}
	for i := range attempts {
		// Per-attempt Entry drives sentinel's flow and circuit-breaking rules.
		entry, blockErr := sentinel.Entry(e.service, sentinel.WithTrafficType(base.Outbound))
		if blockErr != nil {
			return mapBlockError(blockErr)
		}

		err = fn(callCtx)
		// A caller that walked away is not a verdict about the downstream, so it
		// must not feed sentinel's circuit-breaker statistics. A policy-budget
		// timeout still does: that is the unit failing to serve in time.
		if err != nil && !resilience.CallerAborted(ctx, err) {
			sentinel.TraceError(entry, err)
		}
		entry.Exit()

		if err == nil {
			return nil
		}
		if callCtx.Err() != nil {
			break
		}
		if !resilience.ShouldRetry(err) {
			break
		}
		if i == attempts-1 {
			break
		}
		if !timeutil.Sleep(callCtx, resilience.Backoff(eff, i)) {
			break
		}
	}
	return err
}

// Close drops this executor's breaker mirror so a torn-down executor stops
// reporting and the sentinel demultiplexer holds no stale entry.
func (e *sentinelExecutor) Close() error {
	unregisterBreakerRoute(e.service)
	return nil
}

// mapBlockError translates sentinel's block reason into the framework's neutral
// sentinel errors so callers depend only on
// go-spring.org/cloud/governance/dataplane/outbound/resilience.
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
