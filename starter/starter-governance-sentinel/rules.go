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

// rules.go holds the one translation from the neutral resilience knobs onto
// sentinel-golang's rule shapes. Both engines (executor.go outbound,
// server_executor.go inbound) map through these helpers so the two directions
// cannot drift: a knob means the same sentinel field whichever side expressed
// it.
//
// Knobs a driver may legitimately NOT map are not faked here — the tables in
// the READMEs name them (Burst and Algorithm on flow; BulkheadQueueDuration on
// isolation; sentinel v1.0.4 has no counterpart for any of them).

package StarterGovernanceSentinel

import (
	"time"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/isolation"

	"go-spring.org/cloud/resilience"
)

// flowKnobs is the direction-neutral view of the rate-limit stage a policy
// expresses. Both policies assemble one; flowRule is the only consumer.
type flowKnobs struct {
	rateLimit  float64       // per second
	window     time.Duration // stat interval; 0 = one second
	maxWait    time.Duration // queueing budget; 0 = reject immediately
	warmUp     time.Duration // cold-start ramp; 0 = none
	coldFactor int           // 0 = sentinel default (3)
}

// breakerKnobs is the direction-neutral view of the circuit-breaker stage.
// strategy arrives already resolved (see [resilience.ClientPolicy.ResolvedBreakerStrategy]).
type breakerKnobs struct {
	strategy       resilience.BreakerStrategy
	errorThreshold int           // consecutive / error-count count
	rateThreshold  float64       // error-rate ratio
	slowThreshold  time.Duration // a call at/above this is slow
	slowRate       float64       // slow-call ratio
	minRequests    int
	window         time.Duration // stat interval; 0 = one second
	open           time.Duration // how long the circuit stays open; 0 = 5s
	probeN         int           // half-open trials; 0 and 1 both mean one
}

// flowRule translates the rate-limit knobs into one sentinel flow rule for
// service.
//
// UNIT NORMALIZATION is load-bearing: our RateLimit is per second, sentinel's
// Threshold is per StatIntervalInMs — the two agree at the default 1s window
// and diverge silently everywhere else, so the threshold is scaled here once,
// for both engines (mirroring the bundled driver's window limit in
// ratelimit.go's newRateState).
//
// Warm-up wins over queueing: sentinel pairs WarmUp only with Reject, so a
// policy setting both RateLimitMaxWait and RateLimitWarmUpDuration gets warm-up
// with immediate rejection — the ramp bounds admission better than the queue
// would, and the README says so.
func flowRule(service string, k flowKnobs) *flow.Rule {
	statMs := k.window.Milliseconds()
	if statMs <= 0 {
		statMs = 1000
	}
	rule := &flow.Rule{
		Resource:               service,
		TokenCalculateStrategy: flow.Direct,
		ControlBehavior:        flow.Reject,
		Threshold:              k.rateLimit * float64(statMs) / 1000.0,
		StatIntervalInMs:       uint32(statMs),
	}
	if k.maxWait > 0 {
		rule.ControlBehavior = flow.Throttling
		rule.MaxQueueingTimeMs = uint32(k.maxWait.Milliseconds())
	}
	if k.warmUp > 0 {
		rule.TokenCalculateStrategy = flow.WarmUp
		rule.ControlBehavior = flow.Reject // WarmUp pairs with Reject only
		rule.MaxQueueingTimeMs = 0
		rule.WarmUpPeriodSec = max(uint32(k.warmUp.Seconds()), 1)
		rule.WarmUpColdFactor = uint32(k.coldFactor)
		if rule.WarmUpColdFactor == 0 {
			rule.WarmUpColdFactor = 3
		}
	}
	return rule
}

// breakerRule translates the breaker knobs into one sentinel circuit-breaker
// rule for service. sentinel's strategies line up with ours one-to-one except
// consecutive: sentinel has no consecutive-run breaker (its ErrorCount counts
// over the stat window), so consecutive maps onto ErrorCount as a documented
// approximation — an exact windowed mapping is what [resilience.BreakerErrorCount]
// is for.
func breakerRule(service string, k breakerKnobs) *circuitbreaker.Rule {
	openMs := k.open.Milliseconds()
	if openMs <= 0 {
		openMs = 5000
	}
	winMs := k.window.Milliseconds()
	if winMs <= 0 {
		winMs = 1000
	}
	probe := uint64(k.probeN)
	if probe < 1 {
		probe = 1
	}
	rule := &circuitbreaker.Rule{
		Resource:       service,
		RetryTimeoutMs: uint32(openMs),
		StatIntervalMs: uint32(winMs),
		ProbeNum:       probe,
	}
	switch k.strategy {
	case resilience.BreakerErrorRate:
		rule.Strategy = circuitbreaker.ErrorRatio
		rule.MinRequestAmount = uint64(max(k.minRequests, 1))
		rule.Threshold = k.rateThreshold
	case resilience.BreakerSlowCall:
		rule.Strategy = circuitbreaker.SlowRequestRatio
		rule.MinRequestAmount = uint64(max(k.minRequests, 1))
		rule.MaxAllowedRtMs = uint64(k.slowThreshold.Milliseconds())
		rule.Threshold = k.slowRate
	case resilience.BreakerErrorCount:
		rule.Strategy = circuitbreaker.ErrorCount
		rule.MinRequestAmount = uint64(max(k.minRequests, 1))
		rule.Threshold = float64(k.errorThreshold)
	default: // BreakerConsecutive: sentinel approximation, see the doc comment
		rule.Strategy = circuitbreaker.ErrorCount
		rule.MinRequestAmount = 1
		rule.Threshold = float64(k.errorThreshold)
	}
	return rule
}

// isolationRule translates the bulkhead knob into one sentinel isolation rule.
// BulkheadQueueDuration has no sentinel v1.0.4 counterpart (isolation rejects,
// it does not queue) and is deliberately not represented.
func isolationRule(resource string, maxConcurrent int) *isolation.Rule {
	return &isolation.Rule{
		Resource:   resource,
		MetricType: isolation.Concurrency,
		Threshold:  uint32(maxConcurrent),
	}
}
