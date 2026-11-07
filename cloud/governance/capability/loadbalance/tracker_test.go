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
	"context"
	"testing"
	"testing/synctest"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestTrackerSuspendAndRecover(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := NewTracker(TrackerConfig{Threshold: 2, SuspendFor: time.Second})

		// One failure is below threshold: still eligible.
		tr.Record(context.Background(), "a", false)
		assert.That(t, suspended(tr, "a")).False()
		assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)

		// Second consecutive failure trips suspension.
		tr.Record(context.Background(), "a", false)
		assert.That(t, suspended(tr, "a")).True()
		assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"b"})

		// Still cooling down before SuspendFor elapses.
		time.Sleep(500 * time.Millisecond)
		assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"b"})

		// Cool-down elapsed: a half-open trial admits "a" again.
		time.Sleep(600 * time.Millisecond)
		assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)

		// A successful trial fully restores it.
		tr.Record(context.Background(), "a", true)
		assert.That(t, suspended(tr, "a")).False()

		// If the trial fails instead, it re-suspends for another window.
		tr.Record(context.Background(), "a", false)
		tr.Record(context.Background(), "a", false)
		assert.That(t, suspended(tr, "a")).True()
		time.Sleep(1100 * time.Millisecond)
		tr.Allows(eps("a"))                         // admit trial -> half-open
		tr.Record(context.Background(), "a", false) // trial fails
		assert.That(t, suspended(tr, "a")).True()
	})
}

func TestTrackerDisabled(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 0})
	tr.Record(context.Background(), "a", false)
	tr.Record(context.Background(), "a", false)
	tr.Record(context.Background(), "a", false)
	assert.That(t, suspended(tr, "a")).False()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)
}

func TestTrackerIgnoresOutcomesDuringCoolDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
		tr.Record(context.Background(), "a", false)
		assert.That(t, suspended(tr, "a")).True()

		// A success from a request admitted before the suspension must not close
		// the circuit: the endpoint receives no new traffic while cooling down.
		tr.Record(context.Background(), "a", true)
		assert.That(t, suspended(tr, "a")).True()

		// After the cool-down, Allows admits the half-open trial; its success is
		// what recovers the endpoint.
		time.Sleep(time.Minute)
		tr.Allows(eps("a"))
		tr.Record(context.Background(), "a", true)
		assert.That(t, suspended(tr, "a")).False()
	})
}

func TestTrackerPruneDropsVanishedEndpoints(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.Record(context.Background(), "a", false)
	tr.Record(context.Background(), "b", false)
	assert.Number(t, tr.suspendedCount()).Equal(2)

	tr.Prune(eps("b", "c"))

	// "a" is gone from discovery: its suspension state and gauge share go with it.
	assert.Number(t, tr.suspendedCount()).Equal(1)
	assert.Number(t, len(tr.states)).Equal(1)
	assert.That(t, suspended(tr, "b")).True()
}

func TestTrackerPruneIgnoresEmptyEndpointSet(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.Record(context.Background(), "a", false)

	// A source that momentarily advertises nothing is no evidence that "a" is gone.
	tr.Prune(nil)

	assert.Number(t, tr.suspendedCount()).Equal(1)
}

func TestTrackerSuccessLeavesNoState(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 1})
	tr.Record(context.Background(), "a", true)
	tr.Record(context.Background(), "a", true)
	assert.Number(t, len(tr.states)).Equal(0)
}

func TestTrackerRecordAfterCloseIsDropped(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.Close()

	// An in-flight request of a retired generation settles late: it must not
	// recreate state (and a gauge count nothing will ever return).
	tr.Record(context.Background(), "a", false)

	assert.Number(t, len(tr.states)).Equal(0)
	assert.Number(t, tr.suspendedCount()).Equal(0)
}

func TestTrackerCloseDropsState(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.Record(context.Background(), "a", false)
	assert.Number(t, tr.suspendedCount()).Equal(1)

	tr.Close()

	assert.Number(t, tr.suspendedCount()).Equal(0)
	assert.Number(t, len(tr.states)).Equal(0)
}

func TestTrackerAllSuspendedFallsBack(t *testing.T) {
	// Black-holing all traffic is worse than probing a degraded instance: when
	// every endpoint is suspended, Allows returns its input unchanged.
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.Record(context.Background(), "a", false)
	tr.Record(context.Background(), "b", false)
	assert.That(t, suspended(tr, "a")).True()
	assert.That(t, suspended(tr, "b")).True()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"a", "b"})
}
