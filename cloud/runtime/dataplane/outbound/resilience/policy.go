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

	"go-spring.org/cloud/runtime/dataplane/kernel/breaker"
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
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the maximum number of operations allowed to exceed RateLimit
	// momentarily. It defaults to a small multiple of RateLimit when unset; it
	// is ignored when RateLimit is 0.
	Burst int `value:"${burst:=0}"`

	// RateLimitMaxWait is how long an over-limit call may WAIT for a token
	// before being rejected with [chain.ErrRateLimited] (queueing / traffic shaping).
	// 0 (the default) keeps the historical reject-immediately behavior; a
	// positive value turns excess calls into bounded waits, smoothing bursts
	// instead of failing them. The wait also ends when the caller's ctx is
	// done. Ignored when RateLimit is 0.
	RateLimitMaxWait time.Duration `value:"${rate-limit-max-wait:=0}"`

	// Algorithm selects how the rate-limit stage counts: empty means
	// [ratelimit.AlgorithmTokenBucket], the other value is
	// [ratelimit.AlgorithmSlidingWindow]. Ignored when RateLimit is 0.
	Algorithm ratelimit.Algorithm `value:"${algorithm:=}"`

	// Window is the rolling interval [ratelimit.AlgorithmSlidingWindow] counts
	// over; it defaults to one second and is ignored by
	// [ratelimit.AlgorithmTokenBucket]. Ignored when RateLimit is 0.
	Window time.Duration `value:"${window:=0}"`

	// RateLimitWarmUpDuration ramps the rate limit up from a cold start over
	// this duration, instead of admitting the full RateLimit the instant the
	// process starts — a freshly restarted fleet must not hit a cold downstream
	// at full quota. The default driver approximates it with a cold-start token
	// bucket (capacity grows linearly to full over the duration); sentinel maps
	// it onto flow-rule WarmUp. It is the neutral counterpart of OpenSergo's
	// warm-up cold-start. 0 disables warm-up. Ignored when RateLimit is 0.
	RateLimitWarmUpDuration time.Duration `value:"${rate-limit-warm-up-duration:=0}"`

	// WarmUpColdFactor shapes how conservative the ramp-in is: the admission
	// curve starts at roughly RateLimit/coldFactor and rises to full RateLimit
	// by the end of [Policy.RateLimitWarmUpDuration]. 0 means the backend's
	// default (3). Ignored when RateLimitWarmUpDuration is 0.
	WarmUpColdFactor int `value:"${warm-up-cold-factor:=0}"`

	// ErrorThreshold is the failure count that trips the circuit breaker. Under
	// [breaker.Consecutive] (the default) it counts failures in a row; under
	// [breaker.ErrorRate] it is unused (see [Policy.ErrorRateThreshold]). 0
	// disables the consecutive strategy.
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the circuit stays open before a trial request is
	// allowed through (half-open). Ignored when no breaker strategy is active;
	// defaults to a few seconds when unset.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// BreakerStrategy selects how the breaker counts failures (consecutive vs.
	// error-rate). Empty means [breaker.Consecutive]. The breaker is active when
	// [Policy.ErrorThreshold] > 0 (consecutive) or [Policy.ErrorRateThreshold]
	// > 0 (error-rate).
	BreakerStrategy breaker.Strategy `value:"${breaker-strategy:=}"`

	// ErrorRateThreshold is the failure ratio in (0,1] that trips an
	// [breaker.ErrorRate] breaker. 0 disables the rate strategy. Pair with
	// [Policy.MinRequests] and [Policy.BreakerWindow].
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}"`

	// MinRequests is the minimum sample size observed in [Policy.BreakerWindow]
	// before an [breaker.ErrorRate] breaker may trip, so a 1/1 failure does not
	// open the circuit. It defaults to 1 when unset; ignored by the consecutive
	// strategy.
	MinRequests int `value:"${min-requests:=0}"`

	// BreakerWindow is the rolling interval the error-rate strategy counts
	// over. It defaults to one second when unset. The default consecutive
	// breaker ignores it (it counts an unbounded run); sentinel uses it as the
	// stat interval for both strategies.
	BreakerWindow time.Duration `value:"${breaker-window:=0}"`

	// SlowCallDurationThreshold is the latency at or above which an attempt
	// counts as a SLOW call for the breaker: a downstream that never errors but
	// answers slowly is unhealthy too. Pair with [Policy.SlowCallRateThreshold];
	// the slow-call ratio is counted over the error-rate strategy's window and
	// a slow-but-successful attempt counts toward the error-rate numerator the
	// same way a failure does. 0 disables slow-call counting.
	SlowCallDurationThreshold time.Duration `value:"${slow-call-duration-threshold:=0}"`

	// SlowCallRateThreshold is the slow-call ratio in (0,1] that trips the
	// breaker, counted over [Policy.BreakerWindow] with [Policy.MinRequests]
	// samples. Setting it (with SlowCallDurationThreshold) activates the
	// error-rate strategy even when ErrorRateThreshold is 0.
	SlowCallRateThreshold float64 `value:"${slow-call-rate-threshold:=0}"`

	// HalfOpenRequests is how many trial requests the breaker admits in the
	// half-open state before deciding: all N succeeding closes the circuit,
	// the first failure re-opens it. 0 and 1 both mean the historical single
	// trial. A larger N is robust against a flapping first probe. Ignored when
	// no breaker strategy is active.
	HalfOpenRequests int `value:"${half-open-requests:=0}"`

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
	Timeout time.Duration `value:"${timeout:=0}"`
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
		p.ErrorThreshold == 0 && p.OpenDuration == 0 &&
		p.BreakerStrategy == "" && p.ErrorRateThreshold == 0 &&
		p.MinRequests == 0 && p.BreakerWindow == 0 &&
		p.SlowCallDurationThreshold == 0 && p.SlowCallRateThreshold == 0 &&
		p.HalfOpenRequests == 0 &&
		p.Retry.IsZero() &&
		p.Timeout == 0
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

// breakerSpec returns the circuit breaker's thresholds for p, in the breaker's
// own vocabulary (see [breaker.Spec]).
func (p Policy) breakerSpec() breaker.Spec {
	return breaker.Spec{
		Strategy:      p.ResolvedBreakerStrategy(),
		Threshold:     p.ErrorThreshold,
		RateThreshold: p.ErrorRateThreshold,
		MinRequests:   p.MinRequests,
		Window:        p.BreakerWindow,
		SlowThreshold: p.SlowCallDurationThreshold,
		SlowRate:      p.SlowCallRateThreshold,
		HalfOpenN:     p.HalfOpenRequests,
	}
}

// ResolvedBreakerStrategy returns the strategy a driver should apply. An
// explicit [Policy.BreakerStrategy] of error-rate wins; a slow-call pair
// ([Policy.SlowCallDurationThreshold] + [Policy.SlowCallRateThreshold]) without
// an explicit strategy also selects error-rate (slow calls are counted by the
// rate window); otherwise the default is [breaker.Consecutive].
func (p Policy) ResolvedBreakerStrategy() breaker.Strategy {
	return breaker.ResolveStrategy(p.BreakerStrategy, p.SlowCallDurationThreshold, p.SlowCallRateThreshold)
}

// BreakerActive reports whether any breaker strategy is configured (a consecutive
// threshold, an error-rate threshold, or a slow-call pair is set).
func (p Policy) BreakerActive() bool {
	return p.breakerSpec().Active()
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
