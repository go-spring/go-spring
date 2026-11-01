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

// TestFixedWindowWidthChangeResets pins that a width change starts fresh state:
// a count recorded under a different width means nothing in the new one.
func TestFixedWindowWidthChangeResets(t *testing.T) {
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
