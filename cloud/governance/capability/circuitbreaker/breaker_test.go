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

package circuitbreaker_test

import (
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
)

func newConsecutive(t *testing.T, openFor time.Duration) *circuitbreaker.Breaker {
	t.Helper()
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 2,
		OpenDuration:     openFor,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// allow admits one call and returns its permit, failing the test on refusal.
func allow(t *testing.T, b *circuitbreaker.Breaker) circuitbreaker.Permit {
	t.Helper()
	p, ok := b.Allow()
	if !ok {
		t.Fatal("breaker refused an expected admission")
	}
	return p
}

func assertAdmits(t *testing.T, b *circuitbreaker.Breaker) {
	t.Helper()
	if _, ok := b.Allow(); !ok {
		t.Fatal("breaker refused but should admit")
	}
}

func assertRefuses(t *testing.T, b *circuitbreaker.Breaker) {
	t.Helper()
	if _, ok := b.Allow(); ok {
		t.Fatal("breaker admitted but should refuse")
	}
}

// A straggler — a request admitted before the trip that succeeds after it —
// must not close the circuit: its permit predates the Open state, so Record
// drops it. Only a half-open trial may close the circuit.
func TestStragglerSuccessDoesNotClose(t *testing.T) {
	b := newConsecutive(t, 40*time.Millisecond)
	b.Record(allow(t, b), false, 0)
	straggler := allow(t, b)        // admitted in Closed, still in flight
	b.Record(allow(t, b), false, 0) // trips: Open

	b.Record(straggler, true, 0) // stale permit: ignored
	assertRefuses(t, b)

	time.Sleep(60 * time.Millisecond) // cool-down elapses
	trial := allow(t, b)              // half-open trial admitted
	b.Record(trial, true, 0)          // the trial succeeds
	assertAdmits(t, b)
}

// A straggler failure must not refresh the cool-down: the re-open time is
// decided by the trip, not by responses that were already in flight.
func TestStragglerFailureDoesNotExtendCooldown(t *testing.T) {
	b := newConsecutive(t, 40*time.Millisecond)
	b.Record(allow(t, b), false, 0)
	straggler := allow(t, b)
	b.Record(allow(t, b), false, 0) // trips at T0

	time.Sleep(30 * time.Millisecond)
	b.Record(straggler, false, 0) // straggler failure at T0+30ms: stale, ignored

	time.Sleep(20 * time.Millisecond) // T0+50ms > T0+40ms cool-down
	allow(t, b)
}

// By default a half-open round has no deadline: it waits for its trials, and
// low traffic simply keeps it half-open with every arriving call admitted as
// another trial (the resilience4j/gobreaker behavior).
func TestHalfOpenWaitsForItsTrials(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 2,
		HalfOpenTrials:   2,
		OpenDuration:     40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0) // trips

	time.Sleep(60 * time.Millisecond)
	allow(t, b) // first trial, never recorded
	time.Sleep(60 * time.Millisecond)
	allow(t, b) // no deadline: the second trial is still admitted
}

// An ignored permit carries no verdict: it is neither a success nor a failure,
// so it must not dilute the ratio that later real samples produce.
func TestRecordIgnoredAddsNoSample(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:     circuitbreaker.StrategyErrorRate,
		FailureRate:  0.5,
		MinSamples:   2,
		Window:       time.Minute,
		OpenDuration: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	for range 10 {
		b.RecordIgnored(allow(t, b))
	}
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0)
	assertRefuses(t, b) // 2/2 failures, not 2/12
}

// In half-open an ignored permit goes back to the round instead of deciding it:
// a probe without a verdict must neither close nor re-open the circuit.
func TestRecordIgnoredReturnsHalfOpenPermit(t *testing.T) {
	b := newConsecutive(t, 30*time.Millisecond)
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0) // trips

	time.Sleep(40 * time.Millisecond)
	b.RecordIgnored(allow(t, b)) // the half-open trial has no verdict
	if got := b.Stats().State; got != circuitbreaker.StateHalfOpen {
		t.Fatalf("ignored trial changed state to %v", got)
	}

	p := allow(t, b)      // the permit came back, so this is the next trial
	b.Record(p, false, 0) // a real verdict re-opens
	assertRefuses(t, b)
}

