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
	"go-spring.org/cloud/runtime/dataplane/kernel/circuitbreaker"
	"go-spring.org/stdlib/errutil"
)

// ValidateSettings reports whether every policy in s can be materialized. It is
// the pre-commit check the governance center runs before applying s, so a
// document that cannot serve is rejected whole instead of silently losing a
// stage on a later call.
func ValidateSettings(s Settings) error {
	if err := validatePolicy(s.Default); err != nil {
		return errutil.Explain(err, "resilience: default policy")
	}
	for i := range s.Rules {
		if err := validatePolicy(s.Rules[i].Policy); err != nil {
			return errutil.Explain(err, "resilience: rule %q", s.Rules[i].Service)
		}
	}
	return nil
}

// validatePolicy checks the values a stage cannot build from. Only the breaker
// has a constructor that rejects input today; the rate budget treats a
// non-positive limit as "off", so a negative one is called out explicitly
// instead of silently becoming a pass-through.
func validatePolicy(p Policy) error {
	if p.RateLimit < 0 {
		return errutil.Explain(nil, "negative rate limit %v", p.RateLimit)
	}
	if !p.BreakerEnabled() {
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
	_, err := circuitbreaker.New(p.BreakerSpec())
	return err
}
