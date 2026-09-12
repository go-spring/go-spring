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

package StarterResilience

import (
	"sync"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"

	"go-spring.org/cloud/governance/resilience"
)

// breakerRoutes maps a sentinel resource name to the resilience BreakerEventListener
// that wants its state transitions. sentinel-golang's StateChangeListener is a
// process-wide singleton (circuitbreaker.stateChangeListeners), not per-rule, so a
// single registered routeListener demultiplexes by rule.Resource to the right
// per-executor listener. Keyed by resource because a sentinelExecutor builds one
// breaker rule per resource (ensureRules), and the resource string is globally
// unique within a process (derived from resilience.ResourceLabel).
var breakerRoutes sync.Map // resource (string) -> resilience.BreakerEventListener

func registerBreakerRoute(resource string, l resilience.BreakerEventListener) {
	if l == nil {
		return
	}
	breakerRoutes.Store(resource, l)
}

// routeListener is registered once with sentinel-golang and forwards every
// circuit-breaker state transition to the per-resource resilience listener
// stored in breakerRoutes. It implements circuitbreaker.StateChangeListener.
type routeListener struct{}

func (routeListener) OnTransformToClosed(prev circuitbreaker.State, rule circuitbreaker.Rule) {
	forwardBreaker(rule.Resource, prev, circuitbreaker.Closed)
}

func (routeListener) OnTransformToOpen(prev circuitbreaker.State, rule circuitbreaker.Rule, _ any) {
	forwardBreaker(rule.Resource, prev, circuitbreaker.Open)
}

func (routeListener) OnTransformToHalfOpen(prev circuitbreaker.State, rule circuitbreaker.Rule) {
	forwardBreaker(rule.Resource, prev, circuitbreaker.HalfOpen)
}

// forwardBreaker looks up the listener for resource and translates the sentinel
// transition into a resilience.BreakerState change. A resource with no route
// (e.g. a breaker rule loaded outside go-spring) is silently ignored.
func forwardBreaker(resource string, from, to circuitbreaker.State) {
	v, ok := breakerRoutes.Load(resource)
	if !ok {
		return
	}
	v.(resilience.BreakerEventListener).OnBreakerStateChange(resource, toBreakerState(from), toBreakerState(to))
}

// toBreakerState maps sentinel-golang's circuitbreaker.State onto the
// resilience.BreakerState enum (same three states, different package).
func toBreakerState(s circuitbreaker.State) resilience.BreakerState {
	switch s {
	case circuitbreaker.Open:
		return resilience.BreakerOpen
	case circuitbreaker.HalfOpen:
		return resilience.BreakerHalfOpen
	default:
		return resilience.BreakerClosed
	}
}

// routeListenerRegistered guards the one-time registration of routeListener
// with sentinel-golang. Registered in init so any sentinelExecutor created after
// importing starter-resilience has its events routed.
var routeListenerRegistered sync.Once

func ensureRouteListener() {
	routeListenerRegistered.Do(func() {
		circuitbreaker.RegisterStateChangeListeners(routeListener{})
	})
}
