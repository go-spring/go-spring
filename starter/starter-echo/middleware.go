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
	"context"
	"errors"
	"go-spring.org/cloud/governance"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
)

// accessLogTag categorizes the structured access records emitted by the
// starter's AccessLog middleware (registered as the "_app_echo_access" tag).
var accessLogTag = log.RegisterAppTag("echo", "access")

// requestIDCtxKey is the context key under which the RequestID middleware
// stores the request id on the request context, so business code and the log
// package's FieldsFromContext hook can correlate logs to a request.
type requestIDCtxKey struct{}

// LoadTest installs the inbound load-test traffic identification middleware.
// When the incoming request carries the configured marker header (default
// X-LoadTest) it tags the request context, so the handler chain and every
// outbound client the handlers drive can recognise synthetic load through the
// propagator's IsLoadTest. It is the inbound companion to
// cloud/traffic's outbound injection: together they let a load-test
// flag ride an HTTP hop end to end. An empty header falls back to the
// propagator's own header name; prop nil means go-spring's default. Without the
// marker it is a no-op.
func LoadTest(prop traffic.Propagator) echo.MiddlewareFunc {
	if prop == nil {
		// DefaultBinding is complete, so this cannot fail.
		prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := prop.Extract(c.Request().Context(), propagate.Header(c.Request().Header))
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// RequestIDFromContext returns the request id propagated by the starter's
// RequestID middleware, or "" when none is present. Pair it with the log
// package's FieldsFromContext hook to stamp the id onto every business log:
//
//	log.FieldsFromContext = func(ctx context.Context) []log.Field {
//	    if rid := StarterEcho.RequestIDFromContext(ctx); rid != "" {
//	        return []log.Field{log.String("request_id", rid)}
//	    }
//	    return nil
//	}
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// applyMiddlewares installs the enabled built-in middlewares onto the engine in
// a fixed, safe order, all before the application's RouterRegister runs:
//
//	Recovery -> RequestID -> Tracing -> Metrics -> AccessLog -> SecureHeaders -> CORS -> Gzip -> BodyLimit
//
// Recovery is outermost so it catches panics from every later layer; RequestID
// runs before AccessLog so each access record carries the request id; Tracing
// wraps Metrics and AccessLog so every span captures timing and attributes from
// both; AccessLog wraps the policy middlewares so short-circuit responses (413,
// 204, 403) are still logged. BodyLimit sits inside the chain so an over-limit
// 413 is logged and recovered like any other response.
func applyMiddlewares(e *echo.Echo, cfg Config, center *governance.Center, prop traffic.Propagator) error {
	mw := cfg.Middleware

	// LoadTest identification is outermost of all so the marker is on the
	// request context before Recovery, RequestID or any handler runs, letting
	// every downstream layer (and any outbound client the handler calls) branch
	// on the load-test convention's IsLoadTest. A single header lookup; off when
	// disabled.
	if mw.LoadTest.Enabled {
		e.Use(LoadTest(prop))
	}
	if mw.Recovery.Enabled {
		// Starter-owned recover (see recover.go): echo's middleware.Recover
		// semantics, plus reporting through the shared goutil panic chain so
		// the structured log bridge sees it.
		e.Use(Recover())
	}
	if mw.RequestID.Enabled {
		e.Use(middleware.RequestID())
		e.Use(propagateRequestID)
	}
	// The span, the HTTP family metrics and the access log are no longer
	// separate middlewares: they are emitted by the resilience executor this
	// server is admitted through, off the operation the inbound middleware
	// declares (see inbound.go). The per-signal switches remain readable — see
	// observeEnabled — but nothing is installed here for them.
	if mw.SecureHeaders.Enabled {
		e.Use(secureHeadersMiddleware(mw.SecureHeaders, cfg.TLS.Enabled))
	}
	if mw.CORS.Enabled {
		e.Use(corsMiddleware(mw.CORS))
	}
	if mw.Gzip.Enabled {
		e.Use(middleware.GzipWithConfig(middleware.GzipConfig{Level: mw.Gzip.Level}))
	}
	if cfg.MaxBodySize > 0 {
		e.Use(middleware.BodyLimit(strconv.FormatInt(cfg.MaxBodySize, 10)))
	}

	// Resilience inbound (always installed): runs the request through the
	// configured rate-limit / bulkhead / breaker before the handler chain. Sits
	// inside AccessLog so 429/503 rejects are still logged, and outside fault so
	// a rate-limited request is not also faulted. mgr is the governance
	// starter's manager bean (nil when governance is not imported); without it
	// the executor is a transparent pass-through, so installing it costs a call
	// frame and changes nothing else.
	e.Use(buildServerPolicy(cfg, center.Resilience()))

	// Fault injection (always installed, innermost so the resulting 503 is still
	// logged by the access log). inj is the governance starter's injector bean,
	// captured once here and reused for every request (nil-safe: a transparent
	// pass-through when governance is not imported); the center hot-swaps the
	// injector's config in place, so an operator can "set fire" to the running
	// server and hot-toggle it at runtime without a restart.
	e.Use(buildFault(center.Fault()))
	return nil
}

// buildFault builds the inbound fault-injection middleware. It gates the handler
// call with [fault.ApplyServer] so a configured fraction of inbound requests fail or
// slow down — the server-side counterpart to the client starters'
// fault.WrapClientExecutor. inj is the governance starter's injector bean, held for
// the life of the middleware: the center swaps its config in place
// ([fault.Injector.SetConfig]) rather than replacing the bean, so the captured
// reference always sees the live config. When it is nil, Apply is a transparent
// pass-through.
func buildFault(inj *fault.Injector) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := fault.ApplyServer(c.Request().Context(), inj, "echo", func() error {
				return next(c)
			})
			// Only an INJECTED fault (or a latency cancelled by the request
			// deadline) surfaces as 503 — the server is "unavailable" for this
			// request. A handler's own error (echo.ErrNotFound, an HTTPError
			// from the chain, ...) must pass through so Echo's
			// HTTPErrorHandler renders it, exactly as gin's buildFault lets
			// handler errors reach the engine.
			var inj *fault.InjectedError
			if err != nil && !c.Response().Committed && errors.As(err, &inj) {
				return c.String(http.StatusServiceUnavailable, "service unavailable")
			}
			return err
		}
	}
}

