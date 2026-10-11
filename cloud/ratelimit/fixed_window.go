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
	"sync"
	"time"
)

// FixedWindow is the simplest counter: up to limit events per ALIGNED window
// — the window boundaries fall on multiples of window since an arbitrary epoch,
// shared by every FixedWindow with the same width, so budgets align across
// instances without coordination. Its weakness is the boundary burst: up to
// 2*limit events can pass in an instant straddling two windows' edge. Choose
// it when simplicity and a hard per-window guarantee matter more than that
// smoothness; [SlidingWindow] trades two counters' worth of state for it.
type FixedWindow struct {
	mu       sync.Mutex
	limit    float64
	window   time.Duration
	curStart time.Time
	count    float64
}

// NewFixedWindow returns a window counting up to limit events per aligned
// window of the given width.
func NewFixedWindow(limit float64, window time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:    limit,
		window:   window,
		curStart: time.Now().Truncate(window),
	}
}

// AllowN records n events when the current aligned window still has room for
// them.
func (w *FixedWindow) AllowN(n int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if now.Sub(w.curStart) >= w.window {
		// A gap of any length resets the same way: the old window is over.
		w.count = 0
		w.curStart = now.Truncate(w.window)
	}
	if w.count+float64(n) > w.limit {
		return false
	}
	w.count += float64(n)
	return true
}
