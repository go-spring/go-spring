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
	"sync/atomic"
	"time"

	"go-spring.org/cloud/discovery"
)

// suspendState is one endpoint address's suspension bookkeeping. suspendedAt
// doubles as the state flag: zero means "not suspended", non-zero is when the
// current cool-down window started.
type suspendState struct {
	failures    int
	halfOpen    bool
	suspendedAt time.Time
}

// TrackerConfig is the configuration for a [Tracker].
type TrackerConfig struct {
	// Threshold is the number of consecutive failures that suspends an endpoint.
	// 0 (or negative) disables suspension entirely.
	Threshold int

	// SuspendFor is how long an endpoint stays suspended before a half-open trial
	// request is allowed through. Defaults to 5s when unset (and suspension is
	// enabled), matching the resilience breaker default.
	SuspendFor time.Duration
}

// Tracker is the load-balancing layer's outlier-suspension mechanism. It
// watches the success/failure outcome of balanced requests per endpoint and
// temporarily suspends an instance that fails repeatedly, then lets it back in
// on a half-open trial once a cool-down elapses.
//
// It is the LB-layer counterpart to the circuit breaker in
// [go-spring.org/cloud/governance/resilience]: same consecutive-failure + half-open
// semantics, but keyed by endpoint address and *queryable* so a [Pool] can drop
// bad instances from the candidate set proactively rather than only rejecting a
// call after it is routed. The same Complete-err signal that feeds a
// resilience Executor feeds a Tracker, so the two stay consistent without one
// depending on the other.
//
// A Tracker with Threshold <= 0 is disabled: [Tracker.Allows] returns every
// endpoint and [Tracker.Record] is a no-op. It is safe to attach a disabled
// Tracker — it has no effect until Threshold is set.
//
// The config is swappable in place via [Tracker.SetConfig] so a hot-reloaded
// governance policy can retune suspension without rebuilding the tracker —
// which matters because the per-endpoint state (failure counts, cool-down
// windows) is exactly what must survive the retune.
type Tracker struct {
	// cfg is the normalized [TrackerConfig] currently in force, swapped
	// atomically so [Tracker.Admissible] can read it without taking the state
	// mutex on the hot path. SuspendFor's default is applied on the way in, by
	// NewTracker and SetConfig alike.
	cfg atomic.Pointer[TrackerConfig]

	// now is the clock, injectable so tests can drive suspension windows
	// deterministically. Defaults to time.Now.
	now func() time.Time

	mu     sync.Mutex
	states map[string]*suspendState
}

// NewTracker builds a [Tracker] from cfg. A zero SuspendFor is normalized to
// the 5s default here.
func NewTracker(cfg TrackerConfig) *Tracker {
	t := &Tracker{
		now:    time.Now,
		states: map[string]*suspendState{},
	}
	t.SetConfig(cfg)
	return t
}

// SetConfig replaces the suspension config in place. Per-endpoint state is
// kept: endpoints already suspended stay suspended (their remaining cool-down
// is re-evaluated against the new SuspendFor), and failure counts carry over.
// Safe for concurrent use with [Tracker.Admissible] and [Tracker.Record].
func (t *Tracker) SetConfig(cfg TrackerConfig) {
	if cfg.SuspendFor <= 0 {
		cfg.SuspendFor = 5 * time.Second
	}
	t.cfg.Store(&cfg)
}

// Config returns the suspension config currently in force (with the SuspendFor
// default applied). It is primarily an inspection and test helper.
func (t *Tracker) Config() TrackerConfig {
	if t == nil {
		return TrackerConfig{}
	}
	return *t.cfg.Load()
}

// Allows returns the subset of eps that may currently receive traffic:
// endpoints that are suspended and still cooling down are dropped, while a
// suspended endpoint whose cool-down has elapsed is admitted (half-open
// trial). When the tracker is disabled it returns eps unchanged.
//
// If every endpoint is suspended it falls back to eps — black-holing all
// traffic is worse than probing a degraded instance. Callers that own this
// fallback themselves should use [Admissible] instead.
func (t *Tracker) Allows(eps []discovery.Endpoint) []discovery.Endpoint {
	out := t.Admissible(eps)
	if len(out) == 0 {
		return eps
	}
	return out
}

// Admissible is [Allows] without the fallback: when every endpoint is
// suspended it returns an empty slice, leaving that decision to the caller.
func (t *Tracker) Admissible(eps []discovery.Endpoint) []discovery.Endpoint {
	if t == nil {
		return eps
	}
	cfg := *t.cfg.Load()
	if cfg.Threshold <= 0 || len(eps) == 0 {
		return eps
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	out := eps[:0:0]
	for _, ep := range eps {
		if t.admitLocked(ep.Addr, cfg.SuspendFor) {
			out = append(out, ep)
		}
	}
	return out
}

// admitLocked reports whether addr may receive a request, advancing a suspended
// endpoint into the half-open trial state once its cool-down has elapsed. Caller
// holds t.mu.
func (t *Tracker) admitLocked(addr string, suspendFor time.Duration) bool {
	s := t.states[addr]
	if s == nil || s.suspendedAt.IsZero() {
		return true // never failed, or recovered
	}
	if t.now().Sub(s.suspendedAt) < suspendFor {
		return false // suspended, cooling down
	}
	s.halfOpen = true // cool-down elapsed: admit a trial request
	return true
}

// Record folds a request outcome back into the tracker. success=true clears the
// endpoint's failure state (or closes a half-open trial); success=false counts
// toward suspension (or re-suspends a failed half-open trial). It is a no-op when
// the tracker is disabled.
func (t *Tracker) Record(addr string, success bool) {
	if t == nil {
		return
	}
	cfg := *t.cfg.Load()
	if cfg.Threshold <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	s := t.states[addr]
	if s == nil {
		s = &suspendState{}
		t.states[addr] = s
	}
	if success {
		s.failures = 0
		s.suspendedAt = time.Time{}
		s.halfOpen = false
		return
	}
	if s.halfOpen {
		// Trial request failed: restart the cool-down window.
		s.halfOpen = false
		s.suspendedAt = t.now()
		return
	}
	s.failures++
	if s.failures >= cfg.Threshold {
		s.suspendedAt = t.now()
	}
}

// Suspended reports whether addr is currently suspended and still cooling down. It
// is primarily a test/inspection helper; routing decisions go through Allows.
func (t *Tracker) Suspended(addr string) bool {
	if t == nil {
		return false
	}
	cfg := *t.cfg.Load()
	if cfg.Threshold <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.states[addr]
	if s == nil || s.suspendedAt.IsZero() {
		return false
	}
	return t.now().Sub(s.suspendedAt) < cfg.SuspendFor
}
