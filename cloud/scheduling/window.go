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

package scheduling

import (
	"time"

	"go-spring.org/stdlib/errutil"
)

// DailyWindow returns a [Trigger] that fires tr's schedule only inside the
// daily window [start, end), where start and end are time-of-day offsets from
// midnight — e.g. 9*time.Hour, 18*time.Hour for "9:00 to 18:00". A fire that
// would land outside the window is moved to the start of the next window; the
// wrapped trigger keeps anchoring on the fires that did run. Time of day is
// evaluated in the location of each computed fire time.
//
// It panics if start and end do not form a window within one day
// (0 <= start < end <= 24h), or if tr is nil.
func DailyWindow(start, end time.Duration, tr Trigger) Trigger {
	if tr == nil {
		panic(errutil.Explain(nil, "scheduling: DailyWindow requires a non-nil trigger"))
	}
	if start < 0 || end > 24*time.Hour || start >= end {
		panic(errutil.Explain(nil, "scheduling: DailyWindow requires 0 <= start < end <= 24h"))
	}
	return dailyWindow{start: start, end: end, tr: tr}
}

// dailyWindow is the [DailyWindow] trigger: tr supplies the cadence, the window
// decides which of its fires actually happen.
type dailyWindow struct {
	start, end time.Duration
	tr         Trigger
}

// Next implements [Trigger].
func (w dailyWindow) Next(tc TriggerContext) time.Time {
	candidate := w.tr.Next(tc)
	if candidate.IsZero() {
		return candidate
	}

	// tod is the candidate's offset from its own midnight.
	day := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, candidate.Location())
	tod := candidate.Sub(day)
	if tod >= w.start && tod < w.end {
		return candidate
	}
	// Outside the window: fire at the start of the next window instead — later
	// the same day when the candidate was early, the next day otherwise.
	if tod < w.start {
		return day.Add(w.start)
	}
	return day.Add(24 * time.Hour).Add(w.start)
}

// isFixedDelay reports whether tr is a fixedDelay trigger, looking through
// [DailyWindow] so a windowed fixed-delay job keeps its serial semantics.
func isFixedDelay(tr Trigger) bool {
	switch v := tr.(type) {
	case fixedDelay:
		return true
	case dailyWindow:
		return isFixedDelay(v.tr)
	}
	return false
}
