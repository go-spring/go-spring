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

	"go-spring.org/spring/cloud/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

// next is a small helper: it computes the cron trigger's next fire strictly
// after the given reference time.
func nextAfter(t *testing.T, expr string, ref time.Time) time.Time {
	t.Helper()
	tr, err := scheduling.ParseCron(expr)
	assert.Error(t, err).Nil()
	return tr.Next(scheduling.TriggerContext{Now: ref})
}

func TestParseCronErrors(t *testing.T) {
	cases := []string{
		"",              // empty
		"* * * *",       // too few fields
		"* * * * * *",   // too many fields
		"60 * * * *",    // minute out of range
		"* 24 * * *",    // hour out of range
		"* * 0 * *",     // day-of-month below range
		"* * * 13 *",    // month out of range
		"* * * * 8",     // day-of-week out of range
		"*/0 * * * *",   // zero step
		"5-1 * * * *",   // inverted range
		"a * * * *",     // non-numeric
		"@bogus",        // unknown macro
	}
	for _, c := range cases {
		_, err := scheduling.ParseCron(c)
		assert.Error(t, err).NotNil(c)
	}
}

func TestCronEveryMinute(t *testing.T) {
	ref := time.Date(2026, 7, 18, 10, 30, 15, 0, time.UTC)
	got := nextAfter(t, "* * * * *", ref)
	// Seconds are dropped and we advance to the next whole minute.
	assert.That(t, got).Equal(time.Date(2026, 7, 18, 10, 31, 0, 0, time.UTC))
}

func TestCronStepAndBoundary(t *testing.T) {
	// Every 15 minutes: from 10:30:15 the next slot is 10:45.
	ref := time.Date(2026, 7, 18, 10, 30, 15, 0, time.UTC)
	got := nextAfter(t, "*/15 * * * *", ref)
	assert.That(t, got).Equal(time.Date(2026, 7, 18, 10, 45, 0, 0, time.UTC))

	// Hour rollover: at 10:59 the next :00 minute lands in the next hour.
	ref = time.Date(2026, 7, 18, 10, 59, 0, 0, time.UTC)
	got = nextAfter(t, "0 * * * *", ref)
	assert.That(t, got).Equal(time.Date(2026, 7, 18, 11, 0, 0, 0, time.UTC))
}

func TestCronDailyAndMacros(t *testing.T) {
	ref := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	// "@daily" == midnight, so the next fire is the start of the next day.
	got := nextAfter(t, "@daily", ref)
	assert.That(t, got).Equal(time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC))

	// Explicit form matches the macro.
	got = nextAfter(t, "0 0 * * *", ref)
	assert.That(t, got).Equal(time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC))
}

func TestCronDayOfWeekOrDayOfMonth(t *testing.T) {
	// 2026-07-18 is a Saturday. "0 0 1 * 1" fires on the 1st of the month OR any
	// Monday (both restricted => OR). The next match after 2026-07-18 is Monday
	// 2026-07-20.
	ref := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	got := nextAfter(t, "0 0 1 * 1", ref)
	assert.That(t, got).Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
}

func TestCronSpecificDayOfMonth(t *testing.T) {
	// Only day-of-month restricted: fire at 00:00 on the 1st of each month.
	ref := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	got := nextAfter(t, "0 0 1 * *", ref)
	assert.That(t, got).Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
}

func TestCronRespectsLastScheduled(t *testing.T) {
	tr, err := scheduling.ParseCron("* * * * *")
	assert.Error(t, err).Nil()
	// When LastScheduled is ahead of Now, Next advances past it so the same
	// minute is never fired twice.
	now := time.Date(2026, 7, 18, 10, 30, 15, 0, time.UTC)
	last := time.Date(2026, 7, 18, 10, 31, 0, 0, time.UTC)
	got := tr.Next(scheduling.TriggerContext{Now: now, LastScheduled: last})
	assert.That(t, got).Equal(time.Date(2026, 7, 18, 10, 32, 0, 0, time.UTC))
}
