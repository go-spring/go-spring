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

package resilience

import (
	"sync"
	"time"
)

// BreakerStrategy selects how the circuit breaker counts failures. Each [Driver]
// maps it onto its own primitives; the default driver implements both directly,
// sentinel-golang translates them onto its ErrorCount / ErrorRatio rules.
type BreakerStrategy string

const (
	// BreakerConsecutive trips after [Policy.ErrorThreshold] failures in a row
	// (any success resets the count). It is the default when BreakerStrategy is
	// empty, matching the historical behavior.
	BreakerConsecutive BreakerStrategy = "consecutive"
	// BreakerErrorRate trips when the failure ratio over [Policy.BreakerWindow]
	// reaches [Policy.ErrorRateThreshold], once at least [Policy.MinRequests]
	// requests have been observed in the window. Use it for high-throughput
	// resources where a burst of failures or a steady partial-failure rate is
	// more meaningful than a consecutive run.
	BreakerErrorRate BreakerStrategy = "error-rate"
)

// BreakerState is one state of a circuit breaker.
type BreakerState int

const (
	// BreakerClosed is the normal state: calls proceed and their outcomes feed
	// the breaker's failure counting.
	BreakerClosed BreakerState = iota
	// BreakerOpen is the tripped state: calls are rejected with ErrCircuitOpen
	// without invoking fn, until the cool-down elapses.
	BreakerOpen
	// BreakerHalfOpen is the trial state: exactly one call is admitted to probe
	// recovery; its outcome either closes or re-opens the circuit.
	BreakerHalfOpen
)

