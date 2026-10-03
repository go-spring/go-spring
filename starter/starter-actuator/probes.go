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

package StarterActuator

import (
	"context"
	"net/http"
	"slices"
	"sync"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/stdlib/goutil"
	"go-spring.org/stdlib/httputil"
)

// componentStatus is the per-indicator entry reported under the probe endpoints.
type componentStatus struct {
	Status health.Status `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// groupsOf returns the probe groups an indicator contributes to, applying the
// default (readiness + startup) when Groups is empty.
func groupsOf(indicator *health.Indicator) []health.Group {
	if len(indicator.Groups) > 0 {
		return indicator.Groups
	}
	return []health.Group{health.GroupReadiness, health.GroupStartup}
}

// inGroup reports whether an indicator contributes to the given probe group.
func inGroup(indicator *health.Indicator, group health.Group) bool {
	return slices.Contains(groupsOf(indicator), group)
}

// checkGroup runs every indicator that contributes to the given probe group and
// reports the aggregate status plus per-component detail. An indicator that does
// not declare its groups defaults to readiness+startup (never liveness), so a
// dependency check can never fail a liveness probe. The aggregation maps:
//
//	every indicator UP                        -> UP
//	only non-critical indicators DOWN         -> DEGRADED (200: still serving)
//	any critical indicator DOWN               -> DOWN (503: out of rotation)
//
// A non-critical indicator's failure is still reported per-component but does
// not take the pod out of rotation. With no matching indicator the group is
// trivially UP. Indicators run concurrently: the CheckTimeout budget is shared
// by the whole sweep, so serial probing would let one slow dependency starve
// every later indicator of its deadline and report spurious DOWNs.
func (s *Server) checkGroup(ctx context.Context, group health.Group) (health.Status, map[string]componentStatus) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.CheckTimeout)
	defer cancel()

	matched := make([]*health.Indicator, 0, len(s.indicators))
	for _, indicator := range s.indicators {
		if inGroup(indicator, group) {
			matched = append(matched, indicator)
		}
	}

	statuses := make([]componentStatus, len(matched))
	var wg sync.WaitGroup
	for i, indicator := range matched {
		wg.Add(1)
		goutil.Go(ctx, func(ctx context.Context) {
			defer wg.Done()
			if err := indicator.Probe(ctx); err != nil {
				statuses[i] = componentStatus{Status: health.StatusDown, Error: err.Error()}
			} else {
				statuses[i] = componentStatus{Status: health.StatusUp}
			}
		}, goutil.InheritCancel)
	}
	wg.Wait()

	overall := health.StatusUp
	degraded := false
	components := make(map[string]componentStatus, len(matched))
	for i, indicator := range matched {
		components[indicator.Name] = statuses[i]
		if statuses[i].Status == health.StatusDown {
			if !indicator.Optional {
				overall = health.StatusDown
			} else {
				degraded = true
			}
		}
	}
	if overall == health.StatusUp && degraded {
		overall = health.StatusDegraded
	}
	return overall, components
}

// writeProbe writes a probe response: the aggregate status, 503 only when DOWN,
// and the per-component map when any indicator contributed. DEGRADED keeps 200:
// the app is still serving traffic, only a tolerable dependency is failing, and
// the failure detail is visible in the components map.
func writeProbe(w http.ResponseWriter, status health.Status, components map[string]componentStatus) {
	code := http.StatusOK
	if status == health.StatusDown {
		code = http.StatusServiceUnavailable
	}
	body := map[string]any{"status": status}
	if len(components) > 0 {
		body["components"] = components
	}
	httputil.WriteJSON(w, code, body)
}

// The three probe handlers have deliberately different gates, because the
// three Kubernetes probes have asymmetric failure consequences and no call
// order — the kubelet polls each independently:
//
//	liveness fails   -> the container is KILLED and restarted
//	readiness fails  -> the pod just stops receiving traffic, nothing dies
//	startup fails    -> same as liveness, but only until first success
//
// During graceful drain EVERY probe answers 503 OUT_OF_SERVICE: the instance
// is going away, and every poller — kubelet, an external health checker —
// should hear that from any endpoint it happens to watch. Outside drain, the
// gates differ by consequence: liveness failure kills the process, so a slow
// boot or a sick dependency must never trip it (an unresponsive process is
// the one thing liveness may report); readiness failure only costs traffic,
// so any critical dependency down may answer 503.
//
// Serving starts only after the readiness barrier (see Run), so by the time
// any probe is answered the app is already ready: no handler gates on the
// readiness flag, and the not-ready window is observed by the kubelet as
// connection refused burning the startupProbe budget (the README's working
// model), not as a 503 from these handlers.
//
// handleLiveness backs the Kubernetes livenessProbe (/healthz, alias /health):
// the process is up and serving — answering is itself the proof of life, so
// the only gate is drain: during graceful shutdown it reports 503
// OUT_OF_SERVICE like the other probes, telling every poller (kubelet or an
// external health checker) that this instance is going away. It consults only
// indicators that explicitly declare the liveness group (usually none) — a
// degraded dependency should fail readiness, not trigger a liveness restart.
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if s.draining.Load() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "OUT_OF_SERVICE",
		})
		return
	}
	status, components := s.checkGroup(r.Context(), health.GroupLiveness)
	writeProbe(w, status, components)
}

// handleReadiness backs the Kubernetes readinessProbe (/readyz, alias
// /readiness): whether the app can currently serve traffic. Returns 503
// OUT_OF_SERVICE during graceful drain; otherwise the aggregate of every
// readiness-group indicator.
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if s.draining.Load() {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "OUT_OF_SERVICE",
		})
		return
	}
	status, components := s.checkGroup(r.Context(), health.GroupReadiness)
	writeProbe(w, status, components)
}

// handleStartup backs the Kubernetes startupProbe (/startupz, alias /startup):
// the aggregate of every startup-group indicator. No gate: serving implies the
// app finished starting, so this handler only adds the startup-group health
// detail on top. It ignores drain — once startup has succeeded the kubelet
// stops polling it and hands off to the liveness probe.
func (s *Server) handleStartup(w http.ResponseWriter, r *http.Request) {
	status, components := s.checkGroup(r.Context(), health.GroupStartup)
	writeProbe(w, status, components)
}
