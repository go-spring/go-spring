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
	"testing"
	"time"

	"go-spring.org/cloud/ratelimit"
	"go-spring.org/stdlib/testing/assert"
)

func TestFixedWindowLimitAndRoll(t *testing.T) {
	w := ratelimit.NewFixedWindow(2, 20*time.Millisecond)
	assert.That(t, w.AllowN(1)).True()
	assert.That(t, w.AllowN(1)).True()
	assert.That(t, w.AllowN(1)).False() // window full
	time.Sleep(25 * time.Millisecond)   // crossed into the next aligned window
	assert.That(t, w.AllowN(1)).True()  // count reset by the roll
}

// TestFixedWindowAlignedBoundaries pins the alignment property from the
// outside: two independent counters of the same width roll at the SAME wall
// clock instant (the window boundary), not each at its own creation
// anniversary — that is what lets per-instance budgets line up without
// coordination.
func TestFixedWindowAlignedBoundaries(t *testing.T) {
	const win = 100 * time.Millisecond
	w1 := ratelimit.NewFixedWindow(1, win)
	w2 := ratelimit.NewFixedWindow(1, win)
	assert.That(t, w1.AllowN(1)).True()
	assert.That(t, w2.AllowN(1)).True()
	assert.That(t, w1.AllowN(1)).False()
	assert.That(t, w2.AllowN(1)).False()
	// Wait for the NEXT shared boundary: both must re-open together.
	boundary := time.Now().Truncate(win).Add(2 * win)
	time.Sleep(time.Until(boundary))
	assert.That(t, w1.AllowN(1)).True()
	assert.That(t, w2.AllowN(1)).True()
}
