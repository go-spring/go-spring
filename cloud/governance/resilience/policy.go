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

// ClientPolicy is a backend-neutral description of the protection wanted for a set of
// operations. Each [Driver] maps these knobs onto its own primitives (the
// default driver reads them directly; sentinel-golang translates them into its
// flow/circuit-breaker rules). A zero ClientPolicy protects nothing — every stage is
// opt-in, so an unset ClientPolicy makes [ClientExecutor.Execute] a transparent pass-through.
//
// Every field below is zero-value-safe: a zero field keeps the historical
// behavior, so existing ClientPolicy literals and configs are unaffected by the newer
// knobs (backoff, retry classification, total budget, breaker strategy). The
// value tags make ClientPolicy directly bindable as a governance rule-document half —
// there is no separate binding twin: what a document can express IS this type.
// Error classification is the [Retryable] interface (an error opts itself in or
// out), never a ClientPolicy field, because a document cannot express a function.
type ClientPolicy struct {
	// RateLimit caps sustained throughput in operations per second. 0 disables
	// rate limiting.
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the maximum number of operations allowed to exceed RateLimit
	// momentarily. It defaults to a small multiple of RateLimit when unset; it
	// is ignored when RateLimit is 0.
	Burst int `value:"${burst:=0}"`

	// RateLimitMaxWait is how long an over-limit call may WAIT for a token
	// before being rejected with [ErrRateLimited] (queueing / traffic shaping).
	// 0 (the default) keeps the historical reject-immediately behavior; a
	// positive value turns excess calls into bounded waits, smoothing bursts
	// instead of failing them. The wait also ends when the caller's ctx is
	// done. Ignored when RateLimit is 0.
	RateLimitMaxWait time.Duration `value:"${rate-limit-max-wait:=0}"`

	// Algorithm selects how the rate-limit stage counts: empty means
	// [TokenBucket], the other value is [SlidingWindow]. Ignored when RateLimit
	// is 0.
	Algorithm Algorithm `value:"${algorithm:=}"`

	// Window is the rolling interval [SlidingWindow] counts over; it defaults to
	// one second and is ignored by [TokenBucket]. Ignored when RateLimit is 0.
	Window time.Duration `value:"${window:=0}"`

	// ErrorThreshold is the failure count that trips the circuit breaker. Under
	// [BreakerConsecutive] (the default) it counts failures in a row; under
	// [BreakerErrorRate] it is unused (see [ClientPolicy.ErrorRateThreshold]). 0
	// disables the consecutive strategy.
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the circuit stays open before a trial request is
	// allowed through (half-open). Ignored when no breaker strategy is active;
	// defaults to a few seconds when unset.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// BreakerStrategy selects how the breaker counts failures (consecutive vs.
	// error-rate). Empty means [BreakerConsecutive]. The breaker is active when
	// [ClientPolicy.ErrorThreshold] > 0 (consecutive) or [ClientPolicy.ErrorRateThreshold]
	// > 0 (error-rate).
	BreakerStrategy BreakerStrategy `value:"${breaker-strategy:=}"`

	// ErrorRateThreshold is the failure ratio in (0,1] that trips an
	// [BreakerErrorRate] breaker. 0 disables the rate strategy. Pair with
	// [ClientPolicy.MinRequests] and [ClientPolicy.BreakerWindow].
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}"`

	// MinRequests is the minimum sample size observed in [ClientPolicy.BreakerWindow]
	// before an [BreakerErrorRate] breaker may trip, so a 1/1 failure does not
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
	// answers slowly is unhealthy too. Pair with [ClientPolicy.SlowCallRateThreshold];
	// the slow-call ratio is counted over the error-rate strategy's window and
	// a slow-but-successful attempt counts toward the error-rate numerator the
	// same way a failure does. 0 disables slow-call counting.
	SlowCallDurationThreshold time.Duration `value:"${slow-call-duration-threshold:=0}"`

	// SlowCallRateThreshold is the slow-call ratio in (0,1] that trips the
	// breaker, counted over [ClientPolicy.BreakerWindow] with [ClientPolicy.MinRequests]
	// samples. Setting it (with SlowCallDurationThreshold) activates the
	// error-rate strategy even when ErrorRateThreshold is 0.
	SlowCallRateThreshold float64 `value:"${slow-call-rate-threshold:=0}"`

	// HalfOpenRequests is how many trial requests the breaker admits in the
	// half-open state before deciding: all N succeeding closes the circuit,
	// the first failure re-opens it. 0 and 1 both mean the historical single
	// trial. A larger N is robust against a flapping first probe. Ignored when
	// no breaker strategy is active.
	HalfOpenRequests int `value:"${half-open-requests:=0}"`

	// MaxConcurrent caps the number of operations allowed to run against a
	// service at the same time (the bulkhead / isolation stage). Excess calls
	// are rejected with [ErrBulkheadFull] rather than queued, so a slow
	// downstream cannot exhaust the caller's goroutines or connections. 0
	// disables the bulkhead.
	MaxConcurrent int `value:"${max-concurrent:=0}"`

	// MaxRetries is the number of extra attempts after the first failure. 0
	// means a single attempt with no retry. Retries respect the circuit breaker,
	// rate limiter and bulkhead, and are paced by the backoff fields below.
	MaxRetries int `value:"${max-retries:=0}"`

	// RetryBudget caps the number of retry attempts in flight across ALL
	// services served by one executor at the same time. It is the process-level
	// guard against retry amplification: without it, MaxRetries is a per-call
	// promise, and a downstream blip turns every concurrent caller's retry into
	// extra load precisely when the downstream can least afford it. An over-
	// budget retry is rejected with [ErrRetryBudgetExceeded] (the first attempt
	// always runs; only retries take budget). 0 disables the cap.
	RetryBudget int `value:"${retry-budget:=0}"`

	// InitialInterval is the backoff before the first retry. 0 disables backoff
	// entirely (retries run back-to-back, the historical behavior). Pair with
	// [ClientPolicy.Multiplier], [ClientPolicy.MaxInterval] and [ClientPolicy.RandomizationFactor].
	InitialInterval time.Duration `value:"${initial-interval:=0}"`

	// Multiplier is the exponential growth factor applied to InitialInterval
	// between successive retries. 0 and 1 both mean a constant interval.
	Multiplier float64 `value:"${multiplier:=0}"`

	// MaxInterval caps the grown backoff so it does not expand unbounded. 0
	// means no cap.
	MaxInterval time.Duration `value:"${max-interval:=0}"`

	// RandomizationFactor is the jitter fraction in [0,1) applied to each
	// computed interval as interval*(1 ± factor), decorrelating clients that
	// would otherwise retry in lockstep. 0 means no jitter.
	RandomizationFactor float64 `value:"${randomization-factor:=0}"`

	// Timeout bounds each individual attempt via a derived context. 0 means no
	// per-attempt timeout is imposed by the executor.
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`

	// MaxDuration caps the wall time of the whole [ClientExecutor.Execute] across all
	// the retries. When both [ClientPolicy.Timeout] and MaxDuration are set, the effective
	// per-attempt budget is the smaller of Timeout and the remaining MaxDuration.
	// 0 means no total cap (beyond the caller's own context deadline).
	MaxDuration time.Duration `value:"${max-duration:=0}"`
}

// IsZero reports whether p configures no protection — every stage disabled, so
// [ClientExecutor.Execute] would be a transparent pass-through. It replaces struct
// equality (`p == (ClientPolicy{})`) for readers who want intent, not bits.
//
// Endpoint selection is not consulted either, for the same shape of reason: it
// is not part of this type at all. A service's selection knobs live in
// [go-spring.org/cloud/loadbalance.Selection] and are read by the Pool, never by
// [ClientExecutor.Execute].
func (p ClientPolicy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 && p.RateLimitMaxWait == 0 &&
		p.Algorithm == "" && p.Window == 0 &&
		p.ErrorThreshold == 0 && p.OpenDuration == 0 &&
		p.BreakerStrategy == "" && p.ErrorRateThreshold == 0 &&
		p.MinRequests == 0 && p.BreakerWindow == 0 &&
		p.SlowCallDurationThreshold == 0 && p.SlowCallRateThreshold == 0 &&
		p.HalfOpenRequests == 0 &&
		p.MaxConcurrent == 0 && p.MaxRetries == 0 && p.RetryBudget == 0 &&
		p.InitialInterval == 0 && p.Multiplier == 0 &&
		p.MaxInterval == 0 && p.RandomizationFactor == 0 &&
		p.AttemptTimeout == 0 && p.MaxDuration == 0
}

// ResolvedBreakerStrategy returns the strategy a driver should apply. An
// explicit [ClientPolicy.BreakerStrategy] of error-rate wins; a slow-call pair
// ([ClientPolicy.SlowCallDurationThreshold] + [ClientPolicy.SlowCallRateThreshold]) without
// an explicit strategy also selects error-rate (slow calls are counted by the
// rate window); otherwise the default is [BreakerConsecutive]. It is shared by
// both drivers so they always agree on the resolution.
func (p ClientPolicy) ResolvedBreakerStrategy() BreakerStrategy {
	if p.BreakerStrategy == BreakerErrorRate || p.slowCallActive() {
		return BreakerErrorRate
	}
	return BreakerConsecutive
}

// slowCallActive reports whether the slow-call pair alone activates rate-based
// counting.
func (p ClientPolicy) slowCallActive() bool {
	return p.SlowCallDurationThreshold > 0 && p.SlowCallRateThreshold > 0
}

// BreakerActive reports whether any breaker strategy is configured (a consecutive
// threshold, an error-rate threshold, or a slow-call pair is set). Both drivers
// consult it so they build a breaker under exactly the same condition.
func (p ClientPolicy) BreakerActive() bool {
	return p.ErrorThreshold > 0 || p.ErrorRateThreshold > 0 || p.slowCallActive()
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

// ServiceLabel joins prefix with the first non-empty name in names, falling
// back to prefix alone when none is set. Client starters call it to standardize
// the resilience service key the rate-limit counters and the breaker state are
// scoped by:
//
//	resilience.ServiceLabel("redis", c.ServiceName, c.MasterName, c.Addr)
//
// The client-specific fallback chain (only the client knows its own address
// fields) stays in the client; this helper just standardizes the join.
func ServiceLabel(prefix string, names ...string) string {
	for _, n := range names {
		if n != "" {
			return prefix + ":" + n
		}
	}
	return prefix
}
