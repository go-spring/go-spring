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

package StarterEcho

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// bindConfig binds a Config from flat dotted properties the way the container
// would bind ${spring.echo.server}, so tests exercise the same value-tag
// defaults and overrides production sees. "addr" is always supplied because the
// tag has no default — the server refuses to start without it by design.
func bindConfig(t *testing.T, props map[string]any) Config {
	t.Helper()
	props["addr"] = ":0"
	p := flatten.NewPropertiesStorage(flatten.MapProperties(props))
	var c Config
	assert.That(t, conf.Bind(p, &c)).Nil()
	return c
}

// newTestEngine builds a real echo engine with the starter's middleware chain
// applied for the given config, mirroring NewSimpleEchoServer minus the HTTP
// server wrapper. Behavior is asserted through e.ServeHTTP on httptest types.
func newTestEngine(t *testing.T, props map[string]any) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.HideBanner = true
	assert.That(t, applyMiddlewares(e, bindConfig(t, props))).Nil()
	return e
}

// TestConfig_BindDefaults pins the enabled/off defaults of every middleware
// block: only the universally safe trio (plus LoadTest identification and the
// OTel no-op pair) is on by default; CORS/Gzip/SecureHeaders stay opt-in.
func TestConfig_BindDefaults(t *testing.T) {
	c := bindConfig(t, map[string]any{})

	assert.That(t, c.Middleware.LoadTest.Enabled).True()
	assert.That(t, c.Middleware.LoadTest.Header).Equal("X-LoadTest")
	assert.That(t, c.Middleware.Recovery.Enabled).True()
	assert.That(t, c.Middleware.RequestID.Enabled).True()
	assert.That(t, c.Middleware.AccessLog.Enabled).True()
	assert.That(t, c.Middleware.Tracing.Enabled).True()
	assert.That(t, c.Middleware.Metrics.Enabled).True()

	assert.That(t, c.Middleware.CORS.Enabled).False()
	assert.That(t, c.Middleware.Gzip.Enabled).False()
	assert.That(t, c.Middleware.Gzip.Level).Equal(5)
	assert.That(t, c.Middleware.SecureHeaders.Enabled).False()

	assert.That(t, c.MaxBodySize).Equal(int64(0)) // 0 = BodyLimit not installed
	assert.That(t, c.Health.Enabled).False()
	assert.That(t, c.Health.Path).Equal("/healthz")
}

// TestConfig_BindOverrides verifies the enabled toggles and knobs actually
// reach the config struct through the dotted property path.
func TestConfig_BindOverrides(t *testing.T) {
	c := bindConfig(t, map[string]any{
		"middleware.recovery.enabled":    false,
		"middleware.cors.enabled":        true,
		"middleware.cors.allowedOrigins": []string{"https://a.example"},
		"middleware.gzip.enabled":        true,
		"middleware.gzip.level":          9,
		"maxBodySize":                    16,
		"health.enabled":                 true,
		"health.path":                    "/livez",
	})
	assert.That(t, c.Middleware.Recovery.Enabled).False()
	assert.That(t, c.Middleware.CORS.Enabled).True()
	assert.That(t, c.Middleware.CORS.AllowedOrigins).Equal([]string{"https://a.example"})
	assert.That(t, c.Middleware.Gzip.Enabled).True()
	assert.That(t, c.Middleware.Gzip.Level).Equal(9)
	assert.That(t, c.MaxBodySize).Equal(int64(16))
	assert.That(t, c.Health.Enabled).True()
	assert.That(t, c.Health.Path).Equal("/livez")
}

// TestConfig_AddrRequiredWithoutDefault asserts the "no default port" policy:
// binding without spring.echo.server.addr must fail so the server never
// silently starts on a guessed address.
func TestConfig_AddrRequiredWithoutDefault(t *testing.T) {
	p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{}))
	var c Config
	assert.Error(t, conf.Bind(p, &c)).Matches("addr")
}

// --- LoadTest ----------------------------------------------------------------

