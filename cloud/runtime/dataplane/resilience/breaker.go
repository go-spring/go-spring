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

// breaker.go is the outbound view of the shared circuit-breaker vocabulary.
// The state machine and its observation contract live in
// [go-spring.org/cloud/runtime/dataplane/breaker], because inbound guard runs the same one; this
// file keeps the names the resilience API has always exposed so observers and
// backend starters written against them keep compiling.

package resilience

import "go-spring.org/cloud/runtime/dataplane/breaker"

// BreakerState is one state of a circuit breaker.
type BreakerState = breaker.State

const (
	// BreakerClosed is the normal state: calls proceed and their outcomes feed
	// the breaker's failure counting.
	BreakerClosed = breaker.Closed

	// BreakerOpen is the tripped state: calls are rejected with the chain's
	// ErrCircuitOpen without invoking fn, until the cool-down elapses.
	BreakerOpen = breaker.Open

	// BreakerHalfOpen is the trial state: a bounded number of calls is admitted
	// to probe recovery; their outcomes close or re-open the circuit.
	BreakerHalfOpen = breaker.HalfOpen
)

// BreakerEventListener receives circuit-breaker state transitions. When a
// breaker trips, half-opens or recovers, OnBreakerStateChange is called with
// the service and the from/to states.
//
// The listener is invoked synchronously from inside the breaker's state
// transition. It must NOT call back into the same chain.Executor (it would
// deadlock on the breaker's lock) — emit a metric, log, or push onto a channel
// instead.
type BreakerEventListener = breaker.EventListener

// BreakerEventListenerSetter is optionally implemented by chain executors whose
// engine can emit circuit-breaker state transitions. The default resilience
// driver implements it; the observe wrappers use it to attach metric + log
// emission. An engine that cannot observe transitions (e.g. one delegating to a
// library without state callbacks) simply does not implement it, and
// SetBreakerEventListener calls against it are a no-op detected by a failed
// type assertion.
type BreakerEventListenerSetter = breaker.EventListenerSetter

// BreakerStrategy selects how the circuit breaker counts failures. It is the
// shared strategy vocabulary; the constants below keep the historical names.
type BreakerStrategy = breaker.Strategy

const (
	// BreakerConsecutive trips after [ClientPolicy.ErrorThreshold] failures in a
	// row (any success resets the count). It is the default when BreakerStrategy
	// is empty, matching the historical behavior.
	BreakerConsecutive = breaker.Consecutive

	// BreakerErrorRate trips when the failure ratio over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.ErrorRateThreshold], once at least [ClientPolicy.MinRequests]
	// requests have been observed in the window. Use it for high-throughput
	// services where a burst of failures or a steady partial-failure rate is
	// more meaningful than a consecutive run.
	BreakerErrorRate = breaker.ErrorRate

	// BreakerSlowCall trips when the SLOW-call ratio over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.SlowCallRateThreshold], a call being slow at or above
	// [ClientPolicy.SlowCallDurationThreshold] whether it failed or succeeded — a
	// downstream that never errors but answers slowly is unhealthy too. Unlike the
	// slow-call pair under [BreakerErrorRate] (where slow successes fold into the
	// error rate), this strategy counts ONLY slowness. It is the neutral
	// counterpart of sentinel's SlowRequestRatio and OpenSergo's SlowRequestRatio
	// circuit-breaker strategy.
	BreakerSlowCall = breaker.SlowCall

	// BreakerErrorCount trips when the failure COUNT over [ClientPolicy.BreakerWindow]
	// reaches [ClientPolicy.ErrorThreshold], once at least [ClientPolicy.MinRequests]
	// requests have been observed in the window. Unlike [BreakerConsecutive] (which
	// counts an unbounded run and resets on any success), it is a windowed count —
	// the neutral counterpart of sentinel's ErrorCount and OpenSergo's ErrorCount
	// circuit-breaker strategy, so a policy written against either maps exactly.
	BreakerErrorCount = breaker.ErrorCount
)
