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

	"go-spring.org/cloud/observability"
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

// The breaker's state and its observation contracts are defined once in
// [observability] — they are emission concerns, and the observe wrappers live
// there — and aliased here so this engine and its callers keep the resilience
// names.

// BreakerState is one state of a circuit breaker.
type BreakerState = observability.BreakerState

const (
	// BreakerClosed is the normal state: calls proceed and their outcomes feed
	// the breaker's failure counting.
	BreakerClosed = observability.BreakerClosed
	// BreakerOpen is the tripped state: calls are rejected with chain.ErrCircuitOpen
	// without invoking fn, until the cool-down elapses.
	BreakerOpen = observability.BreakerOpen
	// BreakerHalfOpen is the trial state: exactly one call is admitted to probe
	// recovery; its outcome either closes or re-opens the circuit.
	BreakerHalfOpen = observability.BreakerHalfOpen
)

// BreakerEventListener receives circuit-breaker state transitions.
type BreakerEventListener = observability.BreakerEventListener

// BreakerEventListenerSetter is optionally implemented by Executors whose driver
// can emit circuit-breaker state transitions.
type BreakerEventListenerSetter = observability.BreakerEventListenerSetter

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

	// half-open inbound count (0/1 = the historical single trial)
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

// breakerSpec is the circuit breaker's own vocabulary: the thresholds one breaker
// is built for. Both directions assemble it from their policy (see
// [ClientPolicy.breakerSpec] / [ServerPolicy.breakerSpec]), so the one organ with
// real internal state stays shared without either policy being privileged and
// without a direction ever reaching the other's breaker.
type breakerSpec struct {
	strategy      BreakerStrategy
	threshold     int           // consecutive: failures in a row that trip
	rateThreshold float64       // error-rate: bad/total ratio that trips
	minRequests   int           // error-rate: minimum sample before tripping
	window        time.Duration // error-rate: the rolling interval counted over
	slowThreshold time.Duration // a success at or above this counts as bad
	slowRate      float64
	halfOpenN     int // trials admitted half-open; 0 and 1 both mean one
}

// active reports whether any breaker strategy is configured. Both directions
// consult it, so a breaker is built under exactly the same condition on either
// side.
func (s breakerSpec) active() bool {
	return s.threshold > 0 || s.rateThreshold > 0 || (s.slowThreshold > 0 && s.slowRate > 0)
}

// resolveBreakerStrategy picks the strategy a breaker should apply. An explicit
// [BreakerErrorRate] wins; a slow-call pair without an explicit strategy also
// selects it (slow calls are counted by the rate window); otherwise the
// consecutive default. Shared by both directions so neither can disagree with
// the other on what an unset strategy means.
func resolveBreakerStrategy(explicit BreakerStrategy, slowThreshold time.Duration, slowRate float64) BreakerStrategy {
	if explicit == BreakerErrorRate || (slowThreshold > 0 && slowRate > 0) {
		return BreakerErrorRate
	}
	return BreakerConsecutive
}

// newCircuitBreaker builds a breaker for one service or route from spec. open is
// how long the circuit stays open; listener is the executor's listener captured
// at construction, and it never changes for the life of the breaker, so no
// synchronization is needed to read it.
func newCircuitBreaker(spec breakerSpec, open time.Duration, service string, listener BreakerEventListener) *circuitBreaker {
	c := &circuitBreaker{
		strategy:      spec.strategy,
		openFor:       open,
		threshold:     spec.threshold,
		rateThreshold: spec.rateThreshold,
		minRequests:   spec.minRequests,
		service:       service,
		listener:      listener,
	}
	if spec.slowThreshold > 0 && spec.slowRate > 0 {
		c.slowThreshold = spec.slowThreshold
		c.slowRate = spec.slowRate
	}
	if spec.halfOpenN > 1 {
		c.halfOpenN = spec.halfOpenN
	}
	if c.strategy == BreakerErrorRate {
		if c.minRequests <= 0 {
			c.minRequests = 1
		}
		if spec.window > 0 {
			c.win = spec.window
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
