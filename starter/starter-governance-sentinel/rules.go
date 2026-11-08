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

// rules.go holds the one translation from the neutral rate-limit knobs onto
// sentinel-golang's flow rule. It is the ONLY thing this module translates:
// timeout, retry, breaker and matching belong to the resilience engine, which
// this provider plugs into.
//
// Knobs a limiter legitimately cannot map are not faked here — the READMEs name
// them (Burst and the algorithm kind).

package StarterGovernanceSentinel

import (
	"time"

	"github.com/alibaba/sentinel-golang/core/flow"
)

// flowKnobs is the direction-neutral view of the rate-limit stage a policy
// expresses. flowRule is its only consumer.
type flowKnobs struct {
	rateLimit  float64       // per second
	window     time.Duration // stat interval; 0 = one second
	maxWait    time.Duration // queueing budget; 0 = reject immediately
	warmUp     time.Duration // cold-start ramp; 0 = none
	coldFactor int           // 0 = sentinel default (3)
}

// flowRule translates the rate-limit knobs into one sentinel flow rule for
// scope.
//
// UNIT NORMALIZATION is load-bearing: our RateLimit is per second, sentinel's
// Threshold is per StatIntervalInMs — the two agree at the default 1s window
// and diverge silently everywhere else, so the threshold is scaled here once.
//
// Warm-up wins over queueing: sentinel pairs WarmUp only with Reject, so a
// policy setting both MaxWait and WarmUp gets warm-up with immediate
// rejection — the ramp bounds admission better than the queue would.
func flowRule(scope string, k flowKnobs) *flow.Rule {
	statMs := k.window.Milliseconds()
	if statMs <= 0 {
		statMs = 1000
	}
	rule := &flow.Rule{
		Resource:               scope,
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
