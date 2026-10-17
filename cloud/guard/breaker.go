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

package guard

import (
	"sync"
	"time"
)

// BreakerState is one state of a circuit breaker.
type BreakerState int

const (
	// BreakerClosed is the normal state: requests proceed and their outcomes
	// feed the breaker's failure counting.
	BreakerClosed BreakerState = iota

	// BreakerOpen is the tripped state: requests are refused with
	// [ErrCircuitOpen] until the cool-down elapses.
	BreakerOpen

	// BreakerHalfOpen is the trial state: a bounded number of requests are
	// admitted to probe recovery; their outcomes close or re-open the circuit.
	BreakerHalfOpen
)

// String returns the lowercase name ("closed" / "open" / "half_open").
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

// BreakerEventListener receives circuit-breaker state transitions. It is
// invoked synchronously from inside the transition and must not call back into
// the same Guard (it would deadlock).
type BreakerEventListener interface {
	OnBreakerStateChange(service string, from, to BreakerState)
}

// BreakerEventListenerSetter is implemented by a [Guard] that can emit breaker
// transitions. Observability attaches its listener through it at wiring time;
// the engine never imports the observer.
type BreakerEventListenerSetter interface {
	SetBreakerEventListener(BreakerEventListener)
}

type breakerStrategy uint8

const (
	breakerConsecutive breakerStrategy = iota
	breakerErrorRate
	breakerSlowCall
	breakerErrorCount
)

// breakerSpec is the breaker's own vocabulary, assembled from a [Policy] so the
// engine never reads a policy field directly.
type breakerSpec struct {
	strategy      breakerStrategy
	threshold     int
	rateThreshold float64
	minRequests   int
	window        time.Duration
	slowThreshold time.Duration
	slowRate      float64
	halfOpenN     int
}

func (s breakerSpec) active() bool {
	return s.threshold > 0 || s.rateThreshold > 0 || (s.slowThreshold > 0 && s.slowRate > 0)
}

// breakerSpecFor assembles the breaker's view of p. An explicit strategy wins;
// a slow-call pair without one selects error-rate (slow successes fold into the
// error rate); otherwise consecutive is the default.
func breakerSpecFor(p Policy) breakerSpec {
	spec := breakerSpec{
		threshold:     p.ErrorThreshold,
		rateThreshold: p.ErrorRateThreshold,
		minRequests:   p.MinRequests,
		window:        p.BreakerWindow,
		slowThreshold: p.SlowCallDurationThreshold,
		slowRate:      p.SlowCallRateThreshold,
		halfOpenN:     p.HalfOpenRequests,
	}
	switch p.BreakerStrategy {
	case "error-rate":
		spec.strategy = breakerErrorRate
	case "slow-call":
		spec.strategy = breakerSlowCall
	case "error-count":
		spec.strategy = breakerErrorCount
	default:
		if spec.slowThreshold > 0 && spec.slowRate > 0 {
			spec.strategy = breakerErrorRate
		} else {
			spec.strategy = breakerConsecutive
		}
	}
	return spec
}

// Breaker is the circuit-breaker stage as an independent object. It is the same
// implementation [Guard] uses; a caller assembling a custom middleware can take
// one directly.
//
// A Breaker built from a policy with no active strategy admits every request
// and Record is a no-op.
type Breaker struct {
	b *circuitBreaker
}

// NewBreaker builds the breaker for one server instance from p. listener may be
// nil; it receives state transitions.
func NewBreaker(p Policy, service string, listener BreakerEventListener) *Breaker {
	spec := breakerSpecFor(p)
	if !spec.active() {
		return &Breaker{}
	}
	open := p.OpenDuration
	if open <= 0 {
		open = 5 * time.Second
	}
	return &Breaker{b: newCircuitBreaker(spec, open, service, listener)}
}

// Allow reports whether a request may proceed. A nil or inactive Breaker always
// allows.
func (b *Breaker) Allow() bool {
	return b == nil || b.b == nil || b.b.allow()
}

