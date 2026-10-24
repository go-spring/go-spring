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

// Package circuitbreaker is the direction-neutral circuit breaker: the state
// machine an engine runs, without that engine's policy.
//
// # How a circuit breaker works
//
// A failing dependency gets worse when callers keep hammering it: each new
// call ties up a caller thread, a connection and a timeout that the
// dependency cannot serve, which turns one slow component into a cascading
// outage. A circuit breaker cuts that feedback loop by failing fast: once its
// strategy decides the unit is unhealthy, it stops letting calls through, so
// the dependency gets quiet time to recover and callers get an immediate, cheap
// error instead of an expensive timeout.
//
// It is a three-state machine:
//
//		Closed ──threshold reached──▶ Open ──cool-down elapsed──▶ HalfOpen
//		   ▲                             │                          │
//		   └──── all trials succeed ─────┘──── first trial fails ────┘
//		                              (re-opens, cool-down restarts)
//
//	  - Closed is business as usual: calls proceed and every outcome feeds the
//	    counting strategy ([StrategyConsecutive], [StrategyErrorRate],
//	    [StrategySlowCall], [StrategyErrorCount]).
//	  - Open is the tripped state: calls are refused immediately. It lasts
//	    [Spec.OpenDuration], and outcomes from calls admitted before the trip are
//	    ignored — they describe a state that no longer exists.
//	  - HalfOpen is the probe: after the cool-down a bounded number of trial
//	    calls ([Spec.HalfOpenTrials]) check whether the dependency recovered. All
//	    trials must succeed to close the circuit; the first failure re-opens it
//	    and the cool-down restarts. The round waits for its trials' outcomes
//	    instead of closing on a timer; [Spec.HalfOpenTimeout] is an optional
//	    safety valve that re-opens a round which never settles.
//
// # Direction neutrality
//
// The package knows nothing about a call or a request, only about outcomes. The
// outbound resilience engine (go-spring.org/cloud/runtime/dataplane/outbound/resilience)
// consumes it today; the state machine stays in the kernel because it is a
// mechanism, not a direction's policy. A caller assembles a [Spec] from its own
// policy, so a fix to how it counts or half-opens lands once.
//
// The package is a leaf: it depends on the standard library only.
package circuitbreaker

import (
	"sync"
	"time"
)

// defaultOpenDuration is the cool-down used when [Spec.OpenDuration] is unset.
const defaultOpenDuration = 5 * time.Second

// State is one state of a circuit breaker.
type State int

const (
	// StateClosed is the normal state: requests proceed and their outcomes feed
	// the breaker's counting strategy.
	StateClosed State = iota

	// StateOpen is the tripped state: requests are refused until the cool-down
	// elapses.
	StateOpen

	// StateHalfOpen is the trial state: a bounded number of requests are
	// admitted to probe recovery; their outcomes close or re-open the circuit.
	StateHalfOpen
)

// String returns the lowercase name ("closed" / "open" / "half_open").
func (s State) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// Strategy names the counting scheme a breaker uses to decide when to trip.
type Strategy string

const (
	// StrategyConsecutive trips after [Spec.FailureThreshold] failures in a row;
	// any success resets the count.
	StrategyConsecutive Strategy = "consecutive"

	// StrategyErrorRate trips when the failure ratio over [Spec.Window] reaches
	// [Spec.FailureRate], once the window's estimated sample count reaches
	// [Spec.MinSamples]. With a slow-call pair set it also trips when the slow ratio
	// reaches [Spec.SlowRate]; a slow call feeds the slow ratio whether it
	// succeeded or failed, so the two ratios are independent signals.
	StrategyErrorRate Strategy = "error-rate"

	// StrategySlowCall trips when the slow ratio over [Spec.Window] reaches
	// [Spec.SlowRate]; only slowness trips it — a failed request counts as
	// slow only when it was also slow, and fast failures never do.
	StrategySlowCall Strategy = "slow-call"

	// StrategyErrorCount trips when the failure count over [Spec.Window] reaches
	// [Spec.FailureThreshold].
	StrategyErrorCount Strategy = "error-count"
)

// Permit is proof that one call was admitted by a specific state of a breaker.
// Pass it back to Record (or RecordIgnored) exactly once. The outcome is
// accepted only while the admitting state is still current, so a call admitted
// before a trip — or before the half-open round it would otherwise decide —
// cannot influence a later state. The zero Permit is never accepted.
type Permit struct {
	gen uint64
}

