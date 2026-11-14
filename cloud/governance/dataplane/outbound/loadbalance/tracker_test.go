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

	"errors"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// TestEmptyObserverSkipsReports pins the "off" spelling: an [emptyObserver]
// makes every report a no-op while suspension and recovery behave identically.
func TestEmptyObserverSkipsReports(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, emptyObserver{})
		assert.That(t, err).Nil()
		assert.That(t, tr.obs == emptyObserver{}).True("observation off is the empty observer")

		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspended(tr, "a")).True()
		assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Equal([]string{"b"})

		time.Sleep(time.Minute) // cool-down elapsed: timer re-admits, no Record needed
		tr.Available(context.Background(), eps("a", "b"))
		assert.That(t, suspended(tr, "a")).False()

		tr.Close()
	})
}

func TestTrackerSuspendAndRecover(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 2, SuspendDuration: time.Second, MaxEjectionPercent: 100}, emptyObserver{})
		assert.That(t, err).Nil()

		// One failure is below threshold: still usable.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspended(tr, "a")).False()
		assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Length(2)

		// Second consecutive failure trips suspension.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspended(tr, "a")).True()
		assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Equal([]string{"b"})

		// Still cooling down before SuspendDuration elapses.
		time.Sleep(500 * time.Millisecond)
		assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Equal([]string{"b"})

		// Cool-down elapsed: the timer re-admits "a" with a clean slate — no
		// result is needed to decide recovery.
		time.Sleep(600 * time.Millisecond)
		assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Length(2)
		assert.That(t, suspended(tr, "a")).False()

		// The streak was cleared: one failure is once again below threshold.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspended(tr, "a")).False()
		// The second consecutive failure trips a fresh suspension.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspended(tr, "a")).True()
	})
}

func TestTrackerDisabled(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: -1, DialThreshold: -1}, emptyObserver{})
	assert.That(t, err).Nil()
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	assert.That(t, suspended(tr, "a")).False()
	assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Length(2)
}

func TestTrackerIgnoresOutcomesDuringCoolDown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, emptyObserver{})
		assert.That(t, err).Nil()
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspended(tr, "a")).True()

		// A success from a request admitted before the suspension must not
		// recover the endpoint: recovery is the timer's decision.
		tr.Record(context.Background(), ep("a"), nil)
		assert.That(t, suspended(tr, "a")).True()

		// After the cool-down, Available returns the endpoint to rotation by
		// itself; no result decides it.
		time.Sleep(time.Minute)
		tr.Available(context.Background(), eps("a"))
		assert.That(t, suspended(tr, "a")).False()
	})
}

func TestTrackerRemoveStaleDropsVanishedEndpoints(t *testing.T) {
	obs := &recordingObserver{}
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, obs)
	assert.That(t, err).Nil()
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	tr.Record(context.Background(), ep("b"), errors.New("boom"))
	assert.Number(t, tr.suspendedCountLocked()).Equal(2)

	tr.RemoveStale(context.Background(), eps("b", "c"))

	// "a" is gone from discovery: its suspension state and gauge share go with it.
	assert.Number(t, tr.suspendedCountLocked()).Equal(1)
	assert.Number(t, len(tr.states)).Equal(1)
	assert.Slice(t, obs.deltas).Equal([]int64{1, 1, -1}) // one +1 per suspension, one -1 for the stale drop
	assert.That(t, suspended(tr, "b")).True()
}

func TestTrackerRemoveStaleIgnoresEmptyEndpointSet(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, emptyObserver{})
	assert.That(t, err).Nil()
	tr.Record(context.Background(), ep("a"), errors.New("boom"))

	// A source that momentarily advertises nothing is no evidence that "a" is gone.
	tr.RemoveStale(context.Background(), nil)

	assert.Number(t, tr.suspendedCountLocked()).Equal(1)
}

func TestTrackerSuccessLeavesNoState(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 2}, emptyObserver{})
	assert.That(t, err).Nil()

	// A failure below threshold keeps a streak entry...
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	assert.Number(t, len(tr.states)).Equal(1)

	// ...and a success drops it, so the map stays proportional to endpoints
	// that are currently failing.
	tr.Record(context.Background(), ep("a"), nil)
	assert.Number(t, len(tr.states)).Equal(0)
	tr.Record(context.Background(), ep("a"), nil)
	assert.Number(t, len(tr.states)).Equal(0)
}

