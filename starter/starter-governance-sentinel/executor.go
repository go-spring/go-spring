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

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/timeutil"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"

	"go-spring.org/cloud/governance/resilience"
)

// sentinelExecutor maps a backend-neutral resilience.ClientPolicy onto sentinel-golang
// rules. sentinel keys everything by resource name, and this executor IS one
// resource — the service it was built for, whose rules are loaded lazily on the
// first call; retry and per-attempt timeout are applied
// around sentinel's entry check, since sentinel itself models neither.
type sentinelExecutor struct {
	service string

	// mu guards the three fields below: the policy sentinel's rules were built
	// from, the generation that policy came from, and whether those rules are in
	// the SDK. They move together, so a call never runs against a policy its
	// rules do not match.
	mu     sync.Mutex
	policy resilience.ClientPolicy
	gen    uint64
	loaded bool

	// loadedOnce records that sentinel has held rules for this resource at least
	// once, which is what decides whether a refused load is fatal (nothing is
	// governing the resource yet) or tolerated (the last good rules stay in
	// force).
	loadedOnce bool

	// source is the manager, when there is one: sentinel's way of following the
	// config. sentinel keys everything by resource name and keeps its statistics
	// across a rule reload, so a generation change is a RULE SWAP for this driver
	// rather than a rebuild of materialized state — the boundary that makes it
	// the one backend that cannot show the "state starts over" contract.
	source resilience.PolicySource

	listener atomic.Pointer[resilience.BreakerEventListener]
}

// SetPolicySource installs src, the manager this executor follows the config
// through (see [resilience.PolicySourceSetter]). It satisfies the
// construction-time handshake the manager asserts while the executor is still
// private, and must not be called after the first call.
func (e *sentinelExecutor) SetPolicySource(src resilience.PolicySource) {
	e.source = src
}

// SetBreakerEventListener attaches l and routes sentinel breaker state changes
// for the resource this executor serves (via ensureRules) to it. Satisfies
// [resilience.BreakerEventListenerSetter]; observe-resilience uses it.
func (e *sentinelExecutor) SetBreakerEventListener(l resilience.BreakerEventListener) {
	ensureRouteListener()
	e.listener.Store(&l)
}

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

	e.loaded = true
	return nil
}

// load brings this resource's rules up to date with the policy the manager now
// resolves, and returns the policy the call runs under. It is the whole of the
// driver's config-following: the manager pushes nothing (see
// [resilience.PolicySource]), so the resource notices on its own next call that
// the generation moved and swaps sentinel's rules.
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
func (e *sentinelExecutor) load() (resilience.ClientPolicy, error) {
	p, gen := e.constructionPolicy(), uint64(0)
	if e.source != nil {
		c := e.source.ResolveLabel(e.service)
		p, gen = c.Label, c.Gen
	}

	e.mu.Lock()
	if e.loaded && e.gen == gen {
		cur := e.policy
		e.mu.Unlock()
		return cur, nil
	}
	e.policy, e.gen, e.loaded = p, gen, false
	e.mu.Unlock()

	var err error
	if p.RateLimit < 0 {
		err = errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
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
func (e *sentinelExecutor) settled(p resilience.ClientPolicy, err error) (resilience.ClientPolicy, error) {
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

// constructionPolicy is the policy the driver was built with, read under mu.
func (e *sentinelExecutor) constructionPolicy() resilience.ClientPolicy {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.policy
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
	if policy.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, policy.Timeout)
		defer cancel()
	}

	// eff is the retry node this call runs under — the policy node when the
	// governance center pushed a schedule (it wins whole), otherwise the call's own
	// node — resolved by the SAME merge the builtin engine uses, so the two
	// engines cannot disagree on retry. The zero node means a single attempt.
	eff := resilience.EffectiveRetry(policy.Retry, call.Retry)
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
		if err != nil {
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

func (e *sentinelExecutor) Close() error { return nil }

// mapBlockError translates sentinel's block reason into the framework's neutral
// sentinel errors so callers depend only on
// go-spring.org/cloud/governance/resilience.
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
