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

func TestFixedRateAnchorsOnScheduled(t *testing.T) {
	tr := scheduling.FixedRate(10 * time.Second)
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)

	// First fire is one interval after now.
	first := tr.Next(scheduling.TriggerContext{Now: base})
	assert.That(t, first).Equal(base.Add(10 * time.Second))

	// Subsequent fires anchor on LastScheduled, independent of when the run
	// finished (fixed rate). Completion at +100s must not shift the cadence.
	next := tr.Next(scheduling.TriggerContext{
		Now:            base.Add(11 * time.Second),
		LastScheduled:  first,
		LastCompletion: base.Add(100 * time.Second),
	})
	assert.That(t, next).Equal(first.Add(10 * time.Second))
}

func TestFixedRateSkipsMissedSlots(t *testing.T) {
	tr := scheduling.FixedRate(10 * time.Second)
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	last := base

	// A slow run pushed Now 35s past the last scheduled fire. The next fire must
	// be strictly in the future and aligned to the cadence: base+40s.
	next := tr.Next(scheduling.TriggerContext{Now: base.Add(35 * time.Second), LastScheduled: last})
	assert.That(t, next).Equal(base.Add(40 * time.Second))
}

func TestFixedDelayAnchorsOnCompletion(t *testing.T) {
	tr := scheduling.FixedDelay(10 * time.Second)
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)

	// First fire is one interval after now.
	first := tr.Next(scheduling.TriggerContext{Now: base})
	assert.That(t, first).Equal(base.Add(10 * time.Second))

	// Subsequent fires measure from LastCompletion, not LastScheduled: a run that
	// finished at +50s means the next fire is +60s.
	next := tr.Next(scheduling.TriggerContext{
		Now:            base.Add(50 * time.Second),
		LastScheduled:  first,
		LastCompletion: base.Add(50 * time.Second),
	})
	assert.That(t, next).Equal(base.Add(60 * time.Second))
}

func TestFixedRateAndDelayPanicOnNonPositive(t *testing.T) {
	assert.Panic(t, func() { scheduling.FixedRate(0) }, "positive")
	assert.Panic(t, func() { scheduling.FixedDelay(-1) }, "positive")
}

func TestFixedRateInitialDelay(t *testing.T) {
	tr := scheduling.FixedRate(10*time.Second, scheduling.WithInitialDelay(30*time.Second))
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)

	// The first fire waits out the initial delay instead of one interval.
	first := tr.Next(scheduling.TriggerContext{Now: base})
	assert.That(t, first).Equal(base.Add(30 * time.Second))

	// Later fires follow the normal cadence, anchored on LastScheduled.
	next := tr.Next(scheduling.TriggerContext{
		Now:           first.Add(time.Second),
		LastScheduled: first,
	})
	assert.That(t, next).Equal(first.Add(10 * time.Second))
}

func TestFixedDelayInitialDelay(t *testing.T) {
	tr := scheduling.FixedDelay(10*time.Second, scheduling.WithInitialDelay(30*time.Second))
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)

	first := tr.Next(scheduling.TriggerContext{Now: base})
	assert.That(t, first).Equal(base.Add(30 * time.Second))

	// Later fires measure from LastCompletion as usual.
	next := tr.Next(scheduling.TriggerContext{
		Now:            base.Add(60 * time.Second),
		LastScheduled:  first,
		LastCompletion: base.Add(60 * time.Second),
	})
	assert.That(t, next).Equal(base.Add(70 * time.Second))
}

func TestAfterFiresOnce(t *testing.T) {
	tr := scheduling.After(5 * time.Second)
	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)

	// The single fire is d after now.
	first := tr.Next(scheduling.TriggerContext{Now: base})
	assert.That(t, first).Equal(base.Add(5 * time.Second))

	// Once it has fired (LastScheduled set), the trigger reports no next time.
	next := tr.Next(scheduling.TriggerContext{
		Now:           first.Add(time.Second),
		LastScheduled: first,
	})
	assert.That(t, next).Equal(time.Time{})
}

func TestAfterAndInitialDelayPanicOnNonPositive(t *testing.T) {
	assert.Panic(t, func() { scheduling.After(0) }, "positive")
	assert.Panic(t, func() { scheduling.FixedRate(10*time.Second, scheduling.WithInitialDelay(-1)) }, "positive")
}

func TestFixedRateJitterDelaysWithinBound(t *testing.T) {
	tr := scheduling.FixedRate(10*time.Second, scheduling.WithJitter(5*time.Second))
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	// The first fire lands somewhere in [d, d+jitter) — never earlier than d,
	// never later than d+jitter.
	for i := 0; i < 100; i++ {
		first := tr.Next(scheduling.TriggerContext{Now: base})
		assert.That(t, !first.Before(base.Add(10*time.Second))).True()
		assert.That(t, first.Before(base.Add(15*time.Second))).True()
	}

	// Subsequent fires are jittered too, on top of the anchored slot.
	for i := 0; i < 100; i++ {
		last := base.Add(10 * time.Second)
		next := tr.Next(scheduling.TriggerContext{Now: base.Add(11 * time.Second), LastScheduled: last})
		assert.That(t, !next.Before(last.Add(10*time.Second))).True()
		assert.That(t, next.Before(last.Add(15*time.Second))).True()
	}
}

func TestFixedDelayJitterDelaysWithinBound(t *testing.T) {
	tr := scheduling.FixedDelay(10*time.Second, scheduling.WithJitter(5*time.Second))
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	done := base.Add(50 * time.Second)

	for i := 0; i < 100; i++ {
		next := tr.Next(scheduling.TriggerContext{
			Now:            done,
			LastScheduled:  base.Add(40 * time.Second),
			LastCompletion: done,
		})
		assert.That(t, !next.Before(done.Add(10*time.Second))).True()
		assert.That(t, next.Before(done.Add(15*time.Second))).True()
	}
}

func TestWithJitterPanicOnNonPositive(t *testing.T) {
	assert.Panic(t, func() { scheduling.FixedRate(10*time.Second, scheduling.WithJitter(0)) }, "positive")
}
