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

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/stdlib/testing/assert"
)

// readyServer builds a Server that has already crossed its readiness barrier and
// is not draining, so handleReadiness reflects only the indicator aggregate.
func readyServer(inds ...*health.Indicator) *Server {
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
	down := &health.Indicator{Name: "mysql:orders", Probe: func(context.Context) error { return errors.New("dial timeout") }, Optional: false}
	rec := doReadiness(readyServer(down))

	// A critical dependency being DOWN takes the pod out of rotation.
	assert.Number(t, rec.Code).Equal(http.StatusServiceUnavailable)
}

func TestReadiness_RecoversToUp(t *testing.T) {
	up := &health.Indicator{Name: "mysql:orders", Probe: func(context.Context) error { return nil }, Optional: false}
	rec := doReadiness(readyServer(up))

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Contains(`"UP"`)
}

func TestReadiness_NonCriticalDownStaysUp(t *testing.T) {
	// A non-critical dependency (an optional cache) being DOWN is reported but
	// must not lower readiness — the pod keeps serving.
	down := &health.Indicator{Name: "redis:cache", Probe: func(context.Context) error { return errors.New("connection refused") }, Optional: true}
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
	ind := &health.Indicator{Name: "x"}
	assert.Slice(t, groupsOf(ind)).
		Equal([]health.Group{health.GroupReadiness, health.GroupStartup})
}

func TestGroupsOf_UsesExplicitGroupsWhenSet(t *testing.T) {
	ind := &health.Indicator{Name: "x", Groups: []health.Group{health.GroupLiveness}}
	assert.Slice(t, groupsOf(ind)).Equal([]health.Group{health.GroupLiveness})
}

func TestInGroup_DefaultCoversReadinessAndStartupNeverLiveness(t *testing.T) {
	ind := &health.Indicator{Name: "x"}
	assert.That(t, inGroup(ind, health.GroupReadiness)).True()
	assert.That(t, inGroup(ind, health.GroupStartup)).True()
	assert.That(t, inGroup(ind, health.GroupLiveness)).False()
}

func TestInGroup_ExplicitGroupOnly(t *testing.T) {
	ind := &health.Indicator{Name: "x", Groups: []health.Group{health.GroupLiveness}}
	assert.That(t, inGroup(ind, health.GroupLiveness)).True()
	assert.That(t, inGroup(ind, health.GroupReadiness)).False()
}
