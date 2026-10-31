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
	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/stdlib/errutil"
)

// validateBreakerPolicy checks what the breaker cannot build from. Every scalar
// knob is owned by its expr tag (thresholds, windows); what remains is the
// cross-field choice of a strategy and its trip condition.
func validateBreakerPolicy(p BreakerPolicy) error {
	if !p.Enabled() {
		return nil
	}
	// Without an explicit strategy, only an error threshold can be inferred
	// unambiguously (as consecutive). Windowed-strategy knobs have no meaning
	// under that inference, and a general knob such as open-duration cannot
	// choose a strategy at all — name the missing choice instead.
	if p.BreakerStrategy == "" {
		if p.ErrorRateThreshold != 0 || p.SlowCallDurationThreshold != 0 ||
			p.SlowCallRateThreshold != 0 || p.MinRequests != 0 || p.BreakerWindow != 0 {
			return errutil.Explain(nil,
				"breaker-strategy is required when windowed-strategy knobs are set")
		}
		if p.ErrorThreshold == 0 {
			return errutil.Explain(nil,
				"breaker-strategy (or error-threshold) is required to enable the breaker")
		}
	}
	_, err := circuitbreaker.New(p.Spec())
	return err
}