func TestTrackerRecordAfterCloseIsDropped(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, emptyObserver{})
	assert.That(t, err).Nil()
	tr.Close()

	// An in-flight request of a retired generation settles late: it must not
	// recreate state (and a gauge count nothing will ever return).
	tr.Record(context.Background(), ep("a"), errors.New("boom"))

	assert.Number(t, len(tr.states)).Equal(0)
	assert.Number(t, tr.suspendedCountLocked()).Equal(0)
}

func TestTrackerCloseDropsState(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, emptyObserver{})
	assert.That(t, err).Nil()
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	assert.Number(t, tr.suspendedCountLocked()).Equal(1)

	tr.Close()

	assert.Number(t, tr.suspendedCountLocked()).Equal(0)
	assert.Number(t, len(tr.states)).Equal(0)
}

func TestTrackerAllSuspendedFallsBack(t *testing.T) {
	// Black-holing all traffic is worse than probing a degraded instance: when
	// every endpoint is suspended, Available returns its input unchanged.
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, emptyObserver{})
	assert.That(t, err).Nil()
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	tr.Record(context.Background(), ep("b"), errors.New("boom"))
	assert.That(t, suspended(tr, "a")).True()
	assert.That(t, suspended(tr, "b")).True()
	assert.Slice(t, addrs(tr.Available(context.Background(), eps("a", "b")))).Equal([]string{"a", "b"})
}

// recordingObserver captures the events a tracker reports, so the observability
// seam is tested directly rather than through OTel.
type recordingObserver struct {
	reasons []string
	deltas  []int64
}

func (o *recordingObserver) Suspended(_ context.Context, _ string, reason string) {
	o.reasons = append(o.reasons, reason)
}

func (o *recordingObserver) SuspendedDelta(_ context.Context, _ string, delta int64) {
	o.deltas = append(o.deltas, delta)
}

func TestTrackerReportsThroughObserver(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		obs := &recordingObserver{}
		tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute}, obs)
		assert.That(t, err).Nil()

		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.Slice(t, obs.reasons).Equal([]string{"unknown_failed"})
		assert.Slice(t, obs.deltas).Equal([]int64{1})

		time.Sleep(time.Minute)
		tr.Available(context.Background(), eps("a")) // cool-down elapsed: timer returns the gauge share
		assert.Slice(t, obs.deltas).Equal([]int64{1, -1})
	})
}

// TestMinHealthyPercentDoesNotGateEjections pins the split Envoy and Istio make:
// max-ejection-percent is the ejection cap, while min-healthy-percent only
// decides when the panic path serves the whole pool. A floor alone must not
// refuse suspensions.
func TestMinHealthyPercentDoesNotGateEjections(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{
		RequestThreshold:   1,
		SuspendDuration:    time.Minute,
		MinHealthyPercent:  50,
		MaxEjectionPercent: 100,
	}, emptyObserver{})
	assert.That(t, err).Nil()

	tr.Available(context.Background(), eps("a", "b", "c", "d"))
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	tr.Record(context.Background(), ep("b"), errors.New("boom"))
	tr.Record(context.Background(), ep("c"), errors.New("boom"))

	assert.That(t, suspendedState(tr, "a")).True()
	assert.That(t, suspendedState(tr, "b")).True()
	assert.That(t, suspendedState(tr, "c")).True()
}

// TestMinHealthyPercentPanicsWhenThePoolShrinks pins the floor as a continuous
// panic threshold, not just a gate on new suspensions: when ejections outlive a
// discovery shrink and the available share drops below the floor, Available
// admits the whole candidate set again.
func TestMinHealthyPercentPanicsWhenThePoolShrinks(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{
		RequestThreshold:   1,
		SuspendDuration:    time.Minute,
		MinHealthyPercent:  50,
		MaxEjectionPercent: 100,
	}, emptyObserver{})
	assert.That(t, err).Nil()

	tr.Available(context.Background(), eps("a", "b", "c", "d"))
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	tr.Record(context.Background(), ep("b"), errors.New("boom"))

	// Discovery shrinks to the two ejected peers plus one live: 1/3 is below the
	// 50% floor, so the ejections are ignored and the whole set is admitted.
	got := tr.Available(context.Background(), eps("a", "b", "c"))
	assert.Slice(t, addrs(got)).Equal([]string{"a", "b", "c"})
}

