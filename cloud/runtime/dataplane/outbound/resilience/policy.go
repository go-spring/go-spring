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
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/circuitbreaker"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
)

// Policy is a backend-neutral description of the protection wanted for a set of
// operations. Each [Driver] maps these knobs onto its own primitives (the
// default driver reads them directly; sentinel-golang translates them into its
// flow/circuit-breaker rules). A zero Policy protects nothing — every stage is
// opt-in, so an unset Policy makes [chain.Executor.Execute] a transparent pass-through.
//
// Every field below is zero-value-safe: a zero field keeps the historical
// behavior, so existing Policy literals and configs are unaffected by the newer
// knobs (backoff, retry classification, total budget, breaker strategy). The
// value tags make Policy directly bindable as a governance rule-document half —
// there is no separate binding twin: what a document can express IS this type.
// Error classification is the [Retryable] interface (an error opts itself in or
// out), never a Policy field, because a document cannot express a function.
type Policy struct {
	// RateLimit caps sustained throughput in operations per second. 0 disables
	// rate limiting.
	RateLimit float64 `value:"${rate-limit:=0}" expr:"$ >= 0"`

	// Burst is the maximum number of operations allowed to exceed RateLimit
	// momentarily. It defaults to a small multiple of RateLimit when unset; it
	// is ignored when RateLimit is 0.
	Burst int `value:"${burst:=0}" expr:"$ >= 0"`

	// RateLimitMaxWait is how long an over-limit call may WAIT for a token
	// before being rejected with [chain.ErrRateLimited] (queueing / traffic shaping).
	// 0 (the default) keeps the historical reject-immediately behavior; a
	// positive value turns excess calls into bounded waits, smoothing bursts
	// instead of failing them. The wait also ends when the caller's ctx is
	// done. Ignored when RateLimit is 0.
	RateLimitMaxWait time.Duration `value:"${rate-limit-max-wait:=0}" expr:"int($) >= 0"`

	// Algorithm selects how the rate-limit stage counts: empty means
	// [ratelimit.AlgorithmTokenBucket], the other value is
	// [ratelimit.AlgorithmSlidingWindow]. Ignored when RateLimit is 0.
	Algorithm ratelimit.Algorithm `value:"${algorithm:=}" expr:"string($) == '' || string($) in ['token-bucket', 'sliding-window']"`

	// Window is the rolling interval [ratelimit.AlgorithmSlidingWindow] counts
	// over; it defaults to one second and is ignored by
	// [ratelimit.AlgorithmTokenBucket]. Ignored when RateLimit is 0.
	Window time.Duration `value:"${window:=0}" expr:"int($) >= 0"`

	// RateLimitWarmUpDuration ramps the rate limit up from a cold start over
	// this duration, instead of admitting the full RateLimit the instant the
	// process starts — a freshly restarted fleet must not hit a cold downstream
	// at full quota. The default driver approximates it with a cold-start token
	// bucket (capacity grows linearly to full over the duration); sentinel maps
	// it onto flow-rule WarmUp. It is the neutral counterpart of OpenSergo's
	// warm-up cold-start. 0 disables warm-up. Ignored when RateLimit is 0.
	RateLimitWarmUpDuration time.Duration `value:"${rate-limit-warm-up-duration:=0}" expr:"int($) >= 0"`

	// WarmUpColdFactor shapes how conservative the ramp-in is: the admission
	// curve starts at roughly RateLimit/coldFactor and rises to full RateLimit
	// by the end of [Policy.RateLimitWarmUpDuration]. 0 means the backend's
	// default (3). Ignored when RateLimitWarmUpDuration is 0.
	WarmUpColdFactor int `value:"${warm-up-cold-factor:=0}" expr:"$ >= 0"`

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
	// The breaker is active when [Policy.BreakerDisabled] is false and at least
	// one breaker knob is configured; selecting a strategy applies its
	// mainstream defaults to the trip condition (see [Policy.BreakerSpec]).
	BreakerStrategy circuitbreaker.Strategy `value:"${breaker-strategy:=}" expr:"string($) == '' || string($) in ['consecutive', 'error-count', 'error-rate', 'slow-call']"`

	// ErrorRateThreshold is the failure ratio in [0,1] that trips an
	// [circuitbreaker.StrategyErrorRate] breaker. When error-rate is selected
	// and this is 0, the policy default is 0.5 (50%, the Hystrix/resilience4j
	// default). Pair it with [Policy.MinRequests] and [Policy.BreakerWindow].
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}" expr:"$ >= 0 && $ <= 1"`

	// MinRequests is the minimum sample size observed in [Policy.BreakerWindow]
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
	// answers slowly is unhealthy too. Pair it with [Policy.SlowCallRateThreshold];
	// the slow ratio is counted over the same window as the error rate but fed
	// by its own numerator, so slowness never inflates the failure rate.
	// The pair is required when the selected strategy is slow-call, and an
	// error-rate breaker may use it instead of (or in addition to)
	// [Policy.ErrorRateThreshold].
	SlowCallDurationThreshold time.Duration `value:"${slow-call-duration-threshold:=0}" expr:"int($) >= 0"`

	// SlowCallRateThreshold is the slow-call ratio in (0,1] that trips the
	// breaker, counted over [Policy.BreakerWindow] with [Policy.MinRequests]
	// samples. Under [circuitbreaker.StrategyErrorRate] it trips on its own even
	// when ErrorRateThreshold is 0; under [circuitbreaker.StrategySlowCall] it is
	// the only signal. Must be paired with [Policy.SlowCallDurationThreshold].
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

	// Retry is the service-level retry declaration a governance center MAY push
	// for this label. By default a center pushes nothing (the zero node), and the
	// retry a call runs under is the one the caller declares on
	// [chain.Operation.Retry] — the caller is the only side that knows whether
	// repeating the call repeats a side effect. When a center DOES push a node
	// that declares a schedule, it wins WHOLE over the call's node, because a
	// pushed value is how an operator hot-fixes a live incident; the
	// NonIdempotent flag is preserved from both regardless (see
	// [EffectiveRetry], the one place the two are merged).
	//
	// RetryBudget is read from HERE only, at construction time (see
	// [chain.RetryPolicy.RetryBudget]); a per-call node's budget is ignored.
	Retry chain.RetryPolicy `value:"${retry:=}"`

	// Timeout bounds the WALL TIME of the whole [chain.Executor.Execute] — retries
	// and backoff included — as one context deadline applied at the entry and
	// covering every stage beneath it.
	//
	// It is deliberately ONE bound rather than a per-attempt one plus a total: a
	// single deadline is what an operator reasons about ("this call must not
	// exceed N"), and it is what the governance layers of every mainstream stack
	// express. Its consequence is that rounds share the budget — a first attempt
	// that spends it all leaves nothing for the retries — which is the same
	// contract Istio's route timeout and gRPC's deadline carry.
	//
	// 0 means no bound of its own, beyond whatever the caller's context carries.
	Timeout time.Duration `value:"${timeout:=0}" expr:"int($) >= 0"`
}