// propagateRequestID copies the id set by middleware.RequestID onto the request
// context so downstream handlers and the project log package can read it. The
// id is already on the response header by the time this runs.
func propagateRequestID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if rid := c.Response().Header().Get(echo.HeaderXRequestID); rid != "" {
			ctx := context.WithValue(c.Request().Context(), requestIDCtxKey{}, rid)
			c.SetRequest(c.Request().WithContext(ctx))
		}
		return next(c)
	}
}

// secureHeadersMiddleware builds a middleware.Secure handler. Empty SecureConfig
// fields are skipped by echo, so only the safe headers are set; HSTS is added
// only when TLS is enabled, the operator opts in, and a max-age is configured.
func secureHeadersMiddleware(cfg SecureHeadersConfig, tlsEnabled bool) echo.MiddlewareFunc {
	sc := middleware.SecureConfig{
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
		ReferrerPolicy:     "no-referrer",
	}
	if cfg.HSTS.Enabled && tlsEnabled && cfg.HSTS.MaxAge > 0 {
		sc.HSTSMaxAge = int(cfg.HSTS.MaxAge.Seconds())
		sc.HSTSExcludeSubdomains = !cfg.HSTS.IncludeSubDomains
		sc.HSTSPreloadEnabled = cfg.HSTS.Preload
	}
	return middleware.SecureWithConfig(sc)
}

// corsMiddleware builds a middleware.CORS handler from the starter config.
func corsMiddleware(cfg CORSConfig) echo.MiddlewareFunc {
	cc := middleware.CORSConfig{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		ExposeHeaders:    cfg.ExposeHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           int(cfg.MaxAge.Seconds()),
	}
	if cfg.AllowAllOrigins {
		cc.AllowOrigins = []string{"*"}
	}
	if len(cc.AllowMethods) == 0 {
		cc.AllowMethods = []string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodHead, http.MethodOptions,
		}
	}
	return middleware.CORSWithConfig(cc)
}