// Half-open admits at most HalfOpenTrials probes: once the permits are taken,
// further calls are refused until a verdict arrives or the optional timeout
// re-opens the round.
func TestHalfOpenRefusesWhenTrialsSpent(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 1,
		HalfOpenTrials:   1,
		OpenDuration:     30 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Record(allow(t, b), false, 0) // trips

	time.Sleep(40 * time.Millisecond)
	allow(t, b) // the one trial permit
	assertRefuses(t, b)
}

// An ignored permit is still subject to the generation check: the zero permit
// and a permit from an earlier state change nothing.
func TestRecordIgnoredIgnoresZeroAndStalePermits(t *testing.T) {
	b := newConsecutive(t, time.Minute) // trips at two consecutive failures
	b.RecordIgnored(circuitbreaker.Permit{})
	straggler := allow(t, b)
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0) // trips

	b.RecordIgnored(straggler) // stale: must not decide anything
	assertRefuses(t, b)
}

// Closing the circuit resets the counters, so old failures cannot mix into the
// fresh Closed state's evidence.
func TestRecoveryResetsCounters(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:     circuitbreaker.StrategyErrorRate,
		FailureRate:  0.5,
		MinSamples:   1,
		Window:       time.Minute,
		OpenDuration: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Record(allow(t, b), false, 0) // 1/1 failure trips
	time.Sleep(30 * time.Millisecond)
	b.Record(allow(t, b), true, 0) // trial closes the circuit and resets it
	b.Record(allow(t, b), true, 0) // a fresh sample; the old failure must be gone

	if got := b.Stats().State; got != circuitbreaker.StateClosed {
		t.Fatalf("state after a fresh success = %v, want closed", got)
	}
}

// Each windowed strategy trips on its own signal; pinning them here keeps the
// kernel's counting honest without going through an engine's policy mapping.
func TestWindowedStrategiesTrip(t *testing.T) {
	newBreaker := func(t *testing.T, spec circuitbreaker.Spec) *circuitbreaker.Breaker {
		t.Helper()
		b, err := circuitbreaker.New(spec)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("error-rate", func(t *testing.T) {
		b := newBreaker(t, circuitbreaker.Spec{
			Strategy:     circuitbreaker.StrategyErrorRate,
			FailureRate:  0.5,
			MinSamples:   2,
			Window:       time.Minute,
			OpenDuration: time.Hour,
		})
		b.Record(allow(t, b), true, 0)
		b.Record(allow(t, b), false, 0) // 1/2 failures reaches the ratio
		assertRefuses(t, b)
	})

	t.Run("error-rate slow pair", func(t *testing.T) {
		b := newBreaker(t, circuitbreaker.Spec{
			Strategy:      circuitbreaker.StrategyErrorRate,
			FailureRate:   0.9, // failures alone must not trip
			SlowThreshold: 10 * time.Millisecond,
			SlowRate:      0.5,
			MinSamples:    2,
			Window:        time.Minute,
			OpenDuration:  time.Hour,
		})
		b.Record(allow(t, b), true, 0)
		// A slow FAILURE feeds the slow arm too: the error ratio (0.5) is below
		// its 0.9 threshold, so only the slow ratio can trip.
		b.Record(allow(t, b), false, 20*time.Millisecond)
		assertRefuses(t, b)
	})

	t.Run("slow-call", func(t *testing.T) {
		b := newBreaker(t, circuitbreaker.Spec{
			Strategy:      circuitbreaker.StrategySlowCall,
			SlowThreshold: 10 * time.Millisecond,
			SlowRate:      0.5,
			MinSamples:    2,
			Window:        time.Minute,
			OpenDuration:  time.Hour,
		})
		b.Record(allow(t, b), true, 0)
		b.Record(allow(t, b), true, 20*time.Millisecond) // slow 1/2 trips
		assertRefuses(t, b)
	})

	t.Run("error-count", func(t *testing.T) {
		b := newBreaker(t, circuitbreaker.Spec{
			Strategy:         circuitbreaker.StrategyErrorCount,
			FailureThreshold: 2,
			MinSamples:       2,
			Window:           time.Minute,
			OpenDuration:     time.Hour,
		})
		b.Record(allow(t, b), true, 0)
		b.Record(allow(t, b), false, 0) // 1 failure: below the count
		b.Record(allow(t, b), false, 0) // 2 failures: trips
		assertRefuses(t, b)
	})
}

// The slow threshold is strict, matching resilience4j and Sentinel: a call
// exactly at the threshold is not slow.
func TestSlowThresholdIsStrict(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:      circuitbreaker.StrategySlowCall,
		SlowThreshold: 10 * time.Millisecond,
		SlowRate:      0.5,
		MinSamples:    2,
		Window:        time.Minute,
		OpenDuration:  time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Record(allow(t, b), true, 10*time.Millisecond)
	b.Record(allow(t, b), true, 10*time.Millisecond)
	assertAdmits(t, b) // slow ratio 0/2: equality is not slow
}

// HalfOpenTimeout is an opt-in safety valve: a round that outlives it re-opens
// the circuit instead of pinning it half-open forever.
func TestHalfOpenTimeoutReopensUnsettledRound(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 2,
		HalfOpenTrials:   2,
		HalfOpenTimeout:  40 * time.Millisecond,
		OpenDuration:     40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0) // trips

	time.Sleep(60 * time.Millisecond)
	allow(t, b) // the trial never records
	time.Sleep(60 * time.Millisecond)
	assertRefuses(t, b) // timeout re-opens and restarts the cool-down
	time.Sleep(60 * time.Millisecond)
	allow(t, b) // a fresh round starts after the cool-down
}

