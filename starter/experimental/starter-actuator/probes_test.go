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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/spring/cloud/actuator/health"
	"go-spring.org/stdlib/testing/assert"
)

// stubIndicator is a test indicator whose health, criticality, and probe
// groups are controlled by the test. A nil groups field means "no opinion",
// which groupsOf routes to the default (readiness + startup).
type stubIndicator struct {
	name     string
	err      error
	critical bool
	groups   []health.Group
}

func (s stubIndicator) HealthName() string { return s.name }

func (s stubIndicator) CheckHealth(context.Context) error { return s.err }

func (s stubIndicator) IsCritical() bool { return s.critical }

func (s stubIndicator) HealthGroups() []health.Group { return s.groups }

// readyServer builds a Server that has already crossed its readiness barrier and
// is not draining, so handleReadiness reflects only the indicator aggregate.
func readyServer(inds ...health.Indicator) *Server {
	s := &Server{Indicators: inds}
	s.ready.Store(true)
	return s
}

func doReadiness(s *Server) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	s.handleReadiness(rec, req)
	return rec
}

func TestReadiness_CriticalDownIsOutOfService(t *testing.T) {
	down := stubIndicator{name: "mysql:orders", err: errors.New("dial timeout"), critical: true}
	rec := doReadiness(readyServer(down))

	// A critical dependency being DOWN takes the pod out of rotation.
	assert.Number(t, rec.Code).Equal(http.StatusServiceUnavailable)
}

func TestReadiness_RecoversToUp(t *testing.T) {
	up := stubIndicator{name: "mysql:orders", err: nil, critical: true}
	rec := doReadiness(readyServer(up))

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Contains(`"UP"`)
}

func TestReadiness_NonCriticalDownStaysUp(t *testing.T) {
	// A non-critical dependency (an optional cache) being DOWN is reported but
	// must not lower readiness — the pod keeps serving.
	down := stubIndicator{name: "redis:cache", err: errors.New("connection refused"), critical: false}
	rec := doReadiness(readyServer(down))

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	// Its per-component status is still surfaced for observability.
	assert.String(t, rec.Body.String()).Contains(`"redis:cache"`)
}

// The next four tests pin the default-routing invariant: an indicator that
// declines to declare groups is routed to readiness + startup (never liveness),
// so a dependency check can never trigger a pod restart. This logic now lives
// in the collector (moved out of the health package), so it is tested here.

func TestGroupsOf_AppliesDefaultWhenEmpty(t *testing.T) {
	ind := stubIndicator{name: "x"}
	assert.Slice(t, groupsOf(ind)).
		Equal([]health.Group{health.GroupReadiness, health.GroupStartup})
}

func TestGroupsOf_UsesExplicitGroupsWhenSet(t *testing.T) {
	ind := stubIndicator{name: "x", groups: []health.Group{health.GroupLiveness}}
	assert.Slice(t, groupsOf(ind)).Equal([]health.Group{health.GroupLiveness})
}

func TestInGroup_DefaultCoversReadinessAndStartupNeverLiveness(t *testing.T) {
	ind := stubIndicator{name: "x"}
	assert.That(t, inGroup(ind, health.GroupReadiness)).True()
	assert.That(t, inGroup(ind, health.GroupStartup)).True()
	assert.That(t, inGroup(ind, health.GroupLiveness)).False()
}

func TestInGroup_ExplicitGroupOnly(t *testing.T) {
	ind := stubIndicator{name: "x", groups: []health.Group{health.GroupLiveness}}
	assert.That(t, inGroup(ind, health.GroupLiveness)).True()
	assert.That(t, inGroup(ind, health.GroupReadiness)).False()
}