// TestMinHealthyPercentPanicsOnLowHealthyShare pins the health side of the
// floor: too few healthy endpoints in the pool mean the whole pool is served,
// instead of concentrating traffic on the survivors.
func TestMinHealthyPercentPanicsOnLowHealthyShare(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{MinHealthyPercent: 50}, emptyObserver{})
	assert.That(t, err).Nil()

	pool := []discovery.Endpoint{
		{Addr: "h1", Healthy: true},
		{Addr: "h2", Healthy: true},
		{Addr: "u1"},
		{Addr: "u2"},
		{Addr: "u3"},
		{Addr: "u4"},
	}
	// 2/6 healthy is below the 50% floor: panic, so the unhealthy peers are
	// servable too.
	assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal(
		[]string{"h1", "h2", "u1", "u2", "u3", "u4"})
}

// TestMinHealthyPercentServesHealthyAboveTheFloor pins the non-panic path: with
// enough healthy peers, only those are servable.
func TestMinHealthyPercentServesHealthyAboveTheFloor(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{MinHealthyPercent: 50}, emptyObserver{})
	assert.That(t, err).Nil()

	pool := []discovery.Endpoint{
		{Addr: "h1", Healthy: true},
		{Addr: "h2", Healthy: true},
		{Addr: "h3", Healthy: true},
		{Addr: "h4", Healthy: true},
		{Addr: "u1"},
		{Addr: "u2"},
	}
	// 4/6 healthy is above the floor: only healthy endpoints are servable.
	assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal(
		[]string{"h1", "h2", "h3", "h4"})
}

// TestMinHealthyPercentPanicsWhenAllHealthyAreSuspended pins that "no healthy
// available" is a panic when the POOL reports health: the suspended healthy
// peers are served again instead of the pool silently degrading to its
// unhealthy members.
func TestMinHealthyPercentPanicsWhenAllHealthyAreSuspended(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{
		RequestThreshold:   1,
		SuspendDuration:    time.Minute,
		MinHealthyPercent:  50,
		MaxEjectionPercent: 100,
	}, emptyObserver{})
	assert.That(t, err).Nil()

	pool := []discovery.Endpoint{
		{Addr: "h1", Healthy: true},
		{Addr: "h2", Healthy: true},
		{Addr: "u1"},
		{Addr: "u2"},
		{Addr: "u3"},
		{Addr: "u4"},
	}
	tr.Available(context.Background(), pool)
	tr.Record(context.Background(), ep("h1"), errors.New("boom"))
	tr.Record(context.Background(), ep("h2"), errors.New("boom"))

	// Both healthy peers are cooling down: servable is 0/6, so the floor panics
	// and the whole pool (including the suspended healthy peers) is served.
	assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal(
		[]string{"h1", "h2", "u1", "u2", "u3", "u4"})
}

// TestMinHealthyPercentZeroAllowsSuspension pins 0 as "no floor".
func TestMinHealthyPercentZeroAllowsSuspension(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute, MaxEjectionPercent: 100}, emptyObserver{})
	assert.That(t, err).Nil()

	tr.Available(context.Background(), eps("a"))
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	assert.That(t, suspendedState(tr, "a")).True()
}

// TestMinHealthyPercentOutOfRangeRejected pins the construction boundary.
func TestMinHealthyPercentOutOfRangeRejected(t *testing.T) {
	_, err := NewTracker("svc", TrackerConfig{MinHealthyPercent: -1}, emptyObserver{})
	assert.Error(t, err).NotNil()
	_, err = NewTracker("svc", TrackerConfig{MinHealthyPercent: 101}, emptyObserver{})
	assert.Error(t, err).NotNil()
}

// TestMaxEjectionPercentCapsEjections pins Envoy's max_ejection_percent rule:
// with a 10-endpoint pool and a 10% cap, one ejection is allowed and the second
// is refused.
func TestMaxEjectionPercentCapsEjections(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{
		RequestThreshold:   1,
		SuspendDuration:    time.Minute,
		MaxEjectionPercent: 10,
	}, emptyObserver{})
	assert.That(t, err).Nil()

	tr.Available(context.Background(), eps("a", "b", "c", "d", "e", "f", "g", "h", "i", "j"))
	tr.Record(context.Background(), ep("a"), errors.New("boom")) // 1/10: at the cap
	tr.Record(context.Background(), ep("b"), errors.New("boom")) // over it

	assert.That(t, suspendedState(tr, "a")).True()
	assert.That(t, suspendedState(tr, "b")).False()
}