// Low traffic must not be mistaken for failure: successes collected across
// traffic gaps still close the circuit.
func TestHalfOpenAccumulatesSuccessesAcrossTraffic(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 2,
		HalfOpenTrials:   2,
		OpenDuration:     40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0) // trips

	time.Sleep(60 * time.Millisecond)
	b.Record(allow(t, b), true, 0) // first trial succeeds, round stays half-open
	time.Sleep(60 * time.Millisecond)
	b.Record(allow(t, b), true, 0) // second trial succeeds -> closes
	assertAdmits(t, b)
}

// A straggler admitted before the trip arrives while the breaker is half-open:
// its permit predates the round, so it must not close or re-open it.
func TestStragglerDoesNotDecideHalfOpenRound(t *testing.T) {
	b := newConsecutive(t, 40*time.Millisecond)
	b.Record(allow(t, b), false, 0)
	straggler := allow(t, b)
	b.Record(allow(t, b), false, 0) // trips

	time.Sleep(60 * time.Millisecond)
	b.Record(straggler, false, 0) // stale failure during half-open: ignored
	trial := allow(t, b)          // the round is still intact
	b.Record(trial, true, 0)
	assertAdmits(t, b)
}

// A permit from before the trip is still stale after the circuit has recovered:
// it must not feed the fresh Closed counters and trip them again.
func TestStalePermitAfterRecoveryIsIgnored(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 1,
		OpenDuration:     40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	stale := allow(t, b)              // admitted in the first Closed state
	b.Record(allow(t, b), false, 0)   // trips
	time.Sleep(60 * time.Millisecond) // cool-down elapses
	b.Record(allow(t, b), true, 0)    // trial closes the circuit
	b.Record(stale, false, 0)         // would re-trip a fresh Closed state
	assertAdmits(t, b)
}

// The zero Permit is never a valid outcome of Allow, so Record ignores it.
func TestZeroPermitIsIgnored(t *testing.T) {
	b := newConsecutive(t, time.Minute)
	b.Record(circuitbreaker.Permit{}, false, 0)
	b.Record(circuitbreaker.Permit{}, false, 0)
	assertAdmits(t, b)
}

// The strategy must be set: choosing it is the policy layer's job, so an empty
// one reaching the kernel is a wiring bug, not a default to guess.
func TestUnsetStrategyIsRejected(t *testing.T) {
	if b, err := circuitbreaker.New(circuitbreaker.Spec{FailureThreshold: 2}); err == nil || b != nil {
		t.Fatal("Spec without Strategy built a breaker")
	}
}

// A strategy must name its trip condition: a missing threshold is a
// configuration error, not a silent off switch.
func TestEnabledBreakerRequiresThreshold(t *testing.T) {
	for name, spec := range map[string]circuitbreaker.Spec{
		"consecutive no threshold": {Strategy: circuitbreaker.StrategyConsecutive},
		"error-count no threshold": {Strategy: circuitbreaker.StrategyErrorCount, MinSamples: 2},
		"error-rate no trip":       {Strategy: circuitbreaker.StrategyErrorRate, MinSamples: 2, Window: time.Minute},
		"slow-call no pair":        {Strategy: circuitbreaker.StrategySlowCall},
	} {
		if b, err := circuitbreaker.New(spec); err == nil || b != nil {
			t.Errorf("%s: New returned %v, %v", name, b, err)
		}
	}
}

