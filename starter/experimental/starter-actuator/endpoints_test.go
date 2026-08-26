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
	"strings"
	"testing"

	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/stdlib/testing/assert"
)

// fakeLister is a test BeanLister standing in for the bean registry the gs
// core does not export (see beans.go for the boundary).
type fakeLister struct {
	beans []BeanDescriptor
}

func (f fakeLister) Beans() []BeanDescriptor { return f.beans }

// --- health DEGRADED aggregation -------------------------------------------

func TestReadiness_NonCriticalDownIsDegraded(t *testing.T) {
	cache := stubIndicator{name: "redis:cache", err: errors.New("connection refused"), critical: false}
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
	cache := stubIndicator{name: "redis:cache", err: errors.New("connection refused"), critical: false}
	db := stubIndicator{name: "mysql:orders", err: errors.New("dial timeout"), critical: true}
	rec := doReadiness(readyServer(cache, db))

	assert.Number(t, rec.Code).Equal(http.StatusServiceUnavailable)
	assert.String(t, rec.Body.String()).Contains(`"DOWN"`)
	assert.String(t, rec.Body.String()).Contains(`"redis:cache"`)
}

func TestStartup_NonCriticalDownIsDegraded(t *testing.T) {
	cache := stubIndicator{name: "redis:cache", err: errors.New("connection refused"), critical: false}
	s := readyServer(cache)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	s.handleStartup(rec, req)

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Contains(`"DEGRADED"`)
}

// --- /beans ----------------------------------------------------------------

func doBeans(s *Server) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/beans", nil)
	s.handleBeans(rec, req)
	return rec
}

func TestBeans_ListsRegistry(t *testing.T) {
	s := &Server{BeanRegistry: fakeLister{beans: []BeanDescriptor{
		{Name: "orderService", Type: "*app.OrderService"},
		{Name: "cache", Type: "*app.Cache"},
	}}}
	rec := doBeans(s)

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	body := rec.Body.String()
	assert.String(t, body).Contains(`"orderService"`)
	assert.String(t, body).Contains(`"*app.OrderService"`)
	assert.String(t, body).Contains(`"cache"`)
	assert.That(t, strings.Contains(body, `"note"`)).False()
}

func TestBeans_WithoutRegistryReportsBoundary(t *testing.T) {
	// No BeanLister bean: the endpoint must say so rather than return an
	// empty list that would falsely suggest the app has no beans.
	rec := doBeans(&Server{})

	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Contains(`"beans"`)
	assert.String(t, rec.Body.String()).Contains(`bean registry not available`)
}

// --- per-endpoint include/exclude -------------------------------------------

func registeredNames(s *Server) map[string]bool {
	// Recreate the registration decisions Run() would make (probes excluded:
	// they are always registered by design).
	names := map[string]bool{}
	for _, rt := range s.introspectionRoutes() {
		if s.endpointEnabled(context.Background(), rt.name) {
			names[rt.name] = true
		}
	}
	for _, ep := range s.Endpoints {
		name := strings.TrimPrefix(ep.Path(), "/")
		if s.endpointEnabled(context.Background(), name) {
			names[name] = true
		}
	}
	return names
}

// fakeEndpoint is a contributed endpoint (like otel's /metrics).
type fakeEndpoint struct{ path string }

func (f fakeEndpoint) Path() string { return f.path }

func (f fakeEndpoint) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestEndpointFilter_DefaultAllOn(t *testing.T) {
	s := &Server{Endpoints: []endpoint.Endpoint{fakeEndpoint{"/metrics"}}}
	names := registeredNames(s)
	for _, want := range []string{"info", "loggers", "env", "configprops", "threaddump", "beans", "metrics"} {
		assert.That(t, names[want]).True()
	}
}

func TestEndpointFilter_IncludeWhitelist(t *testing.T) {
	s := &Server{
		EndpointInclude: "env, metrics",
		Endpoints:       []endpoint.Endpoint{fakeEndpoint{"/metrics"}},
	}
	names := registeredNames(s)
	assert.That(t, names["env"]).True()
	assert.That(t, names["metrics"]).True()
	// Everything not in the whitelist is off — /env keeps its current
	// behavior, only the mechanism is new.
	for _, off := range []string{"info", "loggers", "configprops", "threaddump", "beans"} {
		assert.That(t, names[off]).False()
	}
}

func TestEndpointFilter_ExcludeAlwaysApplies(t *testing.T) {
	s := &Server{EndpointExclude: "env,threaddump"}
	names := registeredNames(s)
	assert.That(t, names["env"]).False()
	assert.That(t, names["threaddump"]).False()
	assert.That(t, names["configprops"]).True()
}

func TestEndpointFilter_ExcludeWinsOverInclude(t *testing.T) {
	// include selects the whitelist, exclude then removes from it.
	s := &Server{EndpointInclude: "env,info", EndpointExclude: "info"}
	names := registeredNames(s)
	assert.That(t, names["env"]).True()
	assert.That(t, names["info"]).False()
}

func TestEndpointFilter_CaseInsensitiveAndWhitespace(t *testing.T) {
	s := &Server{EndpointInclude: " Env , Info "}
	names := registeredNames(s)
	assert.That(t, names["env"]).True()
	assert.That(t, names["info"]).True()
}