func TestLoadTestMiddleware_TagsContextFromHeader(t *testing.T) {
	var saw bool
	e := echo.New()
	e.Use(LoadTest(""))
	e.GET("/x", func(c echo.Context) error {
		saw = traffic.IsLoadTest(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})

	// With the marker header: the handler sees a load-test context.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(traffic.HeaderLoadTest, "1")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).True()

	// Without it: a plain context.
	saw = false
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, saw).False()
}

func TestLoadTestMiddleware_CustomHeaderAndTruthyValues(t *testing.T) {
	var saw bool
	e := echo.New()
	e.Use(LoadTest("X-Stress"))
	e.GET("/x", func(c echo.Context) error {
		saw = traffic.IsLoadTest(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})

	// Custom header: all truthy spellings match.
	for _, v := range []string{"1", "true", "ON", "Yes"} {
		saw = false
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Stress", v)
		e.ServeHTTP(httptest.NewRecorder(), req)
		assert.That(t, saw).True()
	}

	// Non-truthy value does not tag.
	saw = false
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Stress", "0")
	e.ServeHTTP(httptest.NewRecorder(), req)
	assert.That(t, saw).False()

	// The default header does NOT match when a custom one is configured.
	saw = false
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set(traffic.HeaderLoadTest, "1")
	e.ServeHTTP(httptest.NewRecorder(), req2)
	assert.That(t, saw).False()
}

// --- RequestID ----------------------------------------------------------------

// TestRequestIDMiddleware_GeneratesAndPropagates covers the full default-on
// chain: an id is generated, echoed on the response header, and — via the
// starter's propagateRequestID companion — visible to handlers through
// RequestIDFromContext for log correlation.
func TestRequestIDMiddleware_GeneratesAndPropagates(t *testing.T) {
	var ctxID string
	e := newTestEngine(t, map[string]any{})
	e.GET("/x", func(c echo.Context) error {
		ctxID = RequestIDFromContext(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w.Code).Equal(http.StatusOK)
	rid := w.Header().Get(echo.HeaderXRequestID)
	assert.That(t, rid != "").True()
	assert.That(t, ctxID).Equal(rid)
}

// TestRequestIDMiddleware_HonorsIncomingHeader pins the propagation half: a
// caller-supplied id is kept (not regenerated) so cross-service correlation
// survives.
func TestRequestIDMiddleware_HonorsIncomingHeader(t *testing.T) {
	e := newTestEngine(t, map[string]any{})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderXRequestID, "client-42")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.That(t, w.Header().Get(echo.HeaderXRequestID)).Equal("client-42")
}

// --- enabled switches ---------------------------------------------------------

// TestMiddlewareChain_TogglesOff asserts the opt-out path through the real
// assembly: with every toggle off (and no fault registered) the engine adds no
// RequestID header and no secure headers — i.e. the switches actually remove
// the middleware, not just flip a flag.
func TestMiddlewareChain_TogglesOff(t *testing.T) {
	e := newTestEngine(t, map[string]any{
		"middleware.loadtest.enabled":           false,
		"middleware.recovery.enabled":           false,
		"middleware.requestId.enabled":          false,
		"middleware.accessLog.enabled":          false,
		"middleware.tracing.enabled":            false,
		"middleware.metrics.enabled":            false,
		"middleware.secureHeaders.enabled":      true, // on: for the negative check below
		"middleware.secureHeaders.hsts.enabled": true,
		"middleware.secureHeaders.hsts.maxAge":  "1m",
	})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w.Header().Get(echo.HeaderXRequestID)).Equal("")

	// SecureHeaders on: safe headers set, but HSTS must never be emitted over
	// plain HTTP (httptest is http) even when opted in — sending
	// Strict-Transport-Security over HTTP is a no-op that misleads operators.
	assert.That(t, w.Header().Get("X-Content-Type-Options")).Equal("nosniff")
	assert.That(t, w.Header().Get("X-Frame-Options")).Equal("DENY")
	assert.That(t, w.Header().Get("Referrer-Policy")).Equal("no-referrer")
	assert.That(t, w.Header().Get("Strict-Transport-Security")).Equal("")
}

// TestMiddlewareChain_SecureHeadersOffByDefault asserts the default engine
// emits none of the secure headers until the operator opts in.
func TestMiddlewareChain_SecureHeadersOffByDefault(t *testing.T) {
	e := newTestEngine(t, map[string]any{})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w.Header().Get("X-Frame-Options")).Equal("")
	assert.That(t, w.Header().Get("X-Content-Type-Options")).Equal("")
}

