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

// breaker.go is the circuit-breaker capability: the pushed document, its
// validation, and the checks a policy must pass before the executor can
// materialize it.

package resilience

import (
	"fmt"
	"hash"
	"hash/fnv"
	"time"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// BreakerPolicy is the circuit-breaker half of a policy. Every knob is
// zero-value-safe; a zero policy leaves the breaker off rather than enabling it
// with defaults nobody asked for.
type BreakerPolicy struct {
	// BreakerDisabled switches the breaker off while retaining its knobs: an
	// operator can turn protection off during an incident and re-enable it later
	// without losing the configuration. The switch defaults open — configuring
	// any breaker knob activates the breaker — and a disabled breaker ignores
	// its knobs instead of validating them.
	BreakerDisabled bool `value:"${breaker-disabled:=false}"`

	// ErrorThreshold is the failure count that trips the circuit breaker. Under
	// [circuitbreaker.StrategyConsecutive] (the default) it counts failures in a row; under
	// [circuitbreaker.StrategyErrorCount] it counts failures over the window.
	// When either strategy is selected and this is 0, the policy default is 5
	// (gobreaker's consecutive threshold).
	ErrorThreshold int `value:"${error-threshold:=0}" expr:"$ >= 0"`

	// OpenDuration is how long the circuit stays open before a trial request is
	// allowed through (half-open). Ignored when the breaker is disabled;
	// defaults to 5s (Hystrix's sleep window) when unset.
	OpenDuration time.Duration `value:"${open-duration:=0}" expr:"int($) >= 0"`

	// BreakerStrategy selects how the breaker counts failures. Empty means
	// [circuitbreaker.StrategyConsecutive] when only an error threshold is set.
	// The breaker is active when [BreakerPolicy.BreakerDisabled] is false and at
	// least one breaker knob is configured; selecting a strategy applies its
	// mainstream defaults to the trip condition (see [BreakerPolicy.Spec]).
	BreakerStrategy circuitbreaker.Strategy `value:"${breaker-strategy:=}" expr:"string($) == '' || string($) in ['consecutive', 'error-count', 'error-rate', 'slow-call']"`

	// ErrorRateThreshold is the failure ratio in [0,1] that trips an
	// [circuitbreaker.StrategyErrorRate] breaker. When error-rate is selected
	// and this is 0, the policy default is 0.5 (50%, the Hystrix/resilience4j
	// default). Pair it with [BreakerPolicy.MinRequests] and
	// [BreakerPolicy.BreakerWindow].
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}" expr:"$ >= 0 && $ <= 1"`

	// MinRequests is the minimum sample size observed in [BreakerPolicy.BreakerWindow]
	// before a windowed breaker may trip, so a 1/1 failure does not open the
	// circuit. It defaults to 5 (Sentinel's minRequestAmount) when unset;
	// ignored by the consecutive strategy.
	MinRequests int `value:"${min-requests:=0}" expr:"$ >= 0"`

	// BreakerWindow is the rolling interval the windowed strategies count over
	// (and the denominator's time span). It defaults to one second (Sentinel's
	// statIntervalMs) when unset. The consecutive strategy ignores it (it counts
	// an unbounded run); sentinel uses it as the stat interval for all
	// strategies.
	BreakerWindow time.Duration `value:"${breaker-window:=0}" expr:"int($) >= 0"`

	// SlowCallDurationThreshold is the latency above which an attempt
	// counts as a SLOW call for the breaker: a downstream that never errors but
	// answers slowly is unhealthy too. Pair it with
	// [BreakerPolicy.SlowCallRateThreshold]; the slow ratio is counted over the
	// same window as the error rate but fed by its own numerator, so slowness
	// never inflates the failure rate. The pair is required when the selected
	// strategy is slow-call, and an error-rate breaker may use it instead of (or
	// in addition to) [BreakerPolicy.ErrorRateThreshold].
	SlowCallDurationThreshold time.Duration `value:"${slow-call-duration-threshold:=0}" expr:"int($) >= 0"`

	// SlowCallRateThreshold is the slow-call ratio in (0,1] that trips the
	// breaker, counted over [BreakerPolicy.BreakerWindow] with
	// [BreakerPolicy.MinRequests] samples. Under
	// [circuitbreaker.StrategyErrorRate] it trips on its own even when
	// ErrorRateThreshold is 0; under [circuitbreaker.StrategySlowCall] it is the
	// only signal. Must be paired with
	// [BreakerPolicy.SlowCallDurationThreshold].
	SlowCallRateThreshold float64 `value:"${slow-call-rate-threshold:=0}" expr:"$ >= 0 && $ <= 1"`

	// HalfOpenRequests is how many trial requests the breaker admits in the
	// half-open state before deciding: all N succeeding closes the circuit,
	// the first failure re-opens it. 0 and 1 both mean the historical single
	// trial. A larger N is robust against a flapping first probe. Ignored when
	// the breaker is disabled.
	HalfOpenRequests int `value:"${half-open-requests:=0}" expr:"$ >= 0"`

	// HalfOpenTimeout bounds how long a half-open round may wait for its trial
	// outcomes. 0 (the default, matching resilience4j's
	// maxWaitDurationInHalfOpenState and gobreaker's lack of a half-open
	// deadline) waits indefinitely: low traffic keeps the circuit half-open and
	// every arriving call is another trial. A positive value re-opens the
	// circuit when the round outlives it, so a lost or hung trial cannot pin it
	// half-open forever. Ignored when the breaker is disabled.
	HalfOpenTimeout time.Duration `value:"${half-open-timeout:=0}" expr:"int($) >= 0"`
}

