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

package admission

import "time"

// Policy describes the protection wanted for one inbound server instance. It is
// the server-side half of the governance document and is deliberately different
// from the client's policy type: retry has no representation here, and there is
// no endpoint selection.
//
// Every field is zero-value-safe: an all-zero Policy configures nothing, and a
// Guard built from it admits every request with a no-op Done.
type Policy struct {
	// RateLimit caps inbound throughput in requests per second. 0 disables the
	// rate-limit stage.
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is how many requests may exceed RateLimit momentarily; ignored when
	// RateLimit is 0.
	Burst int `value:"${burst:=0}"`

	// RateLimitMaxWait is how long an over-limit request may wait for a token
	// before being refused. 0 refuses immediately. Ignored when RateLimit is 0.
	RateLimitMaxWait time.Duration `value:"${rate-limit-max-wait:=0}"`

	// Algorithm selects the counting scheme: empty means token bucket,
	// "sliding-window" the other. Ignored when RateLimit is 0.
	Algorithm string `value:"${algorithm:=}"`

	// Window is the rolling interval the sliding-window algorithm counts over;
	// defaults to one second. Ignored by the token bucket.
	Window time.Duration `value:"${window:=0}"`

	// RateLimitWarmUpDuration ramps the limit up from a cold start over this
	// duration. 0 disables warm-up.
	RateLimitWarmUpDuration time.Duration `value:"${rate-limit-warm-up-duration:=0}"`

	// WarmUpColdFactor shapes the ramp-in; 0 means the engine's default.
	WarmUpColdFactor int `value:"${warm-up-cold-factor:=0}"`

	// ErrorThreshold is the failure count that trips the breaker under the
	// consecutive strategy. 0 disables it.
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the circuit stays open before a trial is
	// admitted; the engine applies a small default when unset.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// BreakerStrategy selects how the breaker counts failures: "consecutive"
	// (default), "error-rate", "slow-call" or "error-count".
	BreakerStrategy string `value:"${breaker-strategy:=}"`

	// ErrorRateThreshold is the failure ratio in (0,1] that trips an error-rate
	// breaker. 0 disables the rate strategy.
	ErrorRateThreshold float64 `value:"${error-rate-threshold:=0}"`

	// MinRequests is the minimum window sample before an error-rate breaker may
	// trip; defaults to 1.
	MinRequests int `value:"${min-requests:=0}"`

	// BreakerWindow is the rolling interval the window-based strategies count
	// over; defaults to one second.
	BreakerWindow time.Duration `value:"${breaker-window:=0}"`

	// SlowCallDurationThreshold is the handling time at or above which a request
	// counts as slow. 0 disables slow-call counting.
	SlowCallDurationThreshold time.Duration `value:"${slow-call-duration-threshold:=0}"`

	// SlowCallRateThreshold is the slow-call ratio in (0,1] that trips the
	// breaker. Setting it (with SlowCallDurationThreshold) activates the
	// window-based strategy.
	SlowCallRateThreshold float64 `value:"${slow-call-rate-threshold:=0}"`

	// HalfOpenRequests is how many trial requests the breaker admits in the
	// half-open state; 0 and 1 both mean one.
	HalfOpenRequests int `value:"${half-open-requests:=0}"`

	// MaxConcurrent caps how many requests the instance handles at the same
	// time (the bulkhead). 0 disables it.
	MaxConcurrent int `value:"${max-concurrent:=0}"`

	// BulkheadQueueDuration is how long a request finding the bulkhead full may
	// wait for a slot before being refused. 0 refuses immediately.
	BulkheadQueueDuration time.Duration `value:"${bulkhead-queue-duration:=0}"`

	// Timeout is the handling budget: one context deadline bounding how long the
	// inbound call may take. 0 means the Guard imposes none.
	Timeout time.Duration `value:"${timeout:=0}"`

	// DegradeOnOpen reports a circuit-open refusal as [VerdictDegraded] instead
	// of [VerdictCircuitOpen]. The handler still does not run; the adapter is
	// expected to serve its own degraded response (a cached value, a static
	// fallback, a reduced answer). [Result.Err] still carries [ErrCircuitOpen],
	// so an adapter that has no degraded response can keep mapping it to 503.
	DegradeOnOpen bool `value:"${degrade-on-open:=false}"`
}

// IsZero reports whether p configures no protection at all.
func (p Policy) IsZero() bool {
	return p.RateLimit == 0 && p.Burst == 0 && p.RateLimitMaxWait == 0 &&
		p.Algorithm == "" && p.Window == 0 &&
		p.RateLimitWarmUpDuration == 0 && p.WarmUpColdFactor == 0 &&
		p.ErrorThreshold == 0 && p.OpenDuration == 0 &&
		p.BreakerStrategy == "" && p.ErrorRateThreshold == 0 &&
		p.MinRequests == 0 && p.BreakerWindow == 0 &&
		p.SlowCallDurationThreshold == 0 && p.SlowCallRateThreshold == 0 &&
		p.HalfOpenRequests == 0 &&
		p.MaxConcurrent == 0 && p.BulkheadQueueDuration == 0 &&
		p.Timeout == 0 && !p.DegradeOnOpen
}

// Settings is the inbound half of the governance document as admission receives it:
// one default policy plus per-instance overrides.
type Settings struct {
	// Default governs every instance no Rule matches.
	Default Policy

	// Rules are the per-instance overrides, matched by exact Service. An empty
	// Service matches nothing (use Default for the catch-all).
	Rules []Rule
}

// Rule is one per-instance override. A matched Rule's Policy REPLACES the
// default whole rather than merging field-wise.
type Rule struct {
	// Service is the instance label this Rule matches, exact-compare.
	Service string

	// Policy is what the matched instance runs under.
	Policy Policy
}