// --- Recovery ------------------------------------------------------------------

// TestRecoverMiddleware_PanicBecomes500 pins the unified panic policy seam: a
// panicking handler is recovered (not crashing the test process), reported via
// the goutil chain, and answered with a 500 like echo's own Recover.
func TestRecoverMiddleware_PanicBecomes500(t *testing.T) {
	e := newTestEngine(t, map[string]any{})
	e.GET("/boom", func(c echo.Context) error {
		panic("kaboom")
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	assert.That(t, w.Code).Equal(http.StatusInternalServerError)
}

// --- CORS ----------------------------------------------------------------------

func TestCORSMiddleware_AllowAllOriginsWhenEnabled(t *testing.T) {
	e := newTestEngine(t, map[string]any{
		"middleware.cors.enabled":         true,
		"middleware.cors.allowAllOrigins": true,
	})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderOrigin, "https://evil.example")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.That(t, w.Header().Get(echo.HeaderAccessControlAllowOrigin)).Equal("*")

	// Preflight is answered by the CORS middleware without reaching the handler.
	pre := httptest.NewRequest(http.MethodOptions, "/x", nil)
	pre.Header.Set(echo.HeaderOrigin, "https://evil.example")
	pre.Header.Set(echo.HeaderContentType, "application/json")
	pw := httptest.NewRecorder()
	e.ServeHTTP(pw, pre)
	assert.That(t, pw.Code).Equal(http.StatusNoContent)
	assert.That(t, pw.Header().Get(echo.HeaderAccessControlAllowOrigin)).Equal("*")
}

func TestCORSMiddleware_OffByDefault(t *testing.T) {
	e := newTestEngine(t, map[string]any{})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderOrigin, "https://evil.example")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.That(t, w.Header().Get(echo.HeaderAccessControlAllowOrigin)).Equal("")
}

// --- Gzip ----------------------------------------------------------------------

func TestGzipMiddleware_CompressesWhenAccepted(t *testing.T) {
	e := newTestEngine(t, map[string]any{"middleware.gzip.enabled": true})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, strings.Repeat("hello world ", 100))
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderAcceptEncoding, "gzip")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.That(t, w.Code).Equal(http.StatusOK)
	assert.That(t, w.Header().Get(echo.HeaderContentEncoding)).Equal("gzip")

	// Without Accept-Encoding the body stays plain.
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w2.Header().Get(echo.HeaderContentEncoding)).Equal("")
	assert.String(t, w2.Body.String()).HasPrefix("hello world")
}

// --- BodyLimit -------------------------------------------------------------------

// TestBodyLimit_RejectsOversizedBodyWith413 drives the chain end to end: a body
// over maxBodySize is rejected with 413 by echo's BodyLimit while the outer
// layers (Recovery, AccessLog) still see a normal response.
func TestBodyLimit_RejectsOversizedBodyWith413(t *testing.T) {
	e := newTestEngine(t, map[string]any{"maxBodySize": 16})
	e.POST("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Oversized body: 413 without the handler running.
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("a", 32))))
	assert.That(t, w.Code).Equal(http.StatusRequestEntityTooLarge)

	// Within the limit: passes through.
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("short")))
	assert.That(t, w2.Code).Equal(http.StatusOK)
}

// --- fault injection --------------------------------------------------------------

