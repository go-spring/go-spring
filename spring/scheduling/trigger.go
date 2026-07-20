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

import "time"

// FixedRate returns a [Trigger] that fires every d, measured from each scheduled
// fire time rather than from when a run finishes. The gap between fires stays d
// even if a run takes a while, so runs can overlap; use a [ConcurrencyPolicy] to
// control that. The first fire is one interval after the schedule starts.
//
// It panics if d is not positive, since a non-positive rate could never produce
// a sensible schedule.
func FixedRate(d time.Duration) Trigger {
	if d <= 0 {
		panic("scheduling: FixedRate requires a positive duration")
	}
	return fixedRate{d: d}
}

type fixedRate struct{ d time.Duration }

func (f fixedRate) Next(tc TriggerContext) time.Time {
	if tc.LastScheduled.IsZero() {
		return tc.Now.Add(f.d)
	}
	next := tc.LastScheduled.Add(f.d)
	// If runs fell behind (a slow run pushed us past several intervals), skip the
	// missed slots and schedule the next one strictly in the future, so a backlog
	// does not cause a burst of catch-up fires.
	if !next.After(tc.Now) {
		missed := tc.Now.Sub(tc.LastScheduled) / f.d
		next = tc.LastScheduled.Add((missed + 1) * f.d)
	}
	return next
}

// FixedDelay returns a [Trigger] that fires d after the previous run finishes,
// so two runs never overlap regardless of how long each takes. The first fire is
// one interval after the schedule starts.
//
// It panics if d is not positive.
func FixedDelay(d time.Duration) Trigger {
	if d <= 0 {
		panic("scheduling: FixedDelay requires a positive duration")
	}
	return fixedDelay{d: d}
}

type fixedDelay struct{ d time.Duration }

func (f fixedDelay) Next(tc TriggerContext) time.Time {
	if tc.LastCompletion.IsZero() {
		return tc.Now.Add(f.d)
	}
	return tc.LastCompletion.Add(f.d)
}
