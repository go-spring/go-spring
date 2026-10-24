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

	"go-spring.org/cloud/runtime/dataplane/kernel/circuitbreaker"
	"go-spring.org/stdlib/testing/assert"
)

// TestPolicyIsZeroCoversNewKnobs pins that IsZero sees the newer knobs: a
// policy setting ONLY one of them is no longer "no protection".
func TestPolicyIsZeroCoversNewKnobs(t *testing.T) {
	assert.That(t, (Policy{RateLimitWarmUpDuration: time.Second}).IsZero()).False()
	assert.That(t, (Policy{WarmUpColdFactor: 3}).IsZero()).False()
	assert.That(t, (Policy{BreakerStrategy: circuitbreaker.StrategySlowCall}).IsZero()).False()
	assert.That(t, (Policy{BreakerStrategy: circuitbreaker.StrategyErrorCount}).IsZero()).False()
	assert.That(t, (Policy{BreakerDisabled: true}).IsZero()).False()
}

// TestValidateBreakerEnablement pins the contract that a configured breaker is
// either valid or rejected: disabling is an explicit, accepted choice, while an
// enabled breaker missing its trip condition is a configuration error.
func TestValidateBreakerEnablement(t *testing.T) {
	valid := func(p Policy) {
		t.Helper()
		assert.Error(t, ValidateSettings(Settings{Default: p})).Nil()
	}
	invalid := func(p Policy) {
		t.Helper()
		assert.Error(t, ValidateSettings(Settings{Default: p})).NotNil()
	}

	// Nothing configured: the breaker stays off.
	valid(Policy{})
	// Disabled: knobs are retained without being validated.
	valid(Policy{BreakerDisabled: true, BreakerStrategy: circuitbreaker.StrategyErrorRate, ErrorRateThreshold: 2})
	// A lone error threshold resolves to the consecutive default.
	valid(Policy{ErrorThreshold: 5})
	// A complete breaker.
	valid(Policy{
		BreakerStrategy:    circuitbreaker.StrategyErrorRate,
		ErrorRateThreshold: 0.5,
		MinRequests:        20,
		BreakerWindow:      time.Second,
	})

	// Selecting a strategy applies its mainstream defaults to a missing trip
	// condition, so these are complete configurations.
	valid(Policy{BreakerStrategy: circuitbreaker.StrategyConsecutive})
	valid(Policy{BreakerStrategy: circuitbreaker.StrategyErrorRate})
	valid(Policy{BreakerStrategy: circuitbreaker.StrategyErrorCount})

	// slow-call has no useful default pair: it stays required.
	invalid(Policy{BreakerStrategy: circuitbreaker.StrategySlowCall})
	// A windowed-strategy knob without a strategy is ambiguous.
	invalid(Policy{ErrorRateThreshold: 0.5})
	// A general knob alone cannot choose a strategy.
	invalid(Policy{OpenDuration: time.Second})
}

// TestBreakerSpecDefaults pins the mainstream defaults the policy applies when
// a strategy is selected without spelling out its trip condition.
func TestBreakerSpecDefaults(t *testing.T) {
	if got := (Policy{BreakerStrategy: circuitbreaker.StrategyConsecutive}).BreakerSpec(); got.FailureThreshold != 5 || got.MinSamples != 5 {
		t.Fatalf("consecutive defaults = %+v, want threshold 5 and min samples 5", got)
	}
	if got := (Policy{BreakerStrategy: circuitbreaker.StrategyErrorCount}).BreakerSpec(); got.FailureThreshold != 5 {
		t.Fatalf("error-count default threshold = %d, want 5", got.FailureThreshold)
	}
	if got := (Policy{BreakerStrategy: circuitbreaker.StrategyErrorRate}).BreakerSpec(); got.FailureRate != 0.5 || got.MinSamples != 5 {
		t.Fatalf("error-rate defaults = %+v, want failure rate 0.5 and min samples 5", got)
	}

	// Explicit knobs always win over the defaults.
	spec := Policy{BreakerStrategy: circuitbreaker.StrategyConsecutive, ErrorThreshold: 9, MinRequests: 20}.BreakerSpec()
	if spec.FailureThreshold != 9 || spec.MinSamples != 20 {
		t.Fatalf("explicit knobs did not win: %+v", spec)
	}
}
