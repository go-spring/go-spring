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

package ratelimit_test

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/stdlib/testing/assert"
)

// TestBudgetUsage pins the holder-facing read: an unlimited budget reports
// ok=false (so no series is published), a configured one reports both
// algorithms in their own units.
func TestBudgetUsage(t *testing.T) {
	var unlimited ratelimit.Budget
	if usage := unlimited.Usage(); usage.Limit != 0 {
		t.Fatalf("an unlimited budget reported usage: %+v", usage)
	}

	bucket := ratelimit.NewBudget(ratelimit.Spec{RateLimit: 5, Burst: 10})
	usage := bucket.Usage()
	assert.That(t, usage.Limit).Equal(float64(10))
	assert.That(t, usage.Used < 0.01).True()

	assert.That(t, bucket.Allow(context.Background(), 1)).True()
	usage = bucket.Usage()
	assert.That(t, usage.Used > 0.99 && usage.Used < 1.01).True()

	window := ratelimit.NewBudget(ratelimit.Spec{
		RateLimit: 5,
		Algorithm: ratelimit.AlgorithmSlidingWindow,
	})
	usage = window.Usage()
	assert.That(t, usage.Limit).Equal(float64(5))
	assert.That(t, usage.Used).Equal(float64(0))
	assert.That(t, window.Allow(context.Background(), 2)).True()
	usage = window.Usage()
	assert.That(t, usage.Used > 1.99 && usage.Used < 2.01).True()
}