// Record folds one completed request's outcome into the breaker. OutcomeUnknown
// is ignored; an inactive Breaker ignores everything.
func (b *Breaker) Record(o Outcome, dur time.Duration) {
	if b == nil || b.b == nil || o == OutcomeUnknown {
		return
	}
	b.b.record(o == OutcomeSuccess, dur)
}

// circuitBreaker is one instance's breaker. It supports the same four counting
// strategies as the client engine:
//
//   - consecutive: trips after threshold failures in a row.
//   - error-rate: trips when bad/total over a rolling window reaches the ratio.
//   - slow-call: trips when the slow ratio over the window reaches the ratio.
//   - error-count: trips when the failure count over the window reaches the
//     threshold.
//
// Half-open admits up to halfOpenN trials; all must succeed to close, the first
// failure re-opens.
type circuitBreaker struct {
	strategy breakerStrategy
	openFor  time.Duration

	threshold     int
	rateThreshold float64
	minRequests   int
	win           time.Duration
	slowThreshold time.Duration
	slowRate      float64
	halfOpenN     int

	mu       sync.Mutex
	failures int
	openedAt time.Time
	halfOpen chan struct{}

	curStart  time.Time
	total     int
	fails     int
	slows     int
	prevTotal int
	prevFails int
	prevSlows int

	halfOpenOK int

	service  string
	listener BreakerEventListener
}

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
	if c.usesWindow() {
		if c.minRequests <= 0 {
			c.minRequests = 1
		}
		c.win = spec.window
		if c.win <= 0 {
			c.win = time.Second
		}
		c.curStart = time.Now()
	}
	return c
}

func (c *circuitBreaker) usesWindow() bool { return c.strategy != breakerConsecutive }

// allow reports whether a request may proceed given the current state.
func (c *circuitBreaker) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.openedAt.IsZero() {
		return true
	}
	if time.Since(c.openedAt) < c.openFor {
		return false
	}
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

// record folds one completed request's outcome into the breaker. Unknown
// outcomes are not passed here at all (see [Done]).
func (c *circuitBreaker) record(success bool, dur time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	slow := c.slowThreshold > 0 && dur >= c.slowThreshold

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
		c.halfOpen = nil
		c.openedAt = time.Now()
		c.notifyLocked(BreakerHalfOpen, BreakerOpen)
		return
	}

	if c.usesWindow() {
		c.recordWindow(success, slow)
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

func (c *circuitBreaker) recordWindow(success, slow bool) {
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
		if c.strategy == breakerSlowCall && slow {
			c.slows++
		}
	} else if slow {
		c.slows++
	}
	weight := float64(c.win-elapsed) / float64(c.win)
	estimateTotal := float64(c.prevTotal)*weight + float64(c.total)
	estimateFails := float64(c.prevFails)*weight + float64(c.fails)
	estimateSlows := float64(c.prevSlows)*weight + float64(c.slows)
	trip := false
	if c.total+c.prevTotal >= c.minRequests && estimateTotal > 0 {
		switch c.strategy {
		case breakerErrorCount:
			trip = estimateFails >= float64(c.threshold)
		case breakerSlowCall:
			trip = c.slowRate > 0 && estimateSlows/estimateTotal >= c.slowRate
		default: // error-rate, with slow successes folded in
			if c.rateThreshold > 0 && estimateFails/estimateTotal >= c.rateThreshold {
				trip = true
			}
			if c.slowRate > 0 && estimateSlows/estimateTotal >= c.slowRate {
				trip = true
			}
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
	c.total, c.fails, c.slows = 0, 0, 0
	c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
	c.curStart = time.Now()
}

func (c *circuitBreaker) notifyLocked(from, to BreakerState) {
	if from == to || c.listener == nil {
		return
	}
	c.listener.OnBreakerStateChange(c.service, from, to)
}