// Spec is the breaker's own vocabulary: the knobs one breaker is built from.
// The consuming engine assembles it from its own policy, so the state machine
// never sees that policy's vocabulary.
//
// Exactly one counting strategy runs per breaker: Strategy is a single value,
// not a list, and strategies do not stack. The one de-facto combination: an
// [StrategyErrorRate] breaker with a slow-call pair set (SlowThreshold +
// SlowRate) trips on error ratio OR slow ratio — see [StrategyErrorRate].
// [StrategySlowCall] is built on that pair; consecutive and error-count ignore
// it. Each strategy reads only its own fields.
type Spec struct {
	// Strategy picks the counting scheme; it must be set. The policy layer
	// supplies its own default while assembling the Spec. A turned-off breaker
	// is not a Spec at all: the holder keeps a nil *Breaker (the same OFF
	// convention as *Bulkhead).
	Strategy Strategy

	// Failure counting — each strategy reads only its own threshold:
	FailureThreshold int     // Consecutive (failures in a row) / ErrorCount (failures in the window)
	FailureRate      float64 // ErrorRate (failures/total ratio)

	// Slow-call pair — ErrorRate trips on it as an OR arm, SlowCall is built on
	// it, the other strategies ignore it:
	SlowThreshold time.Duration // a call above this duration counts as slow
	SlowRate      float64       // slow ratio that trips

	// Rolling window — ErrorRate/SlowCall/ErrorCount only. Consecutive has no
	// time dimension and ignores both fields:
	MinSamples int           // minimum decayed sample estimate before tripping; <=0 becomes 1 (trips on the first bad sample)
	Window     time.Duration // the rolling interval

	// Open/half-open state machine — shared by every strategy.

	// OpenDuration is the cool-down after a trip: while Open, every Allow is
	// refused for this long before a half-open round starts. <=0 means the
	// package default (5s).
	OpenDuration time.Duration

	// HalfOpenTrials is how many trials a half-open round admits; anything
	// below 2 means one.
	HalfOpenTrials int

	// HalfOpenTimeout bounds one half-open round. <=0, the default, waits for
	// the trials' outcomes no matter how long they take — the same choice
	// resilience4j (maxWaitDurationInHalfOpenState=0) and gobreaker make, so
	// low traffic simply keeps the circuit half-open and every arriving request
	// is another trial. A positive value re-opens the circuit (restarting the
	// cool-down) when the round outlives it, so a lost or hung trial cannot pin
	// the breaker half-open forever.
	HalfOpenTimeout time.Duration
}

// Breaker is one circuit breaker: the open/half-open state machine plus the
// counting strategy it delegates to (see strategy.go). Counting is delegated to
// one strategy; the Breaker decides only when to switch states. Half-open
// admits up to [Spec.HalfOpenTrials] trials; all must succeed to close, the
// first failure re-opens.
//
// Build it with [New], which refuses a malformed Spec. The zero value is not
// usable; "off" is a nil *Breaker at the holder, not an instance.
type Breaker struct {
	// Immutable after New.
	strategy        countingStrategy
	openDuration    time.Duration
	halfOpenTrials  int
	halfOpenTimeout time.Duration

	// mu guards every mutable field below.
	mu sync.Mutex

	// Current state and the fields a transition derives from it. The transition
	// functions own all of them, so callers never hand a state in. The zero
	// value of state is StateClosed.
	state State

	// openedAt is when Open was entered; it measures the cool-down.
	openedAt time.Time

	// gen identifies the current state instance. Every transition bumps it and
	// every [Permit] carries the generation that admitted its call, so Record
	// can reject outcomes from an earlier state — a Closed permit after a trip,
	// or a trial permit from a round that was abandoned.
	gen uint64

	// Half-open round.

	// halfOpenPermits is non-nil exactly while the breaker is half-open: each
	// admitted trial takes one permit from it, and its presence is what makes
	// Allow hand out trial permits instead of refusing.
	halfOpenPermits chan struct{}

	// halfOpenStartedAt is when the round began. It is consulted only when a
	// HalfOpenTimeout is configured, to re-open a round that never settles.
	halfOpenStartedAt time.Time

	// halfOpenPassed counts the trials that succeeded in the current round.
	halfOpenPassed int

	// Monotonic per-instance transition counters, read (never written) by the
	// owning governance organ when it publishes state metrics. They reset when
	// the holder rebuilds the breaker for a new policy generation, which is the
	// intended meaning: this is a new breaker instance.
	trips     uint64 // entries into Open
	halfOpens uint64 // entries into HalfOpen
	recovers  uint64 // entries into Closed
}

// Stats is a read-only snapshot of one breaker: the current state plus the
// transitions into each state since the breaker was built. A holder publishes
// these as its own metrics; the breaker itself knows nothing about
// instrumentation.
type Stats struct {
	State     State
	Trips     uint64
	HalfOpens uint64
	Recovers  uint64
}

// New builds the breaker for one protected unit (a service or an instance)
// from spec. A Spec must be complete: each strategy constructor validates the
// fields it reads (see newCountingStrategy), and a missing required threshold is
// an error rather than a silent off switch. "Off" is not a Spec at all — the
// holder keeps a nil *Breaker and never calls New.
func New(spec Spec) (*Breaker, error) {
	c, err := newCountingStrategy(spec)
	if err != nil {
		return nil, err
	}
	openDuration := spec.OpenDuration
	if openDuration <= 0 {
		openDuration = defaultOpenDuration
	}
	halfOpenTrials := max(spec.HalfOpenTrials, 1) // anything below 2 means a single trial
	return &Breaker{
		openDuration:    openDuration,
		halfOpenTrials:  halfOpenTrials,
		halfOpenTimeout: spec.HalfOpenTimeout,
		gen:             1, // the zero Permit carries gen 0 and is never accepted
		strategy:        c,
	}, nil
}

