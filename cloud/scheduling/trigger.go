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
	"math/rand/v2"
	"time"

	"go-spring.org/stdlib/errutil"
)

// TriggerOption customizes a fixed-interval [Trigger] at construction.
type TriggerOption func(*triggerOpts)

// triggerOpts carries the customization knobs shared by the fixed-interval
// triggers; a zero value means "no customization".
type triggerOpts struct {
	initial time.Duration // the delay before the first fire, if any
	jitter  time.Duration // the upper bound of each fire's random delay
}

// WithInitialDelay delays the first fire by d after the schedule starts; later
// fires follow the trigger's normal cadence. Useful to let warm-up finish
// before a recurring job starts ticking. When [WithJitter] is also passed, the
// first fire is jittered like any other.
//
// It panics if d is not positive — a non-positive delay is a no-op at best and
// a typo at worst, so it is refused rather than silently ignored.
func WithInitialDelay(d time.Duration) TriggerOption {
	if d <= 0 {
		panic(errutil.Explain(nil, "scheduling: WithInitialDelay requires a positive duration"))
	}
	return func(o *triggerOpts) { o.initial = d }
}

// WithJitter delays each fire by a uniform random duration in [0, d), on top of
// the trigger's own timing. Useful when many jobs (or replicas of one job)
// share a cadence and firing in lockstep would stampede a downstream — a cache
// expiring at :00, a database hammered on the minute mark.
//
// Jitter only ever delays a fire, never advances it, but the delay feeds the
// next fire's anchor (the previous fire time), so the long-run average interval
// is d plus half the jitter, not d. Keep jitter small relative to the interval
// when the average cadence matters.
//
// It panics if d is not positive.
func WithJitter(d time.Duration) TriggerOption {
	if d <= 0 {
		panic(errutil.Explain(nil, "scheduling: WithJitter requires a positive duration"))
	}
	return func(o *triggerOpts) { o.jitter = d }
}

// jitterDelay returns a uniform random duration in [0, d) — the delay to add to
// a computed fire time; 0 when no jitter is configured.
func jitterDelay(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return rand.N(d)
}

// FixedRate returns a [Trigger] that fires every d, measured from each scheduled
// fire time rather than from when a run finishes. The gap between fires stays d
// even if a run takes a while, so runs can overlap; use a [ConcurrencyPolicy] to
// control that. The first fire is one interval after the schedule starts, or
// [WithInitialDelay]'s duration after it when that option is passed.
//
// It panics if d is not positive, since a non-positive rate could never produce
// a sensible schedule.
func FixedRate(d time.Duration, opts ...TriggerOption) Trigger {
	if d <= 0 {
		panic(errutil.Explain(nil, "scheduling: FixedRate requires a positive duration"))
	}
	var o triggerOpts
	for _, opt := range opts {
		opt(&o)
	}
	return fixedRate{d: d, initial: o.initial, jitter: o.jitter}
}

// fixedRate is the [FixedRate] trigger; d is the fixed period between fires,
// initial, when set, replaces the first interval, and jitter, when set, delays
// each fire by a random duration.
type fixedRate struct {
	d       time.Duration
	initial time.Duration
	jitter  time.Duration
}

// Next implements [Trigger].
func (f fixedRate) Next(tc TriggerContext) time.Time {
	if tc.LastScheduled.IsZero() {
		first := f.d
		if f.initial > 0 {
			first = f.initial
		}
		return tc.Now.Add(first + jitterDelay(f.jitter))
	}
	next := tc.LastScheduled.Add(f.d)
	// If runs fell behind (a slow run pushed us past several intervals), skip the
	// missed slots and schedule the next one strictly in the future, so a backlog
	// does not cause a burst of catch-up fires.
	if !next.After(tc.Now) {
		missed := tc.Now.Sub(tc.LastScheduled) / f.d
		next = tc.LastScheduled.Add((missed + 1) * f.d)
	}
	return next.Add(jitterDelay(f.jitter))
}

// FixedDelay returns a [Trigger] that fires d after the previous run finishes,
// so two runs never overlap regardless of how long each takes. The first fire is
// one interval after the schedule starts, or [WithInitialDelay]'s duration
// after it when that option is passed.
//
// It panics if d is not positive.
func FixedDelay(d time.Duration, opts ...TriggerOption) Trigger {
	if d <= 0 {
		panic(errutil.Explain(nil, "scheduling: FixedDelay requires a positive duration"))
	}
	var o triggerOpts
	for _, opt := range opts {
		opt(&o)
	}
	return fixedDelay{d: d, initial: o.initial, jitter: o.jitter}
}

// fixedDelay is the [FixedDelay] trigger; d is the gap measured from the end of
// each run, initial, when set, replaces the first interval, and jitter, when
// set, delays each fire by a random duration.
type fixedDelay struct {
	d       time.Duration
	initial time.Duration
	jitter  time.Duration
}

// Next implements [Trigger].
func (f fixedDelay) Next(tc TriggerContext) time.Time {
	if tc.LastCompletion.IsZero() {
		first := f.d
		if f.initial > 0 {
			first = f.initial
		}
		return tc.Now.Add(first + jitterDelay(f.jitter))
	}
	return tc.LastCompletion.Add(f.d + jitterDelay(f.jitter))
}

// After returns a [Trigger] that fires exactly once, d after the schedule
// starts. Once that fire has run the trigger returns the zero time, which stops
// the job's loop. Use it for "do this once, a bit later" — deferred
// initialization, a single retry, a delayed cleanup.
//
// It panics if d is not positive; use a plain function call for "run now".
func After(d time.Duration) Trigger {
	if d <= 0 {
		panic(errutil.Explain(nil, "scheduling: After requires a positive duration"))
	}
	return after{d: d}
}

// after is the [After] trigger; d is the delay before its single fire. It is
// stateless: LastScheduled being set means the one fire already happened.
type after struct{ d time.Duration }

// Next implements [Trigger].
func (a after) Next(tc TriggerContext) time.Time {
	if tc.LastScheduled.IsZero() {
		return tc.Now.Add(a.d)
	}
	return time.Time{}
}
