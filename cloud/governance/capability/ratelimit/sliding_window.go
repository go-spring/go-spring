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
// weighted two-window estimate: it blends the previous window's count by the
// fraction of it still overlapping the current instant. This bounds events per
// window without storing a timestamp per event.
type slidingWindow struct {
	mu            sync.Mutex
	limit         float64       // events allowed in a full window
	width         time.Duration // the rolling interval
	windowStart   time.Time     // when the current window began
	currentCount  float64       // events recorded in the current window
	previousCount float64       // events the previous window carried

	// policy and configured are driven by [slidingWindow.Allow]: the first call
	// materializes the window from the rule, later ones reconfigure it in place.
	policy     Policy
	configured bool
}

// Allow charges n units under policy, materializing the window on the first
// call and reconfiguring it in place whenever the rule changes. A new WIDTH
// starts fresh state, because the old window's counts mean nothing in it.
func (w *slidingWindow) Allow(_ context.Context, policy Policy, n int) (bool, error) {
	if policy.RateLimit <= 0 || n <= 0 {
		return true, nil
	}
	width, limit := windowLimit(policy)

	w.mu.Lock()
	defer w.mu.Unlock()
	switch {
	case !w.configured || w.width != width:
		w.reset(width, limit)
	case w.policy != policy:
		w.limit = limit
	}
	w.policy = policy
	return w.charge(n), nil
}

// charge records n events when the estimated count in view still has room for
// them. Callers hold mu.
func (w *slidingWindow) charge(n int) bool {
	now := time.Now()
	w.roll(now)
	if w.estimate(now)+float64(n) > w.limit {
		return false
	}
	w.currentCount += float64(n)
	return true
}

// roll advances the window to now: once the current window has elapsed it
// becomes the previous one, and a gap of two or more windows ages both counts
// out. Callers hold mu.
func (w *slidingWindow) roll(now time.Time) {
	elapsed := now.Sub(w.windowStart)
	if elapsed < w.width {
		return
	}
	if elapsed >= 2*w.width {
		w.previousCount = 0
	} else {
		w.previousCount = w.currentCount
	}
	w.currentCount = 0
	w.windowStart = now
}

// estimate is the count in view at now: the current window plus the previous
// one weighted by the fraction still inside the rolling interval. Callers hold
// mu and have already rolled.
func (w *slidingWindow) estimate(now time.Time) float64 {
	weight := float64(w.width-now.Sub(w.windowStart)) / float64(w.width)
	return w.previousCount*weight + w.currentCount
}

// reset is the first-use and width-change transition: it starts a fresh
// window, discarding counts that belong to a different width. Callers hold mu.
func (w *slidingWindow) reset(width time.Duration, limit float64) {
	w.width = width
	w.limit = limit
	w.windowStart = time.Now()
	w.currentCount = 0
	w.previousCount = 0
	w.configured = true
}
