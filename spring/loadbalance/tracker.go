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
	"sync"
	"time"

	"go-spring.org/spring/discovery"
)

// Tracker is the load-balancing layer's health-eviction (outlier detection)
// mechanism. It watches the success/failure outcome of balanced requests per
// endpoint and temporarily evicts an instance that fails repeatedly, then lets
// it back in on a half-open trial once a cool-down elapses.
//
// It is the LB-layer counterpart to the circuit breaker in
// [go-spring.org/spring/resilience]: same consecutive-failure + half-open
// semantics, but keyed by endpoint address and *queryable* so a [Pool] can drop
// bad instances from the candidate set proactively rather than only rejecting a
// call after it is routed. The same [DoneInfo.Err] signal that feeds a
// resilience Executor feeds a Tracker, so the two stay consistent without one
// depending on the other.
//
// A Tracker with Threshold <= 0 is disabled: [Tracker.Eligible] returns every
// endpoint and [Tracker.Record] is a no-op, so wiring one in stays a transparent
// pass-through until eviction is configured.
type Tracker struct {
	threshold int
	ejectFor  time.Duration

	// now is the clock, injectable so tests can drive ejection windows
	// deterministically. Defaults to time.Now.
	now func() time.Time

	mu     sync.Mutex
	states map[string]*ejectState
}

type ejectState struct {
	failures  int
	ejectedAt time.Time
	halfOpen  bool
}

// TrackerConfig configures outlier ejection.
type TrackerConfig struct {
	// Threshold is the number of consecutive failures that ejects an endpoint.
	// 0 (or negative) disables eviction entirely.
	Threshold int

	// EjectFor is how long an endpoint stays evicted before a half-open trial
	// request is allowed through. Defaults to 5s when unset (and eviction is
	// enabled), matching the resilience breaker default.
	EjectFor time.Duration
}

// NewTracker builds a [Tracker] from cfg.
func NewTracker(cfg TrackerConfig) *Tracker {
	ejectFor := cfg.EjectFor
	if ejectFor <= 0 {
		ejectFor = 5 * time.Second
	}
	return &Tracker{
		threshold: cfg.Threshold,
		ejectFor:  ejectFor,
		now:       time.Now,
		states:    map[string]*ejectState{},
	}
}

// Eligible returns the subset of eps that may currently receive traffic,
// dropping endpoints that are ejected and still cooling down. An ejected
// endpoint whose cool-down has elapsed is admitted (half-open trial) so it can
// prove itself. When the tracker is disabled it returns eps unchanged.
//
// Eligible never returns an empty slice when eps is non-empty solely due to
// eviction: if every endpoint is ejected it returns eps unchanged, because
// black-holing all traffic is worse than probing a degraded instance. The
// caller (Pool) applies its own final fallback too.
func (t *Tracker) Eligible(eps []discovery.Endpoint) []discovery.Endpoint {
	if t == nil || t.threshold <= 0 || len(eps) == 0 {
		return eps
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	out := eps[:0:0]
	for _, ep := range eps {
		if t.admitLocked(ep.Addr) {
			out = append(out, ep)
		}
	}
	if len(out) == 0 {
		return eps
	}
	return out
}

// admitLocked reports whether addr may receive a request, advancing an ejected
// endpoint into the half-open trial state once its cool-down has elapsed. Caller
// holds t.mu.
func (t *Tracker) admitLocked(addr string) bool {
	s := t.states[addr]
	if s == nil || s.ejectedAt.IsZero() {
		return true // never failed, or recovered
	}
	if t.now().Sub(s.ejectedAt) < t.ejectFor {
		return false // ejected, cooling down
	}
	s.halfOpen = true // cool-down elapsed: admit a trial request
	return true
}

// Record folds a request outcome back into the tracker. success=true clears the
// endpoint's failure state (or closes a half-open trial); success=false counts
// toward eviction (or re-ejects a failed half-open trial). It is a no-op when
// the tracker is disabled.
func (t *Tracker) Record(addr string, success bool) {
	if t == nil || t.threshold <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	s := t.states[addr]
	if s == nil {
		s = &ejectState{}
		t.states[addr] = s
	}
	if success {
		s.failures = 0
		s.ejectedAt = time.Time{}
		s.halfOpen = false
		return
	}
	if s.halfOpen {
		// Trial request failed: restart the cool-down window.
		s.halfOpen = false
		s.ejectedAt = t.now()
		return
	}
	s.failures++
	if s.failures >= t.threshold {
		s.ejectedAt = t.now()
	}
}

// Ejected reports whether addr is currently evicted and still cooling down. It
// is primarily a test/inspection helper; routing decisions go through Eligible.
func (t *Tracker) Ejected(addr string) bool {
	if t == nil || t.threshold <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.states[addr]
	if s == nil || s.ejectedAt.IsZero() {
		return false
	}
	return t.now().Sub(s.ejectedAt) < t.ejectFor
}
