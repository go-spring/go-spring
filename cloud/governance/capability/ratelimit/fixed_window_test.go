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
	"testing"
	"testing/synctest"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestFixedWindowLimitAndRoll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		p := windowPolicy(2, 20*time.Millisecond)
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).False() // window full
		time.Sleep(20 * time.Millisecond)      // crossed into the next aligned window
		assert.That(t, allow(w, p, 1)).True()  // count reset by the roll
	})
}

// TestFixedWindowRateChangeMovesCap pins the in-place rule update: a new RATE
// under the same WIDTH moves the cap on the spot, while the count already
// recorded keeps counting against it.
func TestFixedWindowRateChangeMovesCap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		assert.That(t, allow(w, windowPolicy(1, time.Second), 1)).True()
		assert.That(t, allow(w, windowPolicy(1, time.Second), 1)).False()
		assert.That(t, allow(w, windowPolicy(2, time.Second), 1)).True() // cap raised, count kept
		assert.That(t, allow(w, windowPolicy(2, time.Second), 1)).False()
	})
}

// TestFixedWindowWindowChangeResets pins that a window change starts fresh
// state: a count recorded under a different window means nothing in the new one.
func TestFixedWindowWindowChangeResets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		assert.That(t, allow(w, windowPolicy(2, time.Second), 2)).True()
		assert.That(t, allow(w, windowPolicy(2, time.Second), 1)).False()
		assert.That(t, allow(w, windowPolicy(2, time.Minute), 2)).True()
	})
}

// TestFixedWindowAlignedBoundaries pins the alignment property from the
// outside: two independent counters of the same width roll at the SAME wall
// clock instant (the window boundary), not each at its own creation
// anniversary — that is what lets per-instance budgets line up without
// coordination.
func TestFixedWindowAlignedBoundaries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const win = 100 * time.Millisecond
		p := windowPolicy(1, win)
		w1 := &fixedWindow{}
		w2 := &fixedWindow{}
		assert.That(t, allow(w1, p, 1)).True()
		assert.That(t, allow(w2, p, 1)).True()
		assert.That(t, allow(w1, p, 1)).False()
		assert.That(t, allow(w2, p, 1)).False()
		// The NEXT shared boundary re-opens both together.
		time.Sleep(2 * win)
		assert.That(t, allow(w1, p, 1)).True()
		assert.That(t, allow(w2, p, 1)).True()
	})
}

// TestFixedWindowFractionalLimit pins why the count is a float: the cap comes
// from rate × width and can land between integers, and whole events are
// admitted only while they still fit under it.
func TestFixedWindowFractionalLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		p := windowPolicy(2.5, time.Second) // limit 2.5: room for two events, not three
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).False() // 2+1 > 2.5

		// The cap floors at one event: a rate that would buy half an event
		// per window still admits exactly one.
		w2 := &fixedWindow{}
		tiny := windowPolicy(0.5, time.Second)
		assert.That(t, allow(w2, tiny, 1)).True()
		assert.That(t, allow(w2, tiny, 1)).False()
	})
}

// TestFixedWindowRejectedChargeIsAllOrNothing pins that a batch over the cap
// is refused as a whole: none of its units are counted, so the same room is
// still available to a charge that fits.
func TestFixedWindowRejectedChargeIsAllOrNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		p := windowPolicy(5, time.Hour)        // never rolls within the test
		assert.That(t, allow(w, p, 6)).False() // refused as a whole
		assert.That(t, allow(w, p, 5)).True()  // the room was never spent
		assert.That(t, allow(w, p, 1)).False()
	})
}

// TestFixedWindowUnlimitedPolicy pins the two charges that bypass the budget:
// a zero rate disables the stage, and a non-positive charge spends nothing.
func TestFixedWindowUnlimitedPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		assert.That(t, allow(w, Policy{}, 100)).True()

		p := windowPolicy(1, time.Second)
		assert.That(t, allow(w, p, 0)).True()
		assert.That(t, allow(w, p, -1)).True()
		assert.That(t, allow(w, p, 1)).True() // the budget was untouched
		assert.That(t, allow(w, p, 1)).False()
	})
}

// TestFixedWindowIdleGapResets pins that an idle stretch spanning several
// windows needs no catch-up: the next call lands directly in the current
// window with a fresh count.
func TestFixedWindowIdleGapResets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &fixedWindow{}
		p := windowPolicy(1, 10*time.Millisecond)
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).False()
		time.Sleep(100 * time.Millisecond) // ten windows came and went unseen
		assert.That(t, allow(w, p, 1)).True()
		assert.That(t, allow(w, p, 1)).False()
	})
}