// String returns the lowercase OTel-style name ("closed" / "open" / "half_open").
func (s BreakerState) String() string {
	switch s {
	case BreakerOpen:
		return "open"
	case BreakerHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// BreakerEventListener receives circuit-breaker state transitions. When a
// breaker trips, half-opens or recovers, OnBreakerStateChange is called with
// the resource and the from/to states.
//
// The listener is invoked synchronously from inside the breaker's state
// transition. It must NOT call back into the same Executor (it would deadlock
// on the breaker's lock) — emit a metric, log, or push onto a channel instead.
type BreakerEventListener interface {
	OnBreakerStateChange(resource string, from, to BreakerState)
}

// BreakerEventListenerSetter is optionally implemented by Executors whose driver
// can emit circuit-breaker state transitions. The default driver implements it;
// observe-resilience uses it to attach metric + log emission. A driver that
// cannot observe transitions (e.g. one delegating to a library without state
// callbacks) simply does not implement it, and SetBreakerEventListener calls
// against it are a no-op detected by a failed type assertion.
type BreakerEventListenerSetter interface {
	SetBreakerEventListener(BreakerEventListener)
}

// circuitBreaker supports two strategies over one open/half-open state machine:
//
//   - consecutive: trips after threshold failures in a row (a success resets
//     the count), the historical behavior.
//   - error-rate: trips when fails/total over a rolling window reaches a ratio,
//     once a minimum sample has accumulated.
//
// Half-open admits exactly one trial: the gate is a single-permit channel, not a
// bool flag, so two concurrent callers arriving right after cool-down cannot
// both be admitted (the prior flag-based implementation had exactly that race).
type circuitBreaker struct {
	strategy BreakerStrategy
	openFor  time.Duration

	// consecutive strategy
	threshold int

	// error-rate strategy
	rateThreshold float64 // fails/total ratio that trips
	minRequests   int     // minimum sample before tripping
	window        time.Duration

	mu       sync.Mutex
	failures int // consecutive counter
	openedAt time.Time
	halfOpen chan struct{} // non-nil while a single trial permit is offered

	// rolling-window counters for the error-rate strategy
	win       time.Duration
	curStart  time.Time
	total     int
	fails     int
	prevTotal int
	prevFails int

	// event emission
	resource string               // label passed to the listener
	listener BreakerEventListener // nil = no listener; fixed at construction
}

// newCircuitBreaker builds a breaker for one resource. listener is the executor's
// listener captured at construction; it never changes for the life of the
// breaker, so no synchronization is needed to read it.
func newCircuitBreaker(p Policy, open time.Duration, resource string, listener BreakerEventListener) *circuitBreaker {
	c := &circuitBreaker{
		strategy:      p.ResolvedBreakerStrategy(),
		openFor:       open,
		threshold:     p.ErrorThreshold,
		rateThreshold: p.ErrorRateThreshold,
		minRequests:   p.MinRequests,
		resource:      resource,
		listener:      listener,
	}
	if c.strategy == BreakerErrorRate {
		if c.minRequests <= 0 {
			c.minRequests = 1
		}
		if p.BreakerWindow > 0 {
			c.win = p.BreakerWindow
		} else {
			c.win = time.Second
		}
		c.curStart = time.Now()
	}
	return c
}

// allow reports whether a request may proceed given the current breaker state.
func (c *circuitBreaker) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openedAt.IsZero() {
		return true // closed
	}
	if time.Since(c.openedAt) < c.openFor {
		return false // open, cooling down
	}
	// Cool-down elapsed: open a single-permit half-open gate if none yet, then
	// take the only permit. A concurrent caller finds the gate empty and is
	// treated as still-open — exactly one trial is admitted.
	if c.halfOpen == nil {
		c.halfOpen = make(chan struct{}, 1)
		c.halfOpen <- struct{}{}
		c.notifyLocked(BreakerOpen, BreakerHalfOpen)
	}
	select {
	case <-c.halfOpen:
		return true
	default:
		return false
	}
}

// record folds an attempt's outcome back into the breaker state.
func (c *circuitBreaker) record(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Half-open trial resolves first: its outcome closes or re-opens the
	// circuit regardless of strategy.
	if c.halfOpen != nil {
		if success {
			c.closeLocked()
			c.notifyLocked(BreakerHalfOpen, BreakerClosed)
			return
		}
		// Trial failed: re-open the cool-down window.
		c.halfOpen = nil
		c.openedAt = time.Now()
		c.notifyLocked(BreakerHalfOpen, BreakerOpen)
		return
	}
	if c.strategy == BreakerErrorRate {
		c.recordRate(success)
		return
	}
	c.recordConsecutive(success)
}

func (c *circuitBreaker) recordConsecutive(success bool) {
	if success {
		c.failures = 0
		c.openedAt = time.Time{}
		return
	}
	c.failures++
	if c.failures >= c.threshold {
		if c.openedAt.IsZero() {
			c.notifyLocked(BreakerClosed, BreakerOpen)
		}
		c.openedAt = time.Now()
	}
}

// recordRate advances the rolling-window counters and trips the breaker when
// the failure ratio reaches rateThreshold with enough samples. It uses the same
// weighted two-window estimate as the standalone slidingWindow limiter.
func (c *circuitBreaker) recordRate(success bool) {
	now := time.Now()
	elapsed := now.Sub(c.curStart)
	if elapsed >= c.win {
		if elapsed >= 2*c.win {
			c.prevTotal, c.prevFails = 0, 0
		} else {
			c.prevTotal, c.prevFails = c.total, c.fails
		}
		c.total, c.fails = 0, 0
		c.curStart = now
		elapsed = 0
	}
	c.total++
	if !success {
		c.fails++
	}
	weight := float64(c.win-elapsed) / float64(c.win)
	estimateTotal := float64(c.prevTotal)*weight + float64(c.total)
	estimateFails := float64(c.prevFails)*weight + float64(c.fails)
	if c.total+c.prevTotal >= c.minRequests && estimateTotal > 0 &&
		estimateFails/estimateTotal >= c.rateThreshold {
		if c.openedAt.IsZero() {
			c.notifyLocked(BreakerClosed, BreakerOpen)
		}
		c.openedAt = now
	}
}

func (c *circuitBreaker) closeLocked() {
	c.failures = 0
	c.openedAt = time.Time{}
	c.halfOpen = nil
	// Reset the windowed counters so a freshly-closed breaker starts clean.
	c.total, c.fails = 0, 0
	c.prevTotal, c.prevFails = 0, 0
	c.curStart = time.Now()
}

// notifyLocked emits a from→to transition to the listener, if one is attached.
// Caller holds mu; the listener must not call back into the breaker/executor
// (documented on [BreakerEventListener]).
func (c *circuitBreaker) notifyLocked(from, to BreakerState) {
	if from == to || c.listener == nil {
		return
	}
	c.listener.OnBreakerStateChange(c.resource, from, to)
}
