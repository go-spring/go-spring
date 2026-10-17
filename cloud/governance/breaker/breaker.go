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

// Package breaker is the direction-neutral circuit breaker: the state machine
// both call directions run, without either direction's policy.
//
// Outbound calls (go-spring.org/cloud/governance/resilience) and inbound requests
// (go-spring.org/cloud/governance/guard) keep separate policies and separate authorities,
// but the breaker they consult is the same state machine, so a fix to how it
// counts or half-opens lands once. A caller assembles a [Spec] from its own
// policy; nothing here knows whether the protected unit is a call or a request.
//
// The package is a leaf: it depends on the standard library only.
package breaker

import (
	"sync"
	"time"
)

const defaultOpenFor = 5 * time.Second

// State is one state of a circuit breaker.
type State int

const (
	// Closed is the normal state: requests proceed and their outcomes feed the
	// breaker's failure counting.
	Closed State = iota

	// Open is the tripped state: requests are refused until the cool-down
	// elapses.
	Open

	// HalfOpen is the trial state: a bounded number of requests are admitted to
	// probe recovery; their outcomes close or re-open the circuit.
	HalfOpen
)

// String returns the lowercase name ("closed" / "open" / "half_open").
func (s State) String() string {
	switch s {
	case Open:
		return "open"
	case HalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// Strategy names how the breaker counts failures.
type Strategy string

const (
	// Consecutive trips after [Spec.Threshold] failures in a row; any success
	// resets the count.
	Consecutive Strategy = "consecutive"

	// ErrorRate trips when the failure ratio over [Spec.Window] reaches
	// [Spec.RateThreshold], once at least [Spec.MinRequests] outcomes have been
	// observed. With a slow-call pair set, slow successes fold into the same
	// ratio.
	ErrorRate Strategy = "error-rate"

	// SlowCall trips when the slow ratio over [Spec.Window] reaches
	// [Spec.SlowRate]; a failed request counts as slow too.
	SlowCall Strategy = "slow-call"

	// ErrorCount trips when the failure count over [Spec.Window] reaches
	// [Spec.Threshold].
	ErrorCount Strategy = "error-count"
)

// EventListener receives circuit-breaker state transitions. It is invoked
// synchronously from inside the transition and must not call back into the same
// Breaker (it would deadlock).
type EventListener interface {
	OnBreakerStateChange(service string, from, to State)
}

// EventListenerSetter is implemented by a holder that can emit breaker
// transitions. Observability attaches its listener through it at wiring time;
// this package never imports the observer.
type EventListenerSetter interface {
	SetBreakerEventListener(EventListener)
}

// Spec is the breaker's own vocabulary: the thresholds one breaker is built
// for. Each direction assembles it from its policy, so the one state machine
// stays shared without either policy being privileged.
type Spec struct {
	Strategy      Strategy
	Threshold     int           // Consecutive/ErrorCount: failures that trip
	RateThreshold float64       // ErrorRate: bad/total ratio that trips
	MinRequests   int           // windowed strategies: minimum sample before tripping
	Window        time.Duration // windowed strategies: the rolling interval
	SlowThreshold time.Duration // a success at or above this counts as bad
	SlowRate      float64       // slow ratio that trips
	HalfOpenN     int           // trials admitted half-open; 0 and 1 both mean one
}

// Active reports whether any strategy is configured. A breaker built from an
// inactive Spec admits everything and its Record is a no-op.
func (s Spec) Active() bool {
	return s.Threshold > 0 || s.RateThreshold > 0 || (s.SlowThreshold > 0 && s.SlowRate > 0)
}

// ResolveStrategy picks the strategy a breaker should apply. An explicit
// strategy wins; a slow-call pair without one selects ErrorRate (slow successes
// are counted by the rate window); otherwise Consecutive is the default.
func ResolveStrategy(explicit Strategy, slowThreshold time.Duration, slowRate float64) Strategy {
	switch explicit {
	case ErrorRate, SlowCall, ErrorCount:
		return explicit
	}
	if slowThreshold > 0 && slowRate > 0 {
		return ErrorRate
	}
	return Consecutive
}

// Breaker is one circuit breaker. Its zero value — and a Breaker built from an
// inactive Spec — admits every request.
type Breaker struct {
	m *machine
}

// New builds the breaker for one protected unit (a service or an instance) from
// spec. listener may be nil; it receives state transitions.
func New(spec Spec, openFor time.Duration, service string, listener EventListener) *Breaker {
	if !spec.Active() {
		return &Breaker{}
	}
	if openFor <= 0 {
		openFor = defaultOpenFor
	}
	return &Breaker{m: newMachine(spec, openFor, service, listener)}
}

// Allow reports whether a request may proceed.
func (b *Breaker) Allow() bool {
	return b == nil || b.m == nil || b.m.allow()
}

// Record folds one completed request's outcome into the breaker. An inactive
// Breaker ignores everything.
func (b *Breaker) Record(success bool, dur time.Duration) {
	if b == nil || b.m == nil {
		return
	}
	b.m.record(success, dur)
}

// machine is one breaker's state. It supports four counting strategies over one
// open/half-open state machine:
//
//   - consecutive: trips after threshold failures in a row.
//   - error-rate: trips when bad/total over a rolling window reaches the ratio.
//   - slow-call: trips when the slow ratio over the window reaches the ratio.
//   - error-count: trips when the failure count over the window reaches the
//     threshold.
//
// Half-open admits up to halfOpenN trials; all must succeed to close, the first
// failure re-opens.
type machine struct {
	strategy Strategy
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
	listener EventListener
}

func newMachine(spec Spec, open time.Duration, service string, listener EventListener) *machine {
	c := &machine{
		strategy:      spec.Strategy,
		openFor:       open,
		threshold:     spec.Threshold,
		rateThreshold: spec.RateThreshold,
		minRequests:   spec.MinRequests,
		service:       service,
		listener:      listener,
	}
	if spec.SlowThreshold > 0 && spec.SlowRate > 0 {
		c.slowThreshold = spec.SlowThreshold
		c.slowRate = spec.SlowRate
	}
	if spec.HalfOpenN > 1 {
		c.halfOpenN = spec.HalfOpenN
	}
	if c.usesWindow() {
		if c.minRequests <= 0 {
			c.minRequests = 1
		}
		c.win = spec.Window
		if c.win <= 0 {
			c.win = time.Second
		}
		c.curStart = time.Now()
	}
	return c
}

func (c *machine) usesWindow() bool { return c.strategy != Consecutive }

// allow reports whether a request may proceed given the current state.
func (c *machine) allow() bool {
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
		c.notifyLocked(Open, HalfOpen)
	}
	select {
	case <-c.halfOpen:
		return true
	default:
		return false
	}
}

// record folds one completed request's outcome into the breaker.
func (c *machine) record(success bool, dur time.Duration) {
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
				c.notifyLocked(HalfOpen, Closed)
			}
			return
		}
		c.halfOpen = nil
		c.openedAt = time.Now()
		c.notifyLocked(HalfOpen, Open)
		return
	}

	if c.usesWindow() {
		c.recordWindow(success, slow)
		return
	}
	c.recordConsecutive(success)
}

func (c *machine) recordConsecutive(success bool) {
	if success {
		c.failures = 0
		c.openedAt = time.Time{}
		return
	}
	c.failures++
	if c.failures >= c.threshold {
		if c.openedAt.IsZero() {
			c.notifyLocked(Closed, Open)
		}
		c.openedAt = time.Now()
	}
}

func (c *machine) recordWindow(success, slow bool) {
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
		if c.strategy == SlowCall && slow {
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
		case ErrorCount:
			trip = estimateFails >= float64(c.threshold)
		case SlowCall:
			trip = c.slowRate > 0 && estimateSlows/estimateTotal >= c.slowRate
		default: // ErrorRate, with slow successes folded in
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
			c.notifyLocked(Closed, Open)
		}
		c.openedAt = now
	}
}

func (c *machine) closeLocked() {
	c.failures = 0
	c.openedAt = time.Time{}
	c.halfOpen = nil
	c.total, c.fails, c.slows = 0, 0, 0
	c.prevTotal, c.prevFails, c.prevSlows = 0, 0, 0
	c.curStart = time.Now()
}

func (c *machine) notifyLocked(from, to State) {
	if from == to || c.listener == nil {
		return
	}
	c.listener.OnBreakerStateChange(c.service, from, to)
}