// Stats returns a read-only snapshot of the breaker's state and per-instance
// transition counts. The holder of the breaker publishes these through its own
// instruments; this package never imports an observability SDK.
func (b *Breaker) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return Stats{
		State:     b.state,
		Trips:     b.trips,
		HalfOpens: b.halfOpens,
		Recovers:  b.recovers,
	}
}

// Allow reports whether a request may proceed and, when it may, the permit
// Record needs to accept its outcome.
func (b *Breaker) Allow() (Permit, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Apply the time-driven transitions first: Open→HalfOpen once the cool-down
	// elapses, and — only when a HalfOpenTimeout is configured — HalfOpen→Open
	// for a round that outlived it. Half-open itself has no deadline: with the
	// default unlimited timeout, the round waits for its trials' outcomes no
	// matter how long that takes, so low traffic just keeps probing.
	switch b.state {
	case StateClosed:
		return Permit{gen: b.gen}, true
	case StateOpen:
		if time.Since(b.openedAt) < b.openDuration {
			return Permit{}, false // still cooling down
		}
		b.toHalfOpenLocked()
	case StateHalfOpen:
		if b.halfOpenTimeout > 0 && time.Since(b.halfOpenStartedAt) >= b.halfOpenTimeout {
			b.toOpenLocked()
			return Permit{}, false
		}
	}

	select {
	case <-b.halfOpenPermits:
		return Permit{gen: b.gen}, true
	default:
		return Permit{}, false
	}
}

// Record folds one completed request's outcome into the breaker. p is the
// permit Allow issued for that request; a call that was admitted before the
// circuit changed state carries an older generation and is dropped — it must
// not close the circuit, refresh the cool-down, feed a stale window, or decide
// a half-open round it never joined.
//
// A zero Permit is ignored.
func (b *Breaker) Record(p Permit, success bool, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p.gen == 0 || p.gen != b.gen {
		return // the state that admitted this call no longer exists
	}

	switch b.state {
	case StateHalfOpen:
		// The permit is a trial, so its outcome decides the round.
		if success {
			b.halfOpenPassed++
			if b.halfOpenPassed >= b.halfOpenTrials {
				b.toClosedLocked()
			}
			return
		}
		b.toOpenLocked() // the first trial failure re-opens
	case StateOpen:
		return // no permit can carry this generation
	case StateClosed:
		if b.strategy.record(success, duration) {
			b.toOpenLocked()
		}
	}
}

// RecordIgnored settles a permit whose outcome carries no verdict — the caller
// was cancelled before the unit's result was known. In Closed it adds no sample
// to the counting strategy, keeping the caller's "no evidence is not a verdict"
// contract. In half-open it returns the trial permit and leaves the round
// waiting: an unclassified probe is neither evidence of recovery nor of failure,
// so a verdict-free trial must not consume the round's trial budget. A stale
// permit is ignored as in [Breaker.Record].
func (b *Breaker) RecordIgnored(p Permit) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p.gen == 0 || p.gen != b.gen {
		return // the state that admitted this call no longer exists
	}
	if b.state == StateHalfOpen {
		// Non-blocking: the permit we took always leaves room, but a duplicate
		// settlement must not block while holding the lock.
		select {
		case b.halfOpenPermits <- struct{}{}:
		default:
		}
	}
}

// One transition is one function: it owns the state change and every field the
// transition derives from it (generation, counters, the state's start time), so
// a transition is defined exactly once and callers only say where the circuit
// goes. Observers read the result through [Breaker.Stats]; the breaker emits
// nothing itself.

// toOpenLocked enters Open: any half-open round is discarded (its outstanding
// trial permits no longer count) and the cool-down restarts.
func (b *Breaker) toOpenLocked() {
	now := time.Now()
	b.state = StateOpen
	b.gen++
	b.halfOpenPermits = nil
	b.halfOpenPassed = 0
	b.openedAt = now
	b.trips++
}

// toHalfOpenLocked opens a fresh half-open round: a bounded channel of trial
// permits, timed for the optional HalfOpenTimeout safety valve (see Allow).
func (b *Breaker) toHalfOpenLocked() {
	now := time.Now()
	b.state = StateHalfOpen
	b.gen++
	b.halfOpenPassed = 0
	b.halfOpenStartedAt = now
	b.halfOpens++
	b.halfOpenPermits = make(chan struct{}, b.halfOpenTrials)
	for range b.halfOpenTrials {
		b.halfOpenPermits <- struct{}{}
	}
}

// toClosedLocked closes the circuit from a settled half-open round: the
// strategy's counters reset, so the fresh Closed state trips on fresh evidence
// only.
func (b *Breaker) toClosedLocked() {
	b.state = StateClosed
	b.gen++
	b.openedAt = time.Time{}
	b.halfOpenPermits = nil
	b.halfOpenPassed = 0
	b.recovers++
	b.strategy.reset()
}
