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
	"go-spring.org/cloud/actuator/health"
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
		name := strings.TrimPrefix(ep.Path, "/")
		if s.endpointEnabled(context.Background(), name) {
			names[name] = true
		}
	}
	return names
}


func TestEndpointFilter_SensitiveOffByDefault(t *testing.T) {
	// Introspection endpoints that expose configuration or internals
	// (env, configprops, threaddump, loggers, beans) must NOT register unless
	// explicitly included — an empty include is no longer a free pass.
	s := &Server{Endpoints: []*endpoint.Endpoint{&endpoint.Endpoint{Path: "/metrics"}}}
	names := registeredNames(s)
	for _, off := range []string{"loggers", "env", "configprops", "threaddump", "beans"} {
		assert.That(t, names[off]).False()
	}
	// /info (build metadata) and contributed endpoints stay default-on.
	assert.That(t, names["info"]).True()
	assert.That(t, names["metrics"]).True()
}

func TestEndpointFilter_SensitiveExplicitInclude(t *testing.T) {
	// Listing a sensitive endpoint in include turns it on.
	s := &Server{EndpointInclude: "env"}
	names := registeredNames(s)
	assert.That(t, names["env"]).True()
	assert.That(t, names["configprops"]).False()
}

func TestEndpointFilter_ExcludeBeatsSensitiveInclude(t *testing.T) {
	// exclude always wins, even over an explicit include of a sensitive endpoint.
	s := &Server{EndpointInclude: "env", EndpointExclude: "env"}
	names := registeredNames(s)
	assert.That(t, names["env"]).False()
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

// --- secret masking ----------------------------------------------------------

func TestMaskValue(t *testing.T) {
	cases := []struct {
		key, in, want string
	}{
		// Key-name rules.
		{"demo.datasource.password", "any", "******"},
		{"auth.access-key", "any", "******"},
		{"demo.api.token", "any", "******"},
		// Bare final-segment key variants now caught.
		{"some.key", "any", "******"},
		{"aws.key", "any", "******"},
		{"db.api_key", "any", "******"},
		{"key", "any", "******"},
		{"KEY", "any", "******"},
		// Word-boundary rule: look-alikes are NOT masked.
		{"demo.monkey", "tail", "tail"},
		{"demo.keyword", "search", "search"},
		{"demo.keynote", "talk", "talk"},
		// ENC(...) placeholder.
		{"demo.encrypted.value", "ENC(9f8a7b6c5d)", "******"},
		// URL-embedded credentials: userinfo redacted, rest kept.
		{"demo.db.url", "mysql://user:pass@host:3306/db", "mysql://******@host:3306/db"},
		{"demo.redis.addr", "redis://:p%40ss@redis:6379/0", "redis://******@redis:6379/0"},
		// Ordinary values untouched, including URLs without credentials.
		{"demo.datasource.url", "jdbc:mysql://localhost:3306/demo", "jdbc:mysql://localhost:3306/demo"},
	}
	for _, c := range cases {
		got := maskValue(c.key, c.in)
		assert.String(t, got).Equal(c.want)
	}
}

func TestEndpointFilter_IncludeWhitelist(t *testing.T) {
	s := &Server{
		EndpointInclude: "env, metrics",
		Endpoints:       []*endpoint.Endpoint{&endpoint.Endpoint{Path: "/metrics"}},
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
	// configprops is sensitive: default-off regardless, and exclude would win anyway.
	assert.That(t, names["configprops"]).False()
	// A non-sensitive endpoint stays on when not excluded.
	assert.That(t, names["info"]).True()
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
