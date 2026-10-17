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

	"go-spring.org/stdlib/testing/assert"
)

// TestPolicyIsZeroCoversNewKnobs pins that IsZero sees the newer knobs: a
// policy setting ONLY one of them is no longer "no protection".
func TestPolicyIsZeroCoversNewKnobs(t *testing.T) {
	assert.That(t, (ClientPolicy{RateLimitWarmUpDuration: time.Second}).IsZero()).False()
	assert.That(t, (ClientPolicy{WarmUpColdFactor: 3}).IsZero()).False()
	assert.That(t, (ClientPolicy{BreakerStrategy: BreakerSlowCall}).IsZero()).False()
	assert.That(t, (ClientPolicy{BreakerStrategy: BreakerErrorCount}).IsZero()).False()
}