// IsZero reports whether this breaker configures nothing.
func (p BreakerPolicy) IsZero() bool {
	return !p.BreakerDisabled && !p.hasConfig()
}

// Enabled reports whether the breaker stage is on: the operator has not
// disabled it and at least one breaker knob is configured. A false value is a
// valid OFF stage — the holder keeps a nil *Breaker — not a construction error.
func (p BreakerPolicy) Enabled() bool {
	return !p.BreakerDisabled && p.hasConfig()
}

// hasConfig reports whether any breaker knob is configured. The switch
// itself is not a knob: "disabled" only means something when there is a
// configuration to disable. An empty policy therefore leaves the breaker off
// rather than enabling it with defaults nobody asked for.
func (p BreakerPolicy) hasConfig() bool {
	return p.BreakerStrategy != "" || p.ErrorThreshold != 0 || p.OpenDuration != 0 ||
		p.ErrorRateThreshold != 0 || p.MinRequests != 0 || p.BreakerWindow != 0 ||
		p.SlowCallDurationThreshold != 0 || p.SlowCallRateThreshold != 0 ||
		p.HalfOpenRequests != 0 || p.HalfOpenTimeout != 0
}

// Policy-level breaker defaults, taken from the mainstream engines where a
// defensible default exists: five consecutive failures (gobreaker), a 50%
// failure rate (Hystrix and resilience4j), and five minimum samples in one
// one-second window (Sentinel's minRequestAmount/statIntervalMs). They apply
// only when the operator selects a strategy without spelling out its trip
// condition; a slow-call pair has no useful default and stays required.
const (
	defaultFailureThreshold = 5
	defaultFailureRate      = 0.5
	defaultMinSamples       = 5
)

// Spec maps p onto the breaker's own vocabulary, applying the policy defaults.
// It is the one mapping validation and materialization use, so every driver
// sees the same resolved knobs. Callers gate on [BreakerPolicy.Enabled] first;
// an OFF breaker is a nil *Breaker, not a Spec.
func (p BreakerPolicy) Spec() circuitbreaker.Spec {
	spec := circuitbreaker.Spec{
		Strategy:         p.ResolvedStrategy(),
		FailureThreshold: p.ErrorThreshold,
		FailureRate:      p.ErrorRateThreshold,
		MinSamples:       p.MinRequests,
		Window:           p.BreakerWindow,
		SlowThreshold:    p.SlowCallDurationThreshold,
		SlowRate:         p.SlowCallRateThreshold,
		HalfOpenTrials:   p.HalfOpenRequests,
		HalfOpenTimeout:  p.HalfOpenTimeout,
		OpenDuration:     p.OpenDuration,
	}
	switch spec.Strategy {
	case circuitbreaker.StrategyConsecutive, circuitbreaker.StrategyErrorCount:
		if spec.FailureThreshold == 0 {
			spec.FailureThreshold = defaultFailureThreshold
		}
	case circuitbreaker.StrategyErrorRate:
		if spec.FailureRate == 0 {
			spec.FailureRate = defaultFailureRate
		}
	}
	if spec.MinSamples == 0 {
		spec.MinSamples = defaultMinSamples
	}
	return spec
}

