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

// fixedWindow is the simplest counter: up to limit events per ALIGNED window —
// the boundaries fall on multiples of the width counted from the zero time, so
// every fixedWindow of the same width rolls at the same instant and instances
// align without coordination. Its weakness is the boundary burst: up to 2*limit
// events can pass in an instant straddling two windows' edge. Choose it when
// simplicity and a hard per-window guarantee matter more than that evenness;
// [slidingWindow] trades two counters' worth of state for it.
type fixedWindow struct {
	mu           sync.Mutex
	limit        float64       // events allowed in a full window
	width        time.Duration // the aligned window width
	windowStart  time.Time     // when the current window began, truncated to width
	currentCount float64       // events recorded in the current window

	// policy and configured are driven by [fixedWindow.Allow]: the first call
	// materializes the window from the rule, later ones reconfigure it in place.
	policy     Policy
	configured bool
}

// Allow charges n units under policy, materializing the window on the first
// call and reconfiguring it in place whenever the rule changes. A new WIDTH
// starts fresh state, because the old window's count means nothing in it.
func (w *fixedWindow) Allow(_ context.Context, policy Policy, n int) (bool, error) {
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

// charge records n events when the current aligned window still has room for
// them. Callers hold mu.
func (w *fixedWindow) charge(n int) bool {
	now := time.Now()
	w.roll(now)
	if w.currentCount+float64(n) > w.limit {
		return false
	}
	w.currentCount += float64(n)
	return true
}

// roll advances the window to now: once the current window has elapsed, the
// next aligned one begins. A gap of any length resets the same way — the old
// window is over. Callers hold mu.
func (w *fixedWindow) roll(now time.Time) {
	if now.Sub(w.windowStart) < w.width {
		return
	}
	w.currentCount = 0
	w.windowStart = now.Truncate(w.width)
}

// reset is the first-use and width-change transition: it starts a fresh
// aligned window, discarding a count that belongs to a different width.
// Callers hold mu.
func (w *fixedWindow) reset(width time.Duration, limit float64) {
	w.width = width
	w.limit = limit
	w.windowStart = time.Now().Truncate(width)
	w.currentCount = 0
	w.configured = true
}