// TestFaultMiddleware_Injects503ThenPassesThrough covers the always-installed
// fault middleware: with a registered injector at Rate 1 every request fails
// with 503; hot-toggling the injector off (no restart) restores pass-through.
// The injector is resolved lazily per request via fault.InjectorFor, which is
// why the middleware was built before the injector existed.
func TestFaultMiddleware_Injects503ThenPassesThrough(t *testing.T) {
	in := fault.NewInjector(fault.Config{Enabled: true, Rate: 1, Error: "generic"})
	fault.RegisterInjector(in)
	t.Cleanup(func() { fault.RegisterInjector(nil) })

	e := newTestEngine(t, map[string]any{})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w.Code).Equal(http.StatusServiceUnavailable)
	assert.That(t, w.Body.String()).Equal("service unavailable")

	// Hot-toggle: swap the live config, the next request passes through.
	in.SetConfig(fault.Config{})
	w2 := httptest.NewRecorder()
	e.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w2.Code).Equal(http.StatusOK)
}

// --- access log skip set --------------------------------------------------------

// TestAccessLogSkipSet_MergesHealthAndConfiguredPaths verifies probe traffic
// never floods the access log: the health path is auto-skipped (only when
// health is on) and merged with the operator's skip list.
func TestAccessLogSkipSet_MergesHealthAndConfiguredPaths(t *testing.T) {
	cfg := bindConfig(t, map[string]any{
		"health.enabled":                 true,
		"middleware.accessLog.skipPaths": []string{"/metrics", "/debug"},
	})
	skip := accessLogSkipSet(cfg)
	assert.That(t, len(skip)).Equal(3)
	for _, p := range []string{"/healthz", "/metrics", "/debug"} {
		_, ok := skip[p]
		assert.That(t, ok).True()
	}

	// Health off: its path is not auto-skipped.
	cfg2 := bindConfig(t, map[string]any{
		"middleware.accessLog.skipPaths": []string{"/metrics"},
	})
	skip2 := accessLogSkipSet(cfg2)
	assert.That(t, len(skip2)).Equal(1)
	_, ok := skip2["/healthz"]
	assert.That(t, ok).False()
}

// --- tracing/metrics no-op safety -------------------------------------------------

// TestMiddlewareChain_OtelNoopGlobalsStillServe asserts the "on by default,
// no-op without starter-otel" claim: with the default (no-op) OTel globals the
// tracing and metrics middlewares ride through without breaking the request.
func TestMiddlewareChain_OtelNoopGlobalsStillServe(t *testing.T) {
	e := newTestEngine(t, map[string]any{})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, w.Code).Equal(http.StatusOK)
}

// --- health endpoint ---------------------------------------------------------------

// TestNewSimpleEchoServer_HealthEndpointRegistered covers the constructor
// wiring: health on registers the endpoint before application routes, health
// off registers nothing. The engine is reached through the wrapper's http.Server
// handler so the test exercises the real assembly path.
func TestNewSimpleEchoServer_HealthEndpointRegistered(t *testing.T) {
	svr, err := NewSimpleEchoServer(func(e *echo.Echo) {}, bindConfig(t, map[string]any{
		"health.enabled": true,
		"health.path":    "/livez",
	}))
	assert.That(t, err).Nil()
	e := svr.svr.Handler.(*echo.Echo)

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/livez", nil))
	assert.That(t, w.Code).Equal(http.StatusOK)
	assert.That(t, w.Body.String()).Equal("ok")

	// Health disabled by default: the route is absent, and an unrouted request
	// gets echo's own 404 — buildFault passes handler errors through and only
	// rewrites INJECTED faults as 503 (mirroring gin's buildFault).
	svr2, err2 := NewSimpleEchoServer(func(e *echo.Echo) {}, bindConfig(t, map[string]any{}))
	assert.That(t, err2).Nil()
	e2 := svr2.svr.Handler.(*echo.Echo)
	w2 := httptest.NewRecorder()
	e2.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.That(t, w2.Code).Equal(http.StatusNotFound)
	assert.That(t, len(e2.Routes())).Equal(0)
}