// TestMaxEjectionPercentRefusesSmallPool pins the default Envoy/Istio rule: the
// percentage is a real cap, and the at-least-one carve-out is opt-in upstream
// (`always_eject_one_host`, off by default), so a 3-endpoint pool at 10% has a
// zero budget. A small pool opts in by raising the percentage.
func TestMaxEjectionPercentRefusesSmallPool(t *testing.T) {
	tr, err := NewTracker("svc", TrackerConfig{
		RequestThreshold:   1,
		SuspendDuration:    time.Minute,
		MaxEjectionPercent: 10,
	}, emptyObserver{})
	assert.That(t, err).Nil()

	tr.Available(context.Background(), eps("a", "b", "c"))
	tr.Record(context.Background(), ep("a"), errors.New("boom"))
	assert.That(t, suspendedState(tr, "a")).False()
}

// TestMaxEjectionPercentOutOfRangeRejected pins the construction boundary.
func TestMaxEjectionPercentOutOfRangeRejected(t *testing.T) {
	_, err := NewTracker("svc", TrackerConfig{MaxEjectionPercent: -1}, emptyObserver{})
	assert.Error(t, err).NotNil()
	_, err = NewTracker("svc", TrackerConfig{MaxEjectionPercent: 101}, emptyObserver{})
	assert.Error(t, err).NotNil()
}

// TestConsecutiveFailuresAreNotTimeBounded pins Envoy's counting rule: the
// streak is CONSECUTIVE, so only a success clears it. Failures spread over time
// still add up — Envoy's `interval` is the sweep cadence, not a counting window,
// and this event-driven tracker has no sweep.
func TestConsecutiveFailuresAreNotTimeBounded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, err := NewTracker("svc", TrackerConfig{
			RequestThreshold:   3,
			SuspendDuration:    time.Minute,
			MaxEjectionPercent: 100,
		}, emptyObserver{})
		assert.That(t, err).Nil()

		tr.Available(context.Background(), eps("a", "b"))
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		time.Sleep(time.Minute)
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspendedState(tr, "a")).False() // two, below threshold

		time.Sleep(time.Hour) // no success in between: the streak survives
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspendedState(tr, "a")).True()

		// Re-admit, then show a success starts a fresh streak.
		time.Sleep(time.Minute)
		tr.Available(context.Background(), eps("a", "b"))
		tr.Record(context.Background(), ep("a"), nil)
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		assert.That(t, suspendedState(tr, "a")).False() // two since the success
	})
}

// TestEjectionTimeEscalates pins base × ejection count, capped: the second
// ejection keeps the endpoint out longer than the base time, and the cap bounds
// further growth.
func TestEjectionTimeEscalates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, err := NewTracker("svc", TrackerConfig{
			RequestThreshold:   1,
			SuspendDuration:    10 * time.Second,
			MaxSuspendDuration: 15 * time.Second,
			MaxEjectionPercent: 100,
		}, emptyObserver{})
		assert.That(t, err).Nil()

		pool := eps("a", "b")
		tr.Available(context.Background(), pool)

		// First ejection: the base 10s.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		time.Sleep(11 * time.Second)
		assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal([]string{"a", "b"})

		// Second ejection: 2 × 10s, capped at 15s.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		time.Sleep(14 * time.Second)
		assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal([]string{"b"}) // still out
		time.Sleep(2 * time.Second)
		assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal([]string{"a", "b"}) // back
	})
}

// TestEjectionBackoffDecaysWhileHealthy pins Envoy's sweep-time decay: an
// endpoint that stays in rotation for a full Interval loses one step of
// backoff, so a recovered instance is not punished forever.
func TestEjectionBackoffDecaysWhileHealthy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tr, err := NewTracker("svc", TrackerConfig{
			RequestThreshold:   1,
			SuspendDuration:    10 * time.Second,
			MaxSuspendDuration: time.Minute,
			Interval:           10 * time.Second,
			MaxEjectionPercent: 100,
		}, emptyObserver{})
		assert.That(t, err).Nil()
		pool := eps("a", "b")
		tr.Available(context.Background(), pool)

		// First ejection lasts the base 10s.
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		time.Sleep(11 * time.Second)
		assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal([]string{"a", "b"})

		time.Sleep(10 * time.Second) // one full interval in rotation
		tr.Record(context.Background(), ep("a"), errors.New("boom"))
		// The backoff decayed before this ejection: it is the base 10s again, not
		// an escalated 20s.
		time.Sleep(11 * time.Second)
		assert.Slice(t, addrs(tr.Available(context.Background(), pool))).Equal([]string{"a", "b"})
	})
}
