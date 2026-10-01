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

// executor_default_breaker.go is the circuit-breaker organ of the bundled
// default executor, with its state-change events.

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
	// BreakerConsecutive trips after [ClientPolicy.ErrorThreshold] failures in a row
	// (any success resets the count). It is the default when BreakerStrategy is
	// empty, matching the historical behavior.
	BreakerConsecutive BreakerStrategy = "consecutive"
	// BreakerErrorRate trips when the failure ratio over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.ErrorRateThreshold], once at least [ClientPolicy.MinRequests]
	// requests have been observed in the window. Use it for high-throughput
	// services where a burst of failures or a steady partial-failure rate is
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
// the service and the from/to states.
//
// The listener is invoked synchronously from inside the breaker's state
// transition. It must NOT call back into the same ClientExecutor (it would deadlock
// on the breaker's lock) — emit a metric, log, or push onto a channel instead.
type BreakerEventListener interface {
	OnBreakerStateChange(service string, from, to BreakerState)
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
//   - error-rate: trips when bad/total over a rolling window reaches a ratio,
//     once a minimum sample has accumulated. A sample is "bad" when it failed
//     OR (with the slow-call pair set) it succeeded but took at least
//     SlowCallDurationThreshold — a downstream that never errors but answers
//     slowly trips the same window.
//
// Half-open admits a bounded number of trials (HalfOpenRequests, default 1):
// the gate is a permit channel, not a bool flag, so concurrent callers arriving
// right after cool-down cannot exceed the admitted count (the prior flag-based
// implementation had exactly that race). All N succeeding closes the circuit;
// the first failure re-opens it.
type circuitBreaker struct {
	strategy BreakerStrategy
	openFor  time.Duration

	// consecutive strategy
	threshold int

	// error-rate strategy
	rateThreshold float64 // bad/total ratio that trips
	minRequests   int     // minimum sample before tripping
	window        time.Duration

	// slow-call counting (error-rate strategy): a success at or above
	// slowThreshold counts as bad when slowRateThreshold is set.
	slowThreshold time.Duration
	slowRate      float64

	// half-open admission count (0/1 = the historical single trial)
	halfOpenN int

	mu       sync.Mutex
	failures int // consecutive counter
	openedAt time.Time
	halfOpen chan struct{} // non-nil while a single trial permit is offered

	// rolling-window counters for the error-rate strategy. fails counts
	// failures, slows counts slow-but-successful attempts; either ratio (or
	// both) can trip.
	win       time.Duration
	curStart  time.Time
	total     int
	fails     int
	slows     int
	prevTotal int
	prevFails int
	prevSlows int

	// half-open trial tracking: how many trials are admitted and how many have
	// succeeded so far; the circuit closes when ok reaches the admitted count.
	halfOpenOK int

	// event emission
	service  string               // label passed to the listener
	listener BreakerEventListener // nil = no listener; fixed at construction
}

// newCircuitBreaker builds a breaker for one service. listener is the executor's
// listener captured at construction; it never changes for the life of the
// breaker, so no synchronization is needed to read it.
func newCircuitBreaker(p ClientPolicy, open time.Duration, service string, listener BreakerEventListener) *circuitBreaker {
	c := &circuitBreaker{
		strategy:      p.ResolvedBreakerStrategy(),
		openFor:       open,
		threshold:     p.ErrorThreshold,
		rateThreshold: p.ErrorRateThreshold,
		minRequests:   p.MinRequests,
		service:       service,
		listener:      listener,
	}
	if p.SlowCallDurationThreshold > 0 && p.SlowCallRateThreshold > 0 {
		c.slowThreshold = p.SlowCallDurationThreshold
		c.slowRate = p.SlowCallRateThreshold
	}
	if p.HalfOpenRequests > 1 {
		c.halfOpenN = p.HalfOpenRequests
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
	// Cool-down elapsed: open a half-open gate with the configured number of
	// trial permits if none yet, then take one. A concurrent caller finding the
	// gate empty is treated as still-open — no more than halfOpenN trials (1 by
	// default) are admitted per cool-down.
	if c.halfOpen == nil {
		n := c.halfOpenN
		if n <= 0 {
			n = 1
		}
		c.halfOpenOK = 0
		c.halfOpen = make(chan struct{}, n)
		for range n {
			c.halfOpen <- struct{}{}
		}
		c.notifyLocked(BreakerOpen, BreakerHalfOpen)
	}
	select {
	case <-c.halfOpen:
		return true
	default:
		return false
	}
}

// record folds an attempt's outcome back into the breaker state. dur is how
// long the recorded attempt took; it feeds slow-call counting (ignored when the
// slow-call pair is unset).
func (c *circuitBreaker) record(success bool, dur time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	slow := c.slowThreshold > 0 && dur >= c.slowThreshold
	// Half-open trial resolves first: its outcome closes or re-opens the
	// circuit regardless of strategy. With N admitted trials, every one must
	// succeed to close; the first failure re-opens immediately.
	if c.halfOpen != nil {
		if success {
			c.halfOpenOK++
			n := c.halfOpenN
			if n <= 0 {
				n = 1
			}
			if c.halfOpenOK >= n {
				c.closeLocked()
				c.notifyLocked(BreakerHalfOpen, BreakerClosed)
			}
			return
		}
		// Trial failed: re-open the cool-down window.
		c.halfOpen = nil
		c.openedAt = time.Now()
		c.notifyLocked(BreakerHalfOpen, BreakerOpen)
		return
	}
	if c.strategy == BreakerErrorRate {
		c.recordRate(success, slow)
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
// the failure ratio or the slow-call ratio reaches its threshold with enough
// samples. It uses the same weighted two-window estimate as the standalone
// slidingWindow limiter.
func (c *circuitBreaker) recordRate(success, slow bool) {
	now := time.Now()
	elapsed := now.Sub(c.curStart)
	if elapsed >= c.win {
		if elapsed >= 2*c.win {
			c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
		} else {
			c.prevTotal, c.prevFails, c.prevSlows = c.total, c.fails, c.slows
		}
		c.total, c.fails, c.slows = 0, 0, 0
		c.curStart = now
		elapsed = 0
	}
	c.total++
	if !success {
		c.fails++
	} else if slow {
		c.slows++
	}
	weight := float64(c.win-elapsed) / float64(c.win)
	estimateTotal := float64(c.prevTotal)*weight + float64(c.total)
	estimateFails := float64(c.prevFails)*weight + float64(c.fails)
	estimateSlows := float64(c.prevSlows)*weight + float64(c.slows)
	trip := false
	if c.total+c.prevTotal >= c.minRequests && estimateTotal > 0 {
		if c.rateThreshold > 0 && estimateFails/estimateTotal >= c.rateThreshold {
			trip = true
		}
		if c.slowRate > 0 && estimateSlows/estimateTotal >= c.slowRate {
			trip = true
		}
	}
	if trip {
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
	c.total, c.fails, c.slows = 0, 0, 0
	c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
	c.curStart = time.Now()
}

// notifyLocked emits a from→to transition to the listener, if one is attached.
// Caller holds mu; the listener must not call back into the breaker/executor
// (documented on [BreakerEventListener]).
func (c *circuitBreaker) notifyLocked(from, to BreakerState) {
	if from == to || c.listener == nil {
		return
	}
	c.listener.OnBreakerStateChange(c.service, from, to)
}
