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

	"go-spring.org/spring/actuator/health"
)

// componentStatus is the per-indicator entry reported under the probe endpoints.
type componentStatus struct {
	Status health.Status `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// checkGroup runs every indicator that contributes to the given probe group and
// reports the aggregate status plus per-component detail. An indicator that does
// not declare its groups defaults to readiness+startup (never liveness), so a
// dependency check can never fail a liveness probe. A DOWN indicator lowers the
// aggregate only when it is critical (the default); a non-critical indicator's
// failure is still reported per-component but does not take the pod out of
// rotation. With no matching indicator the group is trivially UP.
func (s *Server) checkGroup(ctx context.Context, group health.Group) (health.Status, map[string]componentStatus) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	overall := health.StatusUp
	components := make(map[string]componentStatus)
	for _, ind := range s.Indicators {
		if !health.InGroup(ind, group) {
			continue
		}
		if err := ind.CheckHealth(ctx); err != nil {
			components[ind.HealthName()] = componentStatus{Status: health.StatusDown, Error: err.Error()}
			if health.IsCritical(ind) {
				overall = health.StatusDown
			}
		} else {
			components[ind.HealthName()] = componentStatus{Status: health.StatusUp}
		}
	}
	return overall, components
}

// writeProbe writes a probe response: the aggregate status, 503 when down, and
// the per-component map when any indicator contributed.
func writeProbe(w http.ResponseWriter, status health.Status, components map[string]componentStatus) {
	code := http.StatusOK
	if status == health.StatusDown {
		code = http.StatusServiceUnavailable
	}
	body := map[string]any{"status": status}
	if len(components) > 0 {
		body["components"] = components
	}
	writeJSON(w, code, body)
}

// handleLiveness backs the Kubernetes livenessProbe (/healthz, alias /health):
// the process is up and serving. It consults only indicators that explicitly
// declare the liveness group (usually none) — a degraded dependency should fail
// readiness, not trigger a liveness restart.
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	status, components := s.checkGroup(r.Context(), health.GroupLiveness)
	writeProbe(w, status, components)
}

// handleReadiness backs the Kubernetes readinessProbe (/readyz, alias
// /readiness): whether the app can currently serve traffic. Returns 503
// OUT_OF_SERVICE while the readiness barrier has not been crossed or during
// graceful drain; otherwise the aggregate of every readiness-group indicator.
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() || s.draining.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "OUT_OF_SERVICE",
		})
		return
	}
	status, components := s.checkGroup(r.Context(), health.GroupReadiness)
	writeProbe(w, status, components)
}

// handleStartup backs the Kubernetes startupProbe (/startupz, alias /startup):
// 503 OUT_OF_SERVICE until the app has finished starting AND every startup-group
// indicator passes, then 200. It is unaffected by drain — once startup has
// succeeded the kubelet stops polling it and hands off to the liveness probe, so
// a slow boot is not mistaken for a hung process and killed.
func (s *Server) handleStartup(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "OUT_OF_SERVICE",
		})
		return
	}
	status, components := s.checkGroup(r.Context(), health.GroupStartup)
	writeProbe(w, status, components)
}
