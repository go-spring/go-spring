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
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/stdlib/testing/assert"
)

// TestPolicyIsZeroCoversNewKnobs pins that IsZero sees the newer knobs: a
// policy setting ONLY one of them is no longer "no protection".
func TestPolicyIsZeroCoversNewKnobs(t *testing.T) {
	assert.That(t, (RateLimitPolicy{RateLimitWarmUpDuration: time.Second}).IsZero()).False()
	assert.That(t, (RateLimitPolicy{WarmUpColdFactor: 3}).IsZero()).False()
	assert.That(t, (BreakerPolicy{BreakerStrategy: circuitbreaker.StrategySlowCall}).IsZero()).False()
	assert.That(t, (BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyErrorCount}).IsZero()).False()
	assert.That(t, (BreakerPolicy{BreakerDisabled: true}).IsZero()).False()
	assert.That(t, (Limits{Timeout: time.Second}).IsZero()).False()
}

// TestValidateBreakerEnablement pins the contract that a configured breaker is
// either valid or rejected: disabling is an explicit, accepted choice, while an
// enabled breaker missing its trip condition is a configuration error.
func TestValidateBreakerEnablement(t *testing.T) {
	valid := func(p BreakerPolicy) {
		t.Helper()
		assert.Error(t, ValidateBreaker(BreakerSettings{Default: p})).Nil()
	}
	invalid := func(p BreakerPolicy) {
		t.Helper()
		assert.Error(t, ValidateBreaker(BreakerSettings{Default: p})).NotNil()
	}

	// Nothing configured: the breaker stays off.
	valid(BreakerPolicy{})
	// Disabled: knobs are retained without being validated.
	valid(BreakerPolicy{BreakerDisabled: true, BreakerStrategy: circuitbreaker.StrategyErrorRate, ErrorRateThreshold: 2})
	// A lone error threshold resolves to the consecutive default.
	valid(BreakerPolicy{ErrorThreshold: 5})
	// A complete breaker.
	valid(BreakerPolicy{
		BreakerStrategy:    circuitbreaker.StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        20,
		BreakerWindow:      time.Second,
	})

	// Selecting a strategy applies its mainstream defaults to a missing trip
	// condition, so these are complete configurations.
	valid(BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyConsecutive})
	valid(BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyErrorRate})
	valid(BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyErrorCount})

	// slow-call has no useful default pair: it stays required.
	invalid(BreakerPolicy{BreakerStrategy: circuitbreaker.StrategySlowCall})
	// A windowed-strategy knob without a strategy is ambiguous.
	invalid(BreakerPolicy{ErrorRateThreshold: 0.5})
	// A general knob alone cannot choose a strategy.
	invalid(BreakerPolicy{OpenDuration: time.Second})
}

// TestBreakerSpecDefaults pins the mainstream defaults the policy applies when
// a strategy is selected without spelling out its trip condition.
func TestBreakerSpecDefaults(t *testing.T) {
	if got := (BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyConsecutive}).Spec(); got.FailureThreshold != 5 || got.MinSamples != 5 {
		t.Fatalf("consecutive defaults = %+v, want threshold 5 and min samples 5", got)
	}
	if got := (BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyErrorCount}).Spec(); got.FailureThreshold != 5 {
		t.Fatalf("error-count default threshold = %d, want 5", got.FailureThreshold)
	}
	if got := (BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyErrorRate}).Spec(); got.FailureRate != 0.5 || got.MinSamples != 5 {
		t.Fatalf("error-rate defaults = %+v, want failure rate 0.5 and min samples 5", got)
	}

	// Explicit knobs always win over the defaults.
	spec := BreakerPolicy{BreakerStrategy: circuitbreaker.StrategyConsecutive, ErrorThreshold: 9, MinRequests: 20}.Spec()
	if spec.FailureThreshold != 9 || spec.MinSamples != 20 {
		t.Fatalf("explicit knobs did not win: %+v", spec)
	}
}