// IsZero reports whether p configures no protection — every stage disabled, so
// [chain.Executor.Execute] would be a transparent pass-through. It replaces struct
// equality (`p == (Policy{})`) for readers who want intent, not bits.
//
// Endpoint selection is not consulted either, for the same reason: it is not
// part of this type at all. A service's selection knobs live in
// [go-spring.org/cloud/runtime/dataplane/kernel/loadbalance.Selection] and are read by the Pool, never by
// [chain.Executor.Execute].
func (p Policy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 && p.RateLimitMaxWait == 0 &&
		p.Algorithm == "" && p.Window == 0 &&
		p.RateLimitWarmUpDuration == 0 && p.WarmUpColdFactor == 0 &&
		!p.BreakerDisabled && !p.hasBreakerConfig() &&
		p.Retry.IsZero() &&
		p.Timeout == 0
}

// BreakerEnabled reports whether the breaker stage is on: the operator has not
// disabled it and at least one breaker knob is configured. A false value is a
// valid OFF stage — the holder keeps a nil *Breaker — not a construction error.
func (p Policy) BreakerEnabled() bool {
	return !p.BreakerDisabled && p.hasBreakerConfig()
}

// hasBreakerConfig reports whether any breaker knob is configured. The switch
// itself is not a knob: "disabled" only means something when there is a
// configuration to disable. An empty policy therefore leaves the breaker off
// rather than enabling it with defaults nobody asked for.
func (p Policy) hasBreakerConfig() bool {
	return p.BreakerStrategy != "" || p.ErrorThreshold != 0 || p.OpenDuration != 0 ||
		p.ErrorRateThreshold != 0 || p.MinRequests != 0 || p.BreakerWindow != 0 ||
		p.SlowCallDurationThreshold != 0 || p.SlowCallRateThreshold != 0 ||
		p.HalfOpenRequests != 0 || p.HalfOpenTimeout != 0
}

// ratelimit.Spec returns the rate-limit stage's view of p: the knobs the stage and any
// [ratelimit.ClusterLimiter] store behind it read, in that stage's own vocabulary. A caller that
// charges a store directly — a gateway filter, say — builds the spec here rather
// than off the policy, so the one mapping from policy to store lives with the
// model.
func (p Policy) RateSpec() ratelimit.Spec {
	return ratelimit.Spec{
		RateLimit:  p.RateLimit,
		Burst:      p.Burst,
		Algorithm:  p.Algorithm,
		Window:     p.Window,
		MaxWait:    p.RateLimitMaxWait,
		WarmUp:     p.RateLimitWarmUpDuration,
		ColdFactor: p.WarmUpColdFactor,
	}
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

// BreakerSpec maps p onto the breaker's own vocabulary, applying the policy
// defaults. It is the one mapping validation and materialization use, so every
// driver sees the same resolved knobs. Callers gate on [Policy.BreakerEnabled]
// first; an OFF breaker is a nil *Breaker, not a Spec.
func (p Policy) BreakerSpec() circuitbreaker.Spec {
	spec := circuitbreaker.Spec{
		Strategy:         p.ResolvedBreakerStrategy(),
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

// ResolvedBreakerStrategy returns the strategy a driver should apply: an
// explicit [Policy.BreakerStrategy], or the [circuitbreaker.StrategyConsecutive]
// default when unset. The strategy decides which threshold fields count — a
// slow-call pair without an explicit error-rate/slow-call strategy is not
// inferred, it is a config the caller should fix.
func (p Policy) ResolvedBreakerStrategy() circuitbreaker.Strategy {
	if p.BreakerStrategy != "" {
		return p.BreakerStrategy
	}
	return circuitbreaker.StrategyConsecutive
}

// Retryable is implemented by an error to opt itself in or out of retry: the
// error's own type knows best whether another attempt is meaningful (a
// non-idempotent write that already failed server-side opts out; a transient
// reset opts in). This is how a gorm/redis adapter marks a
// non-idempotent-write error (Retryable() false) or a transient one (true)
// without teaching the executor about that client library.
type Retryable interface {
	Retryable() bool
}
