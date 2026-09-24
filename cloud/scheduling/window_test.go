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

package scheduling_test

import (
	"testing"
	"time"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

func TestDailyWindowKeepsInWindowFires(t *testing.T) {
	// Window 9:00–18:00, a fixed-rate cadence of 1h anchored at 10:00.
	tr := scheduling.DailyWindow(9*time.Hour, 18*time.Hour, scheduling.FixedRate(time.Hour))
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	last := base.Add(time.Hour) // 11:00, inside the window

	next := tr.Next(scheduling.TriggerContext{Now: last, LastScheduled: last})
	assert.That(t, next).Equal(base.Add(2 * time.Hour)) // 12:00, still inside
}

func TestDailyWindowMovesLateFiresToNextStart(t *testing.T) {
	tr := scheduling.DailyWindow(9*time.Hour, 18*time.Hour, scheduling.FixedRate(time.Hour))
	// Anchor at 17:00 so the wrapped cadence's next slot is 18:00 — outside
	// [9:00, 18:00), so the fire moves to the next day's 9:00 start.
	now := time.Date(2026, 9, 24, 17, 30, 0, 0, time.UTC)
	last := time.Date(2026, 9, 24, 17, 0, 0, 0, time.UTC)
	next := tr.Next(scheduling.TriggerContext{Now: now, LastScheduled: last})
	assert.That(t, next).Equal(time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC))
}

func TestDailyWindowMovesEarlyFiresToSameDayStart(t *testing.T) {
	tr := scheduling.DailyWindow(9*time.Hour, 18*time.Hour, scheduling.FixedRate(time.Hour))
	// The wrapped next lands at 7:00 — before the window, so the fire moves to
	// the same day's 9:00 start.
	now := time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC)
	next := tr.Next(scheduling.TriggerContext{Now: now})
	// FixedRate's first fire is one interval out: 7:00, before the window.
	assert.That(t, next).Equal(time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC))
}

func TestDailyWindowPassesThroughZeroNext(t *testing.T) {
	tr := scheduling.DailyWindow(9*time.Hour, 18*time.Hour, scheduling.After(time.Hour))
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	// Inside the window: the one fire happens as After would schedule it.
	first := tr.Next(scheduling.TriggerContext{Now: base})
	assert.That(t, first).Equal(base.Add(time.Hour))

	// After it fires, the wrapped trigger reports zero and so does the window.
	next := tr.Next(scheduling.TriggerContext{Now: first, LastScheduled: first})
	assert.That(t, next).Equal(time.Time{})
}

func TestDailyWindowPanicOnInvalid(t *testing.T) {
	fixed := scheduling.FixedRate(time.Hour)
	assert.Panic(t, func() { scheduling.DailyWindow(18*time.Hour, 9*time.Hour, fixed) }, "start < end")
	assert.Panic(t, func() { scheduling.DailyWindow(-time.Hour, 9*time.Hour, fixed) }, "start < end")
	assert.Panic(t, func() { scheduling.DailyWindow(0, 25*time.Hour, fixed) }, "start < end")
	assert.Panic(t, func() { scheduling.DailyWindow(0, 9*time.Hour, nil) }, "non-nil")
}