// ResolvedStrategy returns the strategy a driver should apply: an explicit
// [BreakerPolicy.BreakerStrategy], or the [circuitbreaker.StrategyConsecutive]
// default when unset. The strategy decides which threshold fields count — a
// slow-call pair without an explicit error-rate/slow-call strategy is not
// inferred, it is a config the caller should fix.
func (p BreakerPolicy) ResolvedStrategy() circuitbreaker.Strategy {
	if p.BreakerStrategy != "" {
		return p.BreakerStrategy
	}
	return circuitbreaker.StrategyConsecutive
}

// BreakerSettings is the circuit-breaker half of the outbound document. This
// type IS the document: the control-plane breaker root binds it and hands it
// back unchanged (see [BreakerSettings.Validate]), so there is no separate
// binding twin.
type BreakerSettings struct {
	// Services groups the rules by the label they govern. Duplicate groups are
	// order-shadowed: the first group naming a label wins. There is no implicit
	// default: an instance with no matching rule runs with no breaker.
	Services []BreakerService `value:"${services:=}"`
}

// BreakerService is one label's policy block: every rule inside it governs the
// same service. A service-scope rule WINS OUTRIGHT over the interface-level
// rules of its service, so the two never compete; interface rules are consulted
// only when the service has no service rule.
type BreakerService struct {
	// Service is the label these rules apply to, exact-compare. Required.
	Service string `value:"${service:=}" expr:"$ != ''"`

	// Rules are the breakers for this service.
	Rules []BreakerRule `value:"${rules:=}"`
}

// Validate reports whether every breaker policy in s can be materialized.
// conf.Bind calls it, so an invalid document is rejected at the source boundary
// rather than reaching the center.
//
// Breakers accept both scopes. A service rule WINS OUTRIGHT over the
// interface-level rules of its service, so the two never compete; interface
// rules are consulted only when the service has no service rule.
func (s BreakerSettings) Validate() error {
	for i := range s.Services {
		g := &s.Services[i]
		for j := range g.Rules {
			r := &g.Rules[j]
			if err := validateBreakerPolicy(r.BreakerPolicy); err != nil {
				return errutil.Explain(err, "resilience: breaker rule %q of %q", r.ActionMatch, g.Service)
			}
		}
	}
	return nil
}

// BreakerRule is one breaker rule inside a [BreakerService].
//
// Action is either [matcher.ServiceScopeAction] ("-") or a real exact/prefix. A service
// rule, when present, WINS OUTRIGHT: the interface-level rules of that service
// are not consulted. With no service rule, the most specific interface rule
// applies.
type BreakerRule struct {
	// Action selects the scope: "-" for the whole service, or a real
	// exact/prefix for one interface.
	ActionMatch matcher.ActionMatch `value:"${action:=}"`

	BreakerPolicy
}

// validateBreakerPolicy checks what the breaker cannot build from. Every scalar
// knob is owned by its expr tag (thresholds, windows); what remains is the
// cross-field choice of a strategy and its trip condition.
func validateBreakerPolicy(p BreakerPolicy) error {
	if !p.Enabled() {
		return nil
	}
	// Without an explicit strategy, only an error threshold can be inferred
	// unambiguously (as consecutive). Windowed-strategy knobs have no meaning
	// under that inference, and a general knob such as open-duration cannot
	// choose a strategy at all — name the missing choice instead.
	if p.BreakerStrategy == "" {
		if p.ErrorRateThreshold != 0 || p.SlowCallDurationThreshold != 0 ||
			p.SlowCallRateThreshold != 0 || p.MinRequests != 0 || p.BreakerWindow != 0 {
			return errutil.Explain(nil,
				"breaker-strategy is required when windowed-strategy knobs are set")
		}
		if p.ErrorThreshold == 0 {
			return errutil.Explain(nil,
				"breaker-strategy (or error-threshold) is required to enable the breaker")
		}
	}
	_, err := circuitbreaker.New(p.Spec())
	return err
}

