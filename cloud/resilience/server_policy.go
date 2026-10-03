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
)

// ServerPolicy is a backend-neutral description of the protection wanted for INBOUND
// traffic — the server-side counterpart of [ClientPolicy] (see [Driver.NewServerExecutor]).
// It is what a server applies to the requests it receives: inbound control,
// not call protection.
//
// The two types are deliberately NOT one type with half the fields ignored. Both
// directions share the SAME mechanisms (a rate limiter and a circuit breaker do
// not know which way the bytes flow), but they share almost none of the
// DECISIONS, and the type is where the decisions live:
//
//   - Retry is meaningless inbound. A handler that already produced side effects
//     cannot be replayed, so a server must never retry. Today that rule is a
//     doc-comment convention ("Leave ClientPolicy.MaxRetries at 0") plus a Written()
//     guard; here it has no representation at all, so the executor an inbound
//     [Driver.NewServerExecutor] builds cannot be given a retry even by accident.
//   - Endpoint selection is meaningless inbound: there is no endpoint to choose.
//     It never lived in [ClientPolicy] either — it is [go-spring.org/cloud/loadbalance.
//     Selection], read by the Pool.
//   - [ClientPolicy.MaxDuration] caps the wall time of an operation ACROSS retries, so
//     with one attempt it has nothing left to bound; [ServerPolicy.AttemptTimeout]
//     (the handling budget) covers the whole call.
//
// Every field is zero-value-safe and keeps the sense it has in [ClientPolicy], so a
// value moves between the two directions without relearning its name. The value
// tags make ServerPolicy directly bindable as a governance rule-document half,
// exactly like [ClientPolicy].
type ServerPolicy struct {
	// RateLimit caps inbound throughput in requests per second. 0 disables rate
	// limiting.
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the maximum number of requests allowed to exceed RateLimit
	// momentarily. It defaults to a small multiple of RateLimit when unset; it
	// is ignored when RateLimit is 0.
	Burst int `value:"${burst:=0}"`

	// RateLimitMaxWait is how long an over-limit request may WAIT for a token
	// before being rejected with [chain.ErrRateLimited] (queueing / traffic shaping).
	// 0 (the default) keeps the reject-immediately behavior; a positive value
	// turns excess requests into bounded waits, smoothing bursts instead of
	// failing them. The wait also ends when the request's ctx is done. Ignored
	// when RateLimit is 0.
	RateLimitMaxWait time.Duration `value:"${rate-limit-max-wait:=0}"`

	// Algorithm selects how the rate-limit stage counts: empty means
	// [TokenBucket], the other value is [SlidingWindow]. Ignored when RateLimit
	// is 0.
	Algorithm Algorithm `value:"${algorithm:=}"`

	// Window is the rolling interval [SlidingWindow] counts over; it defaults to
	// one second and is ignored by [TokenBucket]. Ignored when RateLimit is 0.
	Window time.Duration `value:"${window:=0}"`

	// ErrorThreshold is the failure count that trips the inbound circuit breaker.
	// Under [BreakerConsecutive] (the default) it counts failures in a row; under
	// [BreakerErrorRate] it is unused (see [ServerPolicy.ErrorRateThreshold]). 0
	// disables the consecutive strategy.
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the circuit stays open before a trial request is
	// allowed through (half-open). Ignored when no breaker strategy is active;
	// defaults to a few seconds when unset.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// BreakerStrategy selects how the breaker counts failures (consecutive vs.
	// error-rate). Empty means [BreakerConsecutive]. The breaker is active when
	// [ServerPolicy.ErrorThreshold] > 0 (consecutive) or
	// [ServerPolicy.ErrorRateThreshold] > 0 (error-rate).
	BreakerStrategy BreakerStrategy `value:"${breaker-strategy:=}"`

	// ErrorRateThreshold is the failure ratio in (0,1] that trips an
	// [BreakerErrorRate] breaker. 0 disables the rate strategy. Pair with
	// [ServerPolicy.MinRequests] and [ServerPolicy.BreakerWindow].
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}"`

	// MinRequests is the minimum sample size observed in [ServerPolicy.BreakerWindow]
	// before an [BreakerErrorRate] breaker may trip, so a 1/1 failure does not
	// open the circuit. It defaults to 1 when unset; ignored by the consecutive
	// strategy.
	MinRequests int `value:"${min-requests:=0}"`

	// BreakerWindow is the rolling interval the error-rate strategy counts over.
	// It defaults to one second when unset. The default consecutive breaker
	// ignores it (it counts an unbounded run); sentinel uses it as the stat
	// interval for both strategies.
	BreakerWindow time.Duration `value:"${breaker-window:=0}"`

	// SlowCallDurationThreshold is the handling time at or above which a request
	// counts as a SLOW call for the breaker: a route that never errors but
	// answers slowly is unhealthy too. Pair with
	// [ServerPolicy.SlowCallRateThreshold]; the slow-call ratio is counted over the
	// error-rate strategy's window and a slow-but-successful request counts
	// toward the error-rate numerator the same way a failure does. 0 disables
	// slow-call counting.
	SlowCallDurationThreshold time.Duration `value:"${slow-call-duration-threshold:=0}"`

	// SlowCallRateThreshold is the slow-call ratio in (0,1] that trips the
	// breaker, counted over [ServerPolicy.BreakerWindow] with
	// [ServerPolicy.MinRequests] samples. Setting it (with
	// SlowCallDurationThreshold) activates the error-rate strategy even when
	// ErrorRateThreshold is 0.
	SlowCallRateThreshold float64 `value:"${slow-call-rate-threshold:=0}"`

	// HalfOpenRequests is how many trial requests the breaker admits in the
	// half-open state before deciding: all N succeeding closes the circuit, the
	// first failure re-opens it. 0 and 1 both mean the single trial. A larger N
	// is robust against a flapping first probe. Ignored when no breaker strategy
	// is active.
	HalfOpenRequests int `value:"${half-open-requests:=0}"`

	// MaxConcurrent caps the number of requests a server handles at the same
	// time (the bulkhead / isolation stage). Excess requests are rejected with
	// [chain.ErrBulkheadFull] rather than queued, so a slow handler cannot exhaust the
	// server's goroutines or connections. 0 disables the bulkhead.
	MaxConcurrent int `value:"${max-concurrent:=0}"`

	// AttemptTimeout is the HANDLING budget: it bounds how long one inbound call
	// may take, via a derived context. Inbound has no attempts to separate, so
	// this is the whole call's budget. 0 means no budget is imposed by the
	// executor (the caller's own deadline still applies).
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`
}

// IsZero reports whether a configures no protection — every stage disabled, so
// an executor built from it would be a transparent pass-through. It mirrors
// [ClientPolicy.IsZero], for readers who want intent rather than bits.
func (p ServerPolicy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 && p.RateLimitMaxWait == 0 &&
		p.Algorithm == "" && p.Window == 0 &&
		p.ErrorThreshold == 0 && p.OpenDuration == 0 &&
		p.BreakerStrategy == "" && p.ErrorRateThreshold == 0 &&
		p.MinRequests == 0 && p.BreakerWindow == 0 &&
		p.SlowCallDurationThreshold == 0 && p.SlowCallRateThreshold == 0 &&
		p.HalfOpenRequests == 0 &&
		p.MaxConcurrent == 0 && p.AttemptTimeout == 0
}

// RateSpec returns the rate-limit stage's view of p, exactly as
// [ClientPolicy.RateSpec] does for the outbound side: the stage and any [Counters]
// store read the same vocabulary from either direction, and the two agree on every
// shared knob's meaning.
func (p ServerPolicy) RateSpec() RateSpec {
	return RateSpec{
		RateLimit: p.RateLimit,
		Burst:     p.Burst,
		Algorithm: p.Algorithm,
		Window:    p.Window,
		MaxWait:   p.RateLimitMaxWait,
	}
}

// breakerSpec returns the circuit breaker's thresholds for p, in the breaker's own
// vocabulary (see [breakerSpec]).
func (p ServerPolicy) breakerSpec() breakerSpec {
	return breakerSpec{
		strategy:      p.ResolvedBreakerStrategy(),
		threshold:     p.ErrorThreshold,
		rateThreshold: p.ErrorRateThreshold,
		minRequests:   p.MinRequests,
		window:        p.BreakerWindow,
		slowThreshold: p.SlowCallDurationThreshold,
		slowRate:      p.SlowCallRateThreshold,
		halfOpenN:     p.HalfOpenRequests,
	}
}

// ResolvedBreakerStrategy returns the strategy a driver should apply, by the
// same resolution as [ClientPolicy.ResolvedBreakerStrategy] — the two directions must
// not disagree on what an unset strategy means.
func (p ServerPolicy) ResolvedBreakerStrategy() BreakerStrategy {
	return resolveBreakerStrategy(p.BreakerStrategy, p.SlowCallDurationThreshold, p.SlowCallRateThreshold)
}

// BreakerActive reports whether any breaker strategy is configured.
func (p ServerPolicy) BreakerActive() bool {
	return p.breakerSpec().active()
}
