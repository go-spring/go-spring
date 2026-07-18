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

	"go-spring.org/stdlib/scheduling"
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
