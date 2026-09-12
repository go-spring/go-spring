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

// Policy is a backend-neutral description of the protection wanted for a set of
// operations. Each [Driver] maps these knobs onto its own primitives (the
// default driver reads them directly; sentinel-golang translates them into its
// flow/circuit-breaker rules). A zero Policy protects nothing — every stage is
// opt-in, so an unset Policy makes [Executor.Execute] a transparent pass-through.
//
// Every field below is zero-value-safe: a zero field keeps the historical
// behavior, so existing Policy literals and configs are unaffected by the newer
// knobs (backoff, retry classification, total budget, breaker strategy).
type Policy struct {
	// RateLimit caps sustained throughput in operations per second. 0 disables
	// rate limiting.
	RateLimit float64

	// Burst is the maximum number of operations allowed to exceed RateLimit
	// momentarily. It defaults to a small multiple of RateLimit when unset; it
	// is ignored when RateLimit is 0.
	Burst int

	// ErrorThreshold is the failure count that trips the circuit breaker. Under
	// [BreakerConsecutive] (the default) it counts failures in a row; under
	// [BreakerErrorRate] it is unused (see [Policy.ErrorRateThreshold]). 0
	// disables the consecutive strategy.
	ErrorThreshold int

	// OpenDuration is how long the circuit stays open before a trial request is
	// allowed through (half-open). Ignored when no breaker strategy is active;
	// defaults to a few seconds when unset.
	OpenDuration time.Duration

	// BreakerStrategy selects how the breaker counts failures (consecutive vs.
	// error-rate). Empty means [BreakerConsecutive]. The breaker is active when
	// [Policy.ErrorThreshold] > 0 (consecutive) or [Policy.ErrorRateThreshold]
	// > 0 (error-rate).
	BreakerStrategy BreakerStrategy

	// ErrorRateThreshold is the failure ratio in (0,1] that trips an
	// [BreakerErrorRate] breaker. 0 disables the rate strategy. Pair with
	// [Policy.MinRequests] and [Policy.BreakerWindow].
	ErrorRateThreshold float64

	// MinRequests is the minimum sample size observed in [Policy.BreakerWindow]
	// before an [BreakerErrorRate] breaker may trip, so a 1/1 failure does not
	// open the circuit. It defaults to 1 when unset; ignored by the consecutive
	// strategy.
	MinRequests int

	// BreakerWindow is the rolling interval the error-rate strategy counts
	// over. It defaults to one second when unset. The default consecutive
	// breaker ignores it (it counts an unbounded run); sentinel uses it as the
	// stat interval for both strategies.
	BreakerWindow time.Duration

	// MaxConcurrent caps the number of operations allowed to run against a
	// resource at the same time (the bulkhead / isolation stage). Excess calls
	// are rejected with [ErrBulkheadFull] rather than queued, so a slow
	// downstream cannot exhaust the caller's goroutines or connections. 0
	// disables the bulkhead.
	MaxConcurrent int

	// MaxRetries is the number of extra attempts after the first failure. 0
	// means a single attempt with no retry. Retries respect the circuit breaker,
	// rate limiter and bulkhead, and are paced by the backoff fields below.
	MaxRetries int

	// RetryPredicate reports whether err should trigger another attempt. A nil
	// predicate retries on every non-nil error (the historical behavior); supply
	// your own for protocol-specific classification (e.g. only retry idempotent
	// verbs). It is consulted after each attempt; a [Retryable] error overrides
	// it.
	RetryPredicate func(err error) bool

	// InitialInterval is the backoff before the first retry. 0 disables backoff
	// entirely (retries run back-to-back, the historical behavior). Pair with
	// [Policy.Multiplier], [Policy.MaxInterval] and [Policy.RandomizationFactor].
	InitialInterval time.Duration

	// Multiplier is the exponential growth factor applied to InitialInterval
	// between successive retries. 0 and 1 both mean a constant interval.
	Multiplier float64

	// MaxInterval caps the grown backoff so it does not expand unbounded. 0
	// means no cap.
	MaxInterval time.Duration

	// RandomizationFactor is the jitter fraction in [0,1) applied to each
	// computed interval as interval*(1 ± factor), decorrelating clients that
	// would otherwise retry in lockstep. 0 means no jitter.
	RandomizationFactor float64

	// Timeout bounds each individual attempt via a derived context. 0 means no
	// per-attempt timeout is imposed by the executor.
	Timeout time.Duration

	// MaxDuration caps the wall time of the whole [Executor.Execute] across all
	// the retries. When both [Policy.Timeout] and MaxDuration are set, the effective
	// per-attempt budget is the smaller of Timeout and the remaining MaxDuration.
	// 0 means no total cap (beyond the caller's own context deadline).
	MaxDuration time.Duration

	// --- load balancing ---
	//
	// Endpoint selection, not executor protection: these fields are read by the
	// load-balancing Pool ([go-spring.org/cloud/loadbalance]) rather than by
	// [Executor.Execute]. They live here because a resource's selection policy
	// is resolved from the same rule document as its protection policy — pass
	// the same label to [governance.PolicyFor] and get both.

	// Balancer names the load-balancing strategy to select endpoints with.
	// Empty means "keep the client's own default".
	Balancer string

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint from the candidate set. It is the endpoint-keyed counterpart of
	// [Policy.ErrorThreshold] and 0 disables it.
	OutlierThreshold int

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Policy.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration
}

