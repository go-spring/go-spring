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

// sliding_window.go is the sliding-window counter: a rolling per-window cap
// without a timestamp per event.

package ratelimit

import (
	"context"
	"sync"
	"time"
)

// slidingWindow approximates a rolling-window counter with the standard
// weighted two-window estimate: the count in view is the current window's
// count plus the previous window's count weighted by the fraction of it still
// inside the rolling interval. It bounds events per window without storing a
// timestamp per event. The estimate assumes the previous window's events were
// spread evenly, so it can over- or under-count just after a boundary — that
// assumption is what buys the constant state.
type slidingWindow struct {
	mu        sync.Mutex
	limit     float64       // events allowed in a full window; fractional, like the rate it comes from
	window    time.Duration // the rolling interval
	start     time.Time     // start of the current window, advanced by whole windows; zero until first use
	count     float64       // events recorded in the current window; a float so it compares against a fractional cap
	prevCount float64       // events the window before the current one carried
}

// Allow charges n units under policy, materializing the window on the first
// call. A new WINDOW starts fresh state, because counts under the old one mean
// nothing in it; a new RATE with the same window only moves the cap, so it is
// applied on the spot.
func (w *slidingWindow) Allow(_ context.Context, policy Policy, n int) (bool, error) {
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

// charge rolls to now, then records n events when the estimated count in view
// still has room for them. A refused charge leaves the counts untouched — the
// batch is all-or-nothing. Callers hold mu.
func (w *slidingWindow) charge(now time.Time, n int) bool {
	w.roll(now)
	if w.estimate(now)+float64(n) > w.limit {
		return false
	}
	w.count += float64(n)
	return true
}

// roll advances the window to now: once the current window has elapsed it
// becomes the previous one, and a gap of two or more windows ages both counts
// out. The start advances by whole windows rather than jumping to now, so the
// window keeps its phase: after an idle gap the previous count is still
// weighted by the part of it that actually overlaps the rolling interval,
// instead of being inherited at full weight by a freshly opened window.
// Callers hold mu.
func (w *slidingWindow) roll(now time.Time) {
	elapsed := now.Sub(w.start)
	if elapsed < w.window {
		return
	}
	if elapsed >= 2*w.window {
		w.prevCount = 0
	} else {
		w.prevCount = w.count
	}
	w.count = 0
	w.start = w.start.Add(elapsed.Truncate(w.window))
}

// estimate is the count in view at now: the current window plus the previous
// one weighted by the fraction still inside the rolling interval —
// prevCount*(window-elapsed)/window + count. Callers hold mu and have already
// rolled.
func (w *slidingWindow) estimate(now time.Time) float64 {
	weight := float64(w.window-now.Sub(w.start)) / float64(w.window)
	return w.prevCount*weight + w.count
}

// open is the first-use and window-change transition: it starts a fresh
// window at now, discarding counts that belong to a different one. Callers
// hold mu.
func (w *slidingWindow) open(window time.Duration, now time.Time) {
	w.window = window
	w.start = now
	w.count = 0
	w.prevCount = 0
}
