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

// fixed_window.go is the fixed-window counter: the cheapest per-window cap,
// at the cost of a boundary burst.

package ratelimit

import (
	"context"
	"sync"
	"time"
)

// fixedWindow counts events against a per-window cap: up to limit events in
// each ALIGNED window. Boundaries fall on multiples of the window measured from
// the zero time, so every counter of the same width rolls at the same
// wall-clock instant — instances line up without coordinating, as long as their
// clocks agree. The weakness is the boundary burst: a client can spend the
// whole cap at the end of one window and the whole cap again at the start of
// the next, so up to 2*limit events pass in a near-instant straddling an edge.
// Choose it when an aligned per-window cap is enough; [slidingWindow] spends
// one more counter to smooth that edge.
type fixedWindow struct {
	mu     sync.Mutex
	limit  float64       // events allowed in a full window; fractional, like the rate it comes from
	window time.Duration // the aligned window width
	start  time.Time     // aligned start of the window holding the count; zero until first use
	count  float64       // events recorded in that window; a float so it compares against a fractional cap
}

// Allow charges n units under policy, materializing the window on the first
// call. A new WINDOW starts fresh state, because a count recorded under the
// old one says nothing about how full the new window is; a new RATE with the
// same window only moves the cap, so it is applied on the spot.
func (w *fixedWindow) Allow(_ context.Context, policy Policy, n int) (bool, error) {
	if policy.RateLimit <= 0 || n <= 0 {
		return true, nil
	}
	window, limit := windowLimit(policy)

	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if w.start.IsZero() || w.window != window {
		w.open(window, now)
	}
	w.limit = limit // derived from the policy on every call
	return w.charge(now, n), nil
}

// charge rolls to now, then records n events when the current window still has
// room for them. A refused charge leaves the count untouched — the batch is
// all-or-nothing. Callers hold mu.
func (w *fixedWindow) charge(now time.Time, n int) bool {
	w.roll(now)
	if w.count+float64(n) > w.limit {
		return false
	}
	w.count += float64(n)
	return true
}

// roll moves to the window containing now: once the current one has elapsed,
// the count starts over. Any longer gap lands in some later window and resets
// the same way — the intervening empty windows were never observed and need no
// processing. Callers hold mu.
func (w *fixedWindow) roll(now time.Time) {
	if now.Sub(w.start) < w.window {
		return
	}
	w.count = 0
	w.start = now.Truncate(w.window)
}

// open is the first-use and window-change transition: it starts a fresh
// aligned window at now, discarding a count that belongs to a different one.
// Callers hold mu.
func (w *fixedWindow) open(window time.Duration, now time.Time) {
	w.window = window
	w.start = now.Truncate(window)
	w.count = 0
}
