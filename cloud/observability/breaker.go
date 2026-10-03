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

package observability

// BreakerState is one state of a circuit breaker.
type BreakerState int

const (
	// BreakerClosed is the normal state: calls proceed and their outcomes feed
	// the breaker's failure counting.
	BreakerClosed BreakerState = iota
	// BreakerOpen is the tripped state: calls are rejected with the chain's
	// ErrCircuitOpen without invoking fn, until the cool-down elapses.
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
// transition. It must NOT call back into the same chain.Executor (it would deadlock
// on the breaker's lock) — emit a metric, log, or push onto a channel instead.
type BreakerEventListener interface {
	OnBreakerStateChange(service string, from, to BreakerState)
}

// BreakerEventListenerSetter is optionally implemented by chain executors whose
// engine can emit circuit-breaker state transitions. The default resilience
// driver implements it; the observe wrappers use it to attach metric + log
// emission. An engine that cannot observe transitions (e.g. one delegating to a
// library without state callbacks) simply does not implement it, and
// SetBreakerEventListener calls against it are a no-op detected by a failed
// type assertion.
type BreakerEventListenerSetter interface {
	SetBreakerEventListener(BreakerEventListener)
}