// findRule answers the breaker half of the call plan ([Executor.planFor]). A
// service-scope rule WINS OUTRIGHT: interface-level rules of that service are
// not consulted. Without a service rule, the most specific real-action rule
// applies.
//
// The second result reports whether a rule matched. The state key is derived by
// the caller from the rule's selector ([ruleKey]); the document fingerprint
// tracks the rule's content, so a changed rule rebuilds the table rather than
// following an old key to different state.
func (s *BreakerSettings) findRule(op chain.Operation) (BreakerRule, bool) {
	for i := range s.Services {
		if s.Services[i].Service == op.Service {
			// The first group naming a label wins; a duplicate group is
			// order-shadowed, not merged.
			return s.Services[i].findRule(op)
		}
	}
	return BreakerRule{}, false
}

// findRule selects one rule of this service.
func (s *BreakerService) findRule(op chain.Operation) (BreakerRule, bool) {
	best := -1
	var bestMatch matcher.ActionMatch
	for i := range s.Rules {
		r := &s.Rules[i]
		if !r.ActionMatch.Matches(op.Request.Action) {
			continue
		}
		// A service-scope rule wins outright, so the first one seen ends the
		// search: no interface rule can beat it.
		if r.ActionMatch.IsServiceScope() {
			return s.Rules[i], true
		}
		if best < 0 || r.ActionMatch.BetterThan(bestMatch) {
			best, bestMatch = i, r.ActionMatch
		}
	}
	if best >= 0 {
		return s.Rules[best], true
	}
	return BreakerRule{}, false
}

// breakerGen fingerprints the breaker half of everything a label resolves:
// every rule naming it. It is the ONE capability that needs a content
// comparison — a push that changes nothing must not reset a breaker's live
// state. Rate counters survive pushes by construction and timeout/retry values
// are pure, so neither carries a generation.
//
// The state key is the rule's selector (see [ruleKey]), so a reordering does
// not move a breaker; only a change of a rule's content does.
func breakerGen(label string, s *BreakerSettings) uint64 {
	h := fnv.New64a()
	for i := range s.Services {
		g := &s.Services[i]
		if g.Service != label {
			continue
		}
		for j := range g.Rules {
			r := &g.Rules[j]
			fingerprint(h, r.ActionMatch.Exact)
			fingerprint(h, r.ActionMatch.Prefix)
			fingerprintBreaker(h, r.BreakerPolicy)
		}
		break // first group naming the label wins, as in findRule
	}
	return h.Sum64()
}

// breakerFingerprint is the breaker half, in one value so it can be
// fingerprinted without pulling in unrelated knobs.
type breakerFingerprint struct {
	Off         bool
	Threshold   int
	Open        time.Duration
	Strategy    circuitbreaker.Strategy
	Rate        float64
	MinRequests int
	Window      time.Duration
	Slow        time.Duration
	SlowRate    float64
	HalfOpen    int
	HalfTimeout time.Duration
}

func fingerprintBreaker(h hash.Hash64, p BreakerPolicy) {
	fingerprint(h, fmt.Sprintf("%+v", breakerFingerprint{
		Off:         p.BreakerDisabled,
		Threshold:   p.ErrorThreshold,
		Open:        p.OpenDuration,
		Strategy:    p.BreakerStrategy,
		Rate:        p.ErrorRateThreshold,
		MinRequests: p.MinRequests,
		Window:      p.BreakerWindow,
		Slow:        p.SlowCallDurationThreshold,
		SlowRate:    p.SlowCallRateThreshold,
		HalfOpen:    p.HalfOpenRequests,
		HalfTimeout: p.HalfOpenTimeout,
	}))
}

// fingerprint writes one field, delimited so that ("ab","c") and ("a","bc")
// cannot collide.
func fingerprint(h hash.Hash64, s string) {
	h.Write([]byte(s))
	h.Write([]byte{0})
}
