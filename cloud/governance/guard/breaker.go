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

// breaker.go is the inbound view of the shared circuit breaker: it maps a
// [Policy] onto the direction-neutral [go-spring.org/cloud/governance/breaker.Spec] and
// translates this package's [Outcome] into its success boolean. The state
// machine itself lives in the shared leaf package, because outbound resilience
// runs the same one.

package guard

import (
	"time"

	"go-spring.org/cloud/governance/breaker"
)

// BreakerState is one state of a circuit breaker.
type BreakerState = breaker.State

const (
	// BreakerClosed is the normal state: requests proceed and their outcomes
	// feed the breaker's failure counting.
	BreakerClosed = breaker.Closed

	// BreakerOpen is the tripped state: requests are refused with
	// [ErrCircuitOpen] until the cool-down elapses.
	BreakerOpen = breaker.Open

	// BreakerHalfOpen is the trial state: a bounded number of requests are
	// admitted to probe recovery; their outcomes close or re-open the circuit.
	BreakerHalfOpen = breaker.HalfOpen
)

// BreakerEventListener receives circuit-breaker state transitions. It is
// invoked synchronously from inside the transition and must not call back into
// the same Guard (it would deadlock).
type BreakerEventListener = breaker.EventListener

// BreakerEventListenerSetter is implemented by a [Guard] that can emit breaker
// transitions. Observability attaches its listener through it at wiring time;
// the engine never imports the observer.
type BreakerEventListenerSetter = breaker.EventListenerSetter

// Breaker is the circuit-breaker stage as an independent object. It is the same
// implementation [Guard] uses; a caller assembling a custom middleware can take
// one directly.
//
// A Breaker built from a policy with no active strategy admits every request
// and Record is a no-op.
type Breaker struct {
	b *breaker.Breaker
}

// NewBreaker builds the breaker for one server instance from p. listener may be
// nil; it receives state transitions.
func NewBreaker(p Policy, service string, listener BreakerEventListener) *Breaker {
	spec := breaker.Spec{
		Strategy:      breaker.ResolveStrategy(breaker.Strategy(p.BreakerStrategy), p.SlowCallDurationThreshold, p.SlowCallRateThreshold),
		Threshold:     p.ErrorThreshold,
		RateThreshold: p.ErrorRateThreshold,
		MinRequests:   p.MinRequests,
		Window:        p.BreakerWindow,
		SlowThreshold: p.SlowCallDurationThreshold,
		SlowRate:      p.SlowCallRateThreshold,
		HalfOpenN:     p.HalfOpenRequests,
	}
	return &Breaker{b: breaker.New(spec, p.OpenDuration, service, listener)}
}

// Allow reports whether a request may proceed. A nil or inactive Breaker always
// allows.
func (b *Breaker) Allow() bool {
	return b == nil || b.b.Allow()
}

// Record folds one completed request's outcome into the breaker. OutcomeUnknown
// is ignored; an inactive Breaker ignores everything.
func (b *Breaker) Record(o Outcome, dur time.Duration) {
	if b == nil || o == OutcomeUnknown {
		return
	}
	b.b.Record(o == OutcomeSuccess, dur)
}
