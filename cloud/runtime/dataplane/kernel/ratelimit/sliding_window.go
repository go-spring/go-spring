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
	"sync"
	"time"
)

// SlidingWindow approximates a rolling-window counter with the standard
// weighted two-window estimate: it blends the previous window's count by the
// fraction of it still overlapping the current instant. This bounds events per
// window without storing a timestamp per event.
type SlidingWindow struct {
	mu        sync.Mutex
	limit     float64
	window    time.Duration
	curStart  time.Time
	curCount  float64
	prevCount float64
}

// NewSlidingWindow returns a window counting up to limit events per rolling
// window, starting its first window now.
func NewSlidingWindow(limit float64, window time.Duration) *SlidingWindow {
	return &SlidingWindow{limit: limit, window: window, curStart: time.Now()}
}

// AllowN records n events when the window's estimate still has room for them.
func (w *SlidingWindow) AllowN(n int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(w.curStart)
	if elapsed >= w.window {
		// Roll forward: an adjacent window becomes prev; a gap of two or more
		// windows means the old counts have fully aged out.
		if elapsed >= 2*w.window {
			w.prevCount = 0
		} else {
			w.prevCount = w.curCount
		}
		w.curCount = 0
		w.curStart = now
		elapsed = 0
	}
	weight := float64(w.window-elapsed) / float64(w.window)
	estimate := w.prevCount*weight + w.curCount
	if estimate+float64(n) > w.limit {
		return false
	}
	w.curCount += float64(n)
	return true
}
