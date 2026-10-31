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

package StarterGovernanceSentinel

import (
	"sync"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"

	breaker "go-spring.org/cloud/governance/capability/circuitbreaker"
)

// This file is the sentinel driver's own state plane. sentinel-golang exposes
// state transitions ONLY through a process-wide listener and offers no state
// query API, so the driver keeps a mirror per resource and publishes that mirror
// as metrics (see observe.go). Nothing here reaches into another layer's
// observability: the mirror belongs to the organ that owns the breaker.
//
// breakerRoutes maps a sentinel resource name (a service label) to the mirror
// that reports its state. sentinel-golang's StateChangeListener is a
// process-wide singleton (circuitbreaker.stateChangeListeners), not per-rule, so
// the one registered routeListener demultiplexes by rule.Resource. A route is
// added when an executor loads an active breaker rule and removed by Close.
var breakerRoutes sync.Map // service label (string) -> *breakerMirror

// breakerMirror is the driver's own view of one sentinel breaker: the current
// state and the monotonic transition counts the metrics report. The update runs
// synchronously inside sentinel's transition, so it must stay cheap and must
// not call OTel or take another layer's lock.
type breakerMirror struct {
	mu        sync.Mutex
	state     breaker.State
	trips     uint64 // entries into Open
	halfOpens uint64 // entries into HalfOpen
	recovers  uint64 // entries into Closed
}

func newBreakerMirror() *breakerMirror {
	return &breakerMirror{state: breaker.StateClosed}
}

// transition records one sentinel state change. Sentinel does not tell us
// whether a "closed" event is a recovery or the initial state, so the mirror
// starts Closed at creation and every event after that is a real transition.
func (m *breakerMirror) transition(to breaker.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = to
	switch to {
	case breaker.StateOpen:
		m.trips++
	case breaker.StateHalfOpen:
		m.halfOpens++
	case breaker.StateClosed:
		m.recovers++
	}
}

// stats reads the mirror's current value for a collection.
func (m *breakerMirror) stats() breaker.Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return breaker.Stats{
		State:     m.state,
		Trips:     m.trips,
		HalfOpens: m.halfOpens,
		Recovers:  m.recovers,
	}
}

// registerBreakerRoute ensures a mirror exists for service and returns it. A
// second executor for the same label reuses the first mirror rather than
// resetting it: sentinel itself keys state by resource, not by executor.
func registerBreakerRoute(service string) *breakerMirror {
	m := newBreakerMirror()
	actual, _ := breakerRoutes.LoadOrStore(service, m)
	return actual.(*breakerMirror)
}

// unregisterBreakerRoute drops a service's mirror when its executor is closed,
// so a torn-down executor cannot keep reporting the last state it saw and the
// demultiplexer holds no stale entry.
func unregisterBreakerRoute(service string) {
	breakerRoutes.Delete(service)
}

// routeListener is registered once with sentinel-golang and updates the mirror
// of the resource whose breaker changed. It implements
// circuitbreaker.StateChangeListener.
type routeListener struct{}

func (routeListener) OnTransformToClosed(_ circuitbreaker.State, rule circuitbreaker.Rule) {
	transitionBreaker(rule.Resource, circuitbreaker.Closed)
}

func (routeListener) OnTransformToOpen(_ circuitbreaker.State, rule circuitbreaker.Rule, _ any) {
	transitionBreaker(rule.Resource, circuitbreaker.Open)
}

func (routeListener) OnTransformToHalfOpen(_ circuitbreaker.State, rule circuitbreaker.Rule) {
	transitionBreaker(rule.Resource, circuitbreaker.HalfOpen)
}

// transitionBreaker looks up the mirror for a service label and records the
// translated state. A resource with no route (e.g. a breaker rule loaded
// outside go-spring) is silently ignored.
func transitionBreaker(service string, to circuitbreaker.State) {
	v, ok := breakerRoutes.Load(service)
	if !ok {
		return
	}
	v.(*breakerMirror).transition(toBreakerState(to))
}

// toBreakerState maps sentinel-golang's circuitbreaker.State onto the
// breaker.State enum (same three states, different package).
func toBreakerState(s circuitbreaker.State) breaker.State {
	switch s {
	case circuitbreaker.Open:
		return breaker.StateOpen
	case circuitbreaker.HalfOpen:
		return breaker.StateHalfOpen
	default:
		return breaker.StateClosed
	}
}

// routeListenerRegistered guards the one-time registration of routeListener
// with sentinel-golang. Registered on the first active breaker rule so any
// sentinelExecutor created after importing starter-governance-sentinel has its
// events routed.
var routeListenerRegistered sync.Once

func ensureRouteListener() {
	routeListenerRegistered.Do(func() {
		circuitbreaker.RegisterStateChangeListeners(routeListener{})
	})
}
