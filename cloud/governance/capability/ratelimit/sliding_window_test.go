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

package ratelimit

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestSlidingWindowLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &slidingWindow{}
		p := windowPolicy(2, time.Hour) // never rolls within the test
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).False()
	})
}

// TestSlidingWindowWindowChangeResets pins that a window change starts fresh
// state: counts recorded under a different window mean nothing in the new one.
func TestSlidingWindowWindowChangeResets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &slidingWindow{}
		assert.That(t, allow(w, windowPolicy(2, time.Second), 2)).True()
		assert.That(t, allow(w, windowPolicy(2, time.Second), 1)).False()
		assert.That(t, allow(w, windowPolicy(2, time.Minute), 2)).True()
	})
}

// TestSlidingWindowWeightedRoll pins the two-window blend: after a full
// window the previous count is weighted by the fraction still in view.
func TestSlidingWindowWeightedRoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &slidingWindow{}
		p := windowPolicy(10, time.Second)
		assert.That(t, allow(w, p, 5)).True()

		time.Sleep(time.Second) // roll: the old count becomes prev, fully weighted
		assert.That(t, allow(w, p, 5)).True()
		assert.That(t, allow(w, p, 1)).False()

		time.Sleep(500 * time.Millisecond) // prev is half-weighted: 2.5 + 5 spent
		assert.That(t, allow(w, p, 2)).True()
		assert.That(t, allow(w, p, 1)).False()

		time.Sleep(2 * time.Second) // both windows aged out
		assert.That(t, allow(w, p, 10)).True()
	})
}

// TestSlidingWindowIdleGapKeepsPhase pins that a gap longer than one window
// does not inherit the previous count at full weight: the window start advances
// by whole windows, so 10 events from 1.5 windows ago weigh half, not all.
func TestSlidingWindowIdleGapKeepsPhase(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &slidingWindow{}
		p := windowPolicy(10, time.Second)
		assert.That(t, allow(w, p, 10)).True()

		time.Sleep(1500 * time.Millisecond) // one and a half windows idle
		assert.That(t, allow(w, p, 5)).True()
		assert.That(t, allow(w, p, 1)).False()
	})
}

// TestSlidingWindowRejectedChargeIsAllOrNothing pins that a batch over the cap
// is refused as a whole: none of its units are counted, so the same room is
// still available to a charge that fits.
func TestSlidingWindowRejectedChargeIsAllOrNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &slidingWindow{}
		p := windowPolicy(5, time.Hour)        // never rolls within the test
		assert.That(t, allow(w, p, 6)).False() // refused as a whole
		assert.That(t, allow(w, p, 5)).True()  // the room was never spent
		assert.That(t, allow(w, p, 1)).False()
	})
}

// TestSlidingWindowUnlimitedPolicy pins the two charges that bypass the budget:
// a zero rate disables the stage, and a non-positive charge spends nothing.
func TestSlidingWindowUnlimitedPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &slidingWindow{}
		assert.That(t, allow(w, Policy{}, 100)).True()

		p := windowPolicy(1, time.Second)
		assert.That(t, allow(w, p, 0)).True()
		assert.That(t, allow(w, p, -1)).True()
		assert.That(t, allow(w, p, 1)).True() // the budget was untouched
		assert.That(t, allow(w, p, 1)).False()
	})
}

// TestSlidingWindowLimitRaiseKeepsCount pins in-place limit changes: raising the
// cap does not forget the events already counted in the window.
func TestSlidingWindowLimitRaiseKeepsCount(t *testing.T) {
	w := &slidingWindow{}

	policy := Policy{RateLimit: 2, Window: time.Second}
	for range 2 {
		allowed, _ := w.Allow(context.Background(), policy, 1)
		assert.That(t, allowed).True()
	}
	allowed, _ := w.Allow(context.Background(), policy, 1)
	assert.That(t, allowed).False()

	// Raising the limit keeps the two events already counted.
	policy.RateLimit = 4
	for range 2 {
		allowed, _ := w.Allow(context.Background(), policy, 1)
		assert.That(t, allowed).True()
	}
	allowed, _ = w.Allow(context.Background(), policy, 1)
	assert.That(t, allowed).False()
}