// IsZero reports whether p configures no protection — every stage disabled, so
// [Executor.Execute] would be a transparent pass-through. It replaces struct
// equality (`p == (Policy{})`) which is no longer possible now that Policy
// carries a function field ([Policy.RetryPredicate]). Callers that previously
// wrote `p == (Policy{})` to mean "no policy configured" should use IsZero.
//
// RetryPredicate is intentionally not consulted: with MaxRetries == 0 no retry
// runs, so a bare predicate set on an otherwise-zero Policy protects nothing.
//
// The load-balancing fields (Balancer, OutlierThreshold, OutlierSuspendFor) are
// deliberately NOT consulted either: they are read by the Pool, not by
// [Executor.Execute], so a Policy carrying only those still makes Execute a
// transparent pass-through — which is exactly what this predicate claims.
func (p Policy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 &&
		p.ErrorThreshold == 0 && p.OpenDuration == 0 &&
		p.BreakerStrategy == "" && p.ErrorRateThreshold == 0 &&
		p.MinRequests == 0 && p.BreakerWindow == 0 &&
		p.MaxConcurrent == 0 && p.MaxRetries == 0 &&
		p.InitialInterval == 0 && p.Multiplier == 0 &&
		p.MaxInterval == 0 && p.RandomizationFactor == 0 &&
		p.Timeout == 0 && p.MaxDuration == 0
}

// ResolvedBreakerStrategy returns the strategy a driver should apply, defaulting
// to [BreakerConsecutive] when [Policy.BreakerStrategy] is empty. It is shared by
// both drivers so they always agree on the resolution.
func (p Policy) ResolvedBreakerStrategy() BreakerStrategy {
	if p.BreakerStrategy == BreakerErrorRate {
		return BreakerErrorRate
	}
	return BreakerConsecutive
}

// BreakerActive reports whether any breaker strategy is configured (a consecutive
// threshold or an error-rate threshold is set). Both drivers consult it so they
// build a breaker under exactly the same condition.
func (p Policy) BreakerActive() bool {
	return p.ErrorThreshold > 0 || p.ErrorRateThreshold > 0
}

// Retryable is implemented by an error to opt itself in or out of retry,
// overriding [Policy.RetryPredicate] when the error type is known to the caller
// but not to the executor. This is how a gorm/redis adapter marks a
// non-idempotent-write error (Retryable() false) or a transient one (true)
// without teaching the executor about that client library.
type Retryable interface {
	Retryable() bool
}
