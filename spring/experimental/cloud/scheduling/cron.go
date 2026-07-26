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
	"fmt"
	"strconv"
	"strings"
	"time"
)

// cronSpec is a parsed 5-field cron expression. Each field is a bitmask of the
// values that match; a set bit at index i means "i matches". dom/dow also record
// whether the field was restricted (anything other than "*"), which drives the
// classic day-of-month / day-of-week OR rule.
type cronSpec struct {
	minute uint64 // bits 0..59
	hour   uint64 // bits 0..23
	dom    uint64 // bits 1..31
	month  uint64 // bits 1..12
	dow    uint64 // bits 0..6 (Sunday=0)

	domRestricted bool
	dowRestricted bool
}

// Cron returns a [Trigger] for a standard 5-field cron expression
// ("minute hour day-of-month month day-of-week"). It panics on a malformed
// expression; use [ParseCron] to handle the error instead.
func Cron(expr string) Trigger {
	t, err := ParseCron(expr)
	if err != nil {
		panic(err)
	}
	return t
}

// ParseCron parses a standard 5-field cron expression and returns a [Trigger].
//
// Each field supports "*", a single value, ranges "a-b", steps "*/n" and
// "a-b/n", and comma-separated lists of these. Fields and their ranges:
//
//	minute        0-59
//	hour          0-23
//	day-of-month  1-31
//	month         1-12
//	day-of-week   0-6 (Sunday=0; 7 is also accepted for Sunday)
//
// The following macros are also accepted: @yearly (@annually), @monthly,
// @weekly, @daily (@midnight) and @hourly.
//
// When both day-of-month and day-of-week are restricted, a day matches if
// *either* matches — the traditional Vixie-cron behaviour. Cron times are
// evaluated in the location of the reference time passed to Next (the scheduler
// uses the local time zone).
func ParseCron(expr string) (Trigger, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "@") {
		if m, ok := cronMacros[expr]; ok {
			expr = m
		} else {
			return nil, fmt.Errorf("scheduling: unknown cron macro %q", expr)
		}
	}

	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("scheduling: cron expression %q must have 5 fields, got %d", expr, len(fields))
	}

	var s cronSpec
	var err error
	if s.minute, _, err = parseCronField(fields[0], 0, 59, "minute"); err != nil {
		return nil, err
	}
	if s.hour, _, err = parseCronField(fields[1], 0, 23, "hour"); err != nil {
		return nil, err
	}
	if s.dom, s.domRestricted, err = parseCronField(fields[2], 1, 31, "day-of-month"); err != nil {
		return nil, err
	}
	if s.month, _, err = parseCronField(fields[3], 1, 12, "month"); err != nil {
		return nil, err
	}
	if s.dow, s.dowRestricted, err = parseCronField(fields[4], 0, 7, "day-of-week"); err != nil {
		return nil, err
	}
	// Normalize Sunday: 7 -> 0 so day matching only consults bits 0..6.
	if s.dow&(1<<7) != 0 {
		s.dow = (s.dow &^ (1 << 7)) | 1
	}

	return &s, nil
}

var cronMacros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

// parseCronField parses one field into a bitmask. restricted reports whether the
// field was anything other than a bare "*".
func parseCronField(field string, min, max int, name string) (mask uint64, restricted bool, err error) {
	restricted = field != "*"
	for part := range strings.SplitSeq(field, ",") {
		lo, hi, step, perr := parseCronPart(part, min, max)
		if perr != nil {
			return 0, false, fmt.Errorf("scheduling: cron %s field %q: %w", name, field, perr)
		}
		for v := lo; v <= hi; v += step {
			mask |= 1 << uint(v)
		}
	}
	return mask, restricted, nil
}

// parseCronPart parses a single comma-separated component: "*", "*/n", "a",
// "a-b" or "a-b/n". It returns the inclusive [lo, hi] range and the step.
func parseCronPart(part string, min, max int) (lo, hi, step int, err error) {
	step = 1
	rangePart := part
	if before, after, ok := strings.Cut(part, "/"); ok {
		rangePart = before
		step, err = strconv.Atoi(after)
		if err != nil || step <= 0 {
			return 0, 0, 0, fmt.Errorf("invalid step %q", part)
		}
	}

	switch {
	case rangePart == "*":
		lo, hi = min, max
	case strings.IndexByte(rangePart, '-') >= 0:
		before, after, _ := strings.Cut(rangePart, "-")
		lo, err = strconv.Atoi(before)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid range %q", part)
		}
		hi, err = strconv.Atoi(after)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid range %q", part)
		}
	default:
		lo, err = strconv.Atoi(rangePart)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid value %q", part)
		}
		hi = lo
	}

	if lo < min || hi > max || lo > hi {
		return 0, 0, 0, fmt.Errorf("value out of range [%d,%d] in %q", min, max, part)
	}
	return lo, hi, step, nil
}

// Next implements [Trigger]. It returns the next matching minute strictly after
// the later of tc.Now and tc.LastScheduled, or the zero time if no time within a
// five-year horizon matches (an impossible expression such as Feb 30).
func (s *cronSpec) Next(tc TriggerContext) time.Time {
	ref := tc.Now
	if !tc.LastScheduled.IsZero() && !tc.LastScheduled.Before(ref) {
		ref = tc.LastScheduled
	}
	// Start at the next whole minute after ref (seconds/nanos zeroed).
	t := ref.Truncate(time.Minute).Add(time.Minute)

	limit := t.AddDate(5, 0, 0)
	for t.Before(limit) {
		if s.month&(1<<uint(t.Month())) == 0 {
			// Advance to the first day of the next month.
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
			continue
		}
		if !s.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
			continue
		}
		if s.hour&(1<<uint(t.Hour())) == 0 {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).Add(time.Hour)
			continue
		}
		if s.minute&(1<<uint(t.Minute())) == 0 {
			t = t.Add(time.Minute)
			continue
		}
		return t
	}
	return time.Time{}
}

// dayMatches applies the classic day-of-month / day-of-week rule: if both fields
// are restricted a day matches when *either* matches; otherwise the restricted
// one (or both being "*") decides.
func (s *cronSpec) dayMatches(t time.Time) bool {
	domOK := s.dom&(1<<uint(t.Day())) != 0
	dowOK := s.dow&(1<<uint(t.Weekday())) != 0
	if s.domRestricted && s.dowRestricted {
		return domOK || dowOK
	}
	return domOK && dowOK
}