// Each strategy validates only the fields it reads: fields another
// strategy would read are ignored — the strategy decides what counts.
func TestStrategyOwnsItsFields(t *testing.T) {

	// Fields another strategy would read: ignored, not rejected.
	spec := circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 2, FailureRate: 0.5,
		SlowThreshold: time.Second, SlowRate: 0.5, Window: time.Minute,
	}
	b, err := circuitbreaker.New(spec)
	if err != nil {
		t.Fatalf("extra fields rejected: %v", err)
	}
	b.Record(allow(t, b), false, 0)
	b.Record(allow(t, b), false, 0) // only the consecutive threshold counts: trips at 2
	assertRefuses(t, b)
}

// New rejects malformed Specs precisely — each check lives where its field is
// read — so a bad config is reported instead of the breaker silently degrading.
func TestNewRejectsMalformed(t *testing.T) {
	ok := circuitbreaker.Spec{
		Strategy: circuitbreaker.StrategyErrorRate, FailureRate: 0.5,
		MinSamples: 20, Window: time.Minute,
		SlowThreshold: time.Second, SlowRate: 0.8, HalfOpenTrials: 3,
	}
	if b, err := circuitbreaker.New(ok); err != nil || b == nil {
		t.Fatalf("valid Spec rejected: %v", err)
	}

	for name, spec := range map[string]circuitbreaker.Spec{
		"unknown strategy":      {Strategy: "error_rate", FailureRate: 0.5},
		"empty strategy":        {FailureThreshold: 2},
		"ratio above one":       {Strategy: circuitbreaker.StrategyErrorRate, FailureRate: 1.5},
		"negative failure rate": {Strategy: circuitbreaker.StrategyErrorRate, FailureRate: -0.1},
		"negative threshold":    {Strategy: circuitbreaker.StrategyConsecutive, FailureThreshold: -1},
		"half slow pair":        {Strategy: circuitbreaker.StrategySlowCall, SlowThreshold: time.Second},
		"slow rate above one":   {Strategy: circuitbreaker.StrategySlowCall, SlowThreshold: time.Second, SlowRate: 1.5},
		"negative window":       {Strategy: circuitbreaker.StrategyErrorCount, FailureThreshold: 2, Window: -time.Second},
	} {
		if b, err := circuitbreaker.New(spec); err == nil || b != nil {
			t.Errorf("%s: built a breaker", name)
		}
	}
}

// TestStatsTracksStateAndTransitions pins the read-only snapshot a holder
// publishes: the current state and the monotonic transition counts.
func TestStatsTracksStateAndTransitions(t *testing.T) {
	b, err := circuitbreaker.New(circuitbreaker.Spec{
		Strategy:         circuitbreaker.StrategyConsecutive,
		FailureThreshold: 2,
		OpenDuration:     10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := b.Stats()
	if s.State != circuitbreaker.StateClosed || s.Trips != 0 || s.HalfOpens != 0 || s.Recovers != 0 {
		t.Fatalf("fresh breaker stats = %+v", s)
	}

	for range 2 {
		p, ok := b.Allow()
		if !ok {
			t.Fatal("call refused while closed")
		}
		b.Record(p, false, 0)
	}
	s = b.Stats()
	if s.State != circuitbreaker.StateOpen || s.Trips != 1 || s.HalfOpens != 0 || s.Recovers != 0 {
		t.Fatalf("after trip stats = %+v", s)
	}

	time.Sleep(20 * time.Millisecond)
	p, ok := b.Allow() // crosses to half-open
	if !ok {
		t.Fatal("trial refused after cool-down")
	}
	s = b.Stats()
	if s.State != circuitbreaker.StateHalfOpen || s.HalfOpens != 1 {
		t.Fatalf("half-open stats = %+v", s)
	}

	b.Record(p, true, 0)
	s = b.Stats()
	if s.State != circuitbreaker.StateClosed || s.Recovers != 1 {
		t.Fatalf("recovered stats = %+v", s)
	}
}
