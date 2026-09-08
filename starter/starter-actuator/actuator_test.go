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

	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/stdlib/testing/assert"
)

// --- health DEGRADED aggregation -------------------------------------------

func TestReadiness_NonCriticalDownIsDegraded(t *testing.T) {
	cache := &health.Indicator{Name: "redis:cache", Probe: func(context.Context) error { return errors.New("connection refused") }, Optional: true}
	rec := doReadiness(readyServer(cache))

	// A tolerable dependency failing lowers the verdict to DEGRADED but keeps
	// 200: the pod stays in rotation and the failure stays observable.
	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Contains(`"DEGRADED"`)
	assert.String(t, rec.Body.String()).Contains(`"redis:cache"`)
}

func TestReadiness_CriticalDownDominatesDegraded(t *testing.T) {
	// One non-critical AND one critical failure: DOWN wins — any critical
	// failure takes the pod out of rotation regardless of degraded extras.
	cache := &health.Indicator{Name: "redis:cache", Probe: func(context.Context) error { return errors.New("connection refused") }, Optional: true}
	db := &health.Indicator{Name: "mysql:orders", Probe: func(context.Context) error { return errors.New("dial timeout") }, Optional: false}
	rec := doReadiness(readyServer(cache, db))

	assert.Number(t, rec.Code).Equal(http.StatusServiceUnavailable)
	assert.String(t, rec.Body.String()).Contains(`"DOWN"`)
	assert.String(t, rec.Body.String()).Contains(`"redis:cache"`)
}

func TestStartup_NonCriticalDownIsDegraded(t *testing.T) {
	cache := &health.Indicator{Name: "redis:cache", Probe: func(context.Context) error { return errors.New("connection refused") }, Optional: true}
	s := readyServer(cache)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	s.handleStartup(rec, req)

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Contains(`"DEGRADED"`)
}

// --- per-endpoint include filter -------------------------------------------

func registeredNames(s *Server) map[string]bool {
	// Recreate the registration decisions Run() would make (probes excluded:
	// they are always registered by design).
	names := map[string]bool{}
	for _, rt := range s.introspectionEndpoints() {
		path := endpointPath(rt.Pattern)
		if s.endpointEnabled(context.Background(), path, rt.Sensitive) {
			names[path] = true
		}
	}
	for _, ep := range s.Endpoints {
		path := endpointPath(ep.Pattern)
		if s.endpointEnabled(context.Background(), path, ep.Sensitive) {
			names[path] = true
		}
	}
	return names
}

func TestEndpointFilter_SensitiveDefaultOff(t *testing.T) {
	// A contributed endpoint that declares itself sensitive registers ONLY
	// when explicitly listed in include — even when the list is empty.
	s := &Server{Endpoints: []*endpoint.Endpoint{
		&endpoint.Endpoint{Pattern: "/metrics"},
		&endpoint.Endpoint{Pattern: "/pprof", Sensitive: true},
	}}
	names := registeredNames(s)
	assert.That(t, names["/metrics"]).True()
	assert.That(t, names["/pprof"]).False()

	s.EndpointInclude = "/pprof"
	names = registeredNames(s)
	assert.That(t, names["/pprof"]).True()
	assert.That(t, names["/metrics"]).False() // whitelist mode now
}

// --- authentication guard ---------------------------------------------------

func TestAuth_BearerToken(t *testing.T) {
	s := &Server{Token: "s3cret", Address: "127.0.0.1:9370"}
	h := s.buildHandler(context.Background())

	// No header / wrong token -> 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Number(t, rec.Code).Equal(http.StatusUnauthorized)

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)
	assert.Number(t, rec.Code).Equal(http.StatusUnauthorized)

	// Correct bearer -> 200 (probe endpoints stay open for K8s once authed).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	h.ServeHTTP(rec, req)
	assert.Number(t, rec.Code).Equal(http.StatusOK)
}

func TestAuth_Basic(t *testing.T) {
	s := &Server{Username: "admin", Password: "pw", Address: "127.0.0.1:9370"}
	h := s.buildHandler(context.Background())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/info", nil))
	assert.Number(t, rec.Code).Equal(http.StatusUnauthorized)
	assert.String(t, rec.Header().Get("WWW-Authenticate")).Contains("Basic")

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	req.SetBasicAuth("admin", "pw")
	h.ServeHTTP(rec, req)
	assert.Number(t, rec.Code).Equal(http.StatusOK)
}

func TestAuth_DisabledByDefault(t *testing.T) {
	// No credentials configured: the guard is a no-op and loopback serving
	// needs no header.
	s := &Server{Address: "127.0.0.1:9370"}
	h := s.buildHandler(context.Background())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Number(t, rec.Code).Equal(http.StatusOK)
}

func TestEndpointFilter_IncludeWhitelist(t *testing.T) {
	s := &Server{
		EndpointInclude: "/metrics",
		Endpoints:       []*endpoint.Endpoint{&endpoint.Endpoint{Pattern: "/metrics"}},
	}
	names := registeredNames(s)
	assert.That(t, names["/metrics"]).True()
	// Everything not in the whitelist is off.
	for _, off := range []string{"/info"} {
		assert.That(t, names[off]).False()
	}
}

func TestEndpointFilter_CaseInsensitiveAndWhitespace(t *testing.T) {
	s := &Server{EndpointInclude: " /Info , /Metrics ", Endpoints: []*endpoint.Endpoint{&endpoint.Endpoint{Pattern: "/metrics"}}}
	names := registeredNames(s)
	assert.That(t, names["/info"]).True()
	assert.That(t, names["/metrics"]).True()
}
