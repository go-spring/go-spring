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

package loadbalance

import (
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestTrackerSuspendAndRecover(t *testing.T) {
	now := time.Unix(0, 0)
	tr := NewTracker(TrackerConfig{Threshold: 2, SuspendFor: time.Second})
	tr.now = func() time.Time { return now }

	// One failure is below threshold: still eligible.
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).False()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)

	// Second consecutive failure trips suspension.
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).True()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"b"})

	// Still cooling down before SuspendFor elapses.
	now = now.Add(500 * time.Millisecond)
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"b"})

	// Cool-down elapsed: a half-open trial admits "a" again.
	now = now.Add(600 * time.Millisecond)
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)

	// A successful trial fully restores it.
	tr.Record("a", true)
	assert.That(t, tr.Suspended("a")).False()

	// If the trial fails instead, it re-suspends for another window.
	tr.Record("a", false)
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).True()
	now = now.Add(1100 * time.Millisecond)
	tr.Allows(eps("a"))   // admit trial -> half-open
	tr.Record("a", false) // trial fails
	assert.That(t, tr.Suspended("a")).True()
}

func TestTrackerDisabled(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 0})
	tr.Record("a", false)
	tr.Record("a", false)
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).False()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)
}

func TestTrackerAdmissibleIsPure(t *testing.T) {
	// Unlike Allows, Admissible applies no fallback: with every endpoint
	// suspended it returns an empty slice and leaves the decision to the caller.
	now := time.Unix(0, 0)
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.now = func() time.Time { return now }
	tr.Record("a", false)
	tr.Record("b", false)
	assert.Slice(t, addrs(tr.Admissible(eps("a", "b")))).Length(0)
	assert.Slice(t, addrs(tr.Admissible(eps("a", "b", "c")))).Equal([]string{"c"})
}

func TestTrackerAllSuspendedFallsBack(t *testing.T) {
	// Black-holing all traffic is worse than probing a degraded instance: when
	// every endpoint is suspended, Allows returns its input unchanged.
	now := time.Unix(0, 0)
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.now = func() time.Time { return now }
	tr.Record("a", false)
	tr.Record("b", false)
	assert.That(t, tr.Suspended("a")).True()
	assert.That(t, tr.Suspended("b")).True()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"a", "b"})
}
