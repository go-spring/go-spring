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

package resilience_test

import (
	"context"
	"testing"
	"time"

	"go-spring.org/spring/resilience"
	"go-spring.org/stdlib/testing/assert"
)

func newLimiter(t *testing.T, p resilience.LimitPolicy) resilience.RateLimiter {
	d, err := resilience.MustGetLimiter("default")
	assert.That(t, err).Nil()
	rl, err := d.NewRateLimiter(p)
	assert.That(t, err).Nil()
	return rl
}

func TestRateLimiterUnlimited(t *testing.T) {
	rl := newLimiter(t, resilience.LimitPolicy{}) // Rate 0 -> pass-through
	for range 1000 {
		ok, err := rl.Allow(context.Background(), "k")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	}
}

func TestRateLimiterTokenBucketBurst(t *testing.T) {
	rl := newLimiter(t, resilience.LimitPolicy{Rate: 1, Burst: 3})
	ctx := context.Background()
	// Burst of 3 is available immediately; the 4th is rejected.
	for range 3 {
		ok, _ := rl.Allow(ctx, "k")
		assert.That(t, ok).True()
	}
	ok, _ := rl.Allow(ctx, "k")
	assert.That(t, ok).False()
}

func TestRateLimiterPerKeyIsolation(t *testing.T) {
	rl := newLimiter(t, resilience.LimitPolicy{Rate: 1, Burst: 1})
	ctx := context.Background()
	ok, _ := rl.Allow(ctx, "a")
	assert.That(t, ok).True()
	// A different key has its own budget.
	ok, _ = rl.Allow(ctx, "b")
	assert.That(t, ok).True()
	// The first key is now empty.
	ok, _ = rl.Allow(ctx, "a")
	assert.That(t, ok).False()
}

func TestRateLimiterAllowN(t *testing.T) {
	rl := newLimiter(t, resilience.LimitPolicy{Rate: 1, Burst: 5})
	ctx := context.Background()
	ok, _ := rl.AllowN(ctx, "k", 5)
	assert.That(t, ok).True()
	ok, _ = rl.AllowN(ctx, "k", 1)
	assert.That(t, ok).False()
}

func TestRateLimiterSlidingWindow(t *testing.T) {
	// Rate 10/s over a 1s window => cap of 10 events per rolling second.
	rl := newLimiter(t, resilience.LimitPolicy{
		Rate:      10,
		Algorithm: resilience.SlidingWindow,
		Window:    time.Second,
	})
	ctx := context.Background()
	allowed := 0
	for range 20 {
		if ok, _ := rl.Allow(ctx, "k"); ok {
			allowed++
		}
	}
	assert.That(t, allowed).Equal(10)
}

func TestRegisterLimiterGuards(t *testing.T) {
	assert.Panic(t, func() { resilience.RegisterLimiter("", builtinNop{}) }, "empty name")
	assert.Panic(t, func() { resilience.RegisterLimiter("x", nil) }, "nil limiter driver")
	assert.Panic(t, func() { resilience.RegisterLimiter("default", builtinNop{}) }, "already registered")
	_, err := resilience.MustGetLimiter("nope")
	assert.That(t, err).NotNil()
}

type builtinNop struct{}

func (builtinNop) NewRateLimiter(resilience.LimitPolicy) (resilience.RateLimiter, error) {
	return nil, nil
}
