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
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go-spring.org/log"
)

// accessLogTag categorizes the structured access records emitted by the
// starter's AccessLog middleware (registered as the "_app_echo_access" tag).
var accessLogTag = log.RegisterAppTag("echo", "access")

// requestIDCtxKey is the context key under which the RequestID middleware
// stores the request id on the request context, so business code and the log
// package's FieldsFromContext hook can correlate logs to a request.
type requestIDCtxKey struct{}

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
//	Recovery -> RequestID -> AccessLog -> SecureHeaders -> CORS -> Gzip -> BodyLimit
//
// Recovery is outermost so it catches panics from every later layer; RequestID
// runs before AccessLog so each access record carries the request id; AccessLog
// wraps the policy middlewares so short-circuit responses (413, 204, 403) are
// still logged. BodyLimit sits inside the chain so an over-limit 413 is logged
// and recovered like any other response.
func applyMiddlewares(e *echo.Echo, cfg Config) error {
	mw := cfg.Middleware

	if mw.Recovery.Enabled {
		e.Use(middleware.Recover())
	}
	if mw.RequestID.Enabled {
		e.Use(middleware.RequestID())
		e.Use(propagateRequestID)
	}
	if mw.AccessLog.Enabled {
		e.Use(accessLog(accessLogSkipSet(cfg)))
	}
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
	return nil
}

// accessLogSkipSet builds the set of paths the access log should not record. It
// merges the operator-configured skip list with the health endpoint path, so
// liveness/readiness probes never flood the log.
func accessLogSkipSet(cfg Config) map[string]struct{} {
	skip := make(map[string]struct{}, len(cfg.Middleware.AccessLog.SkipPaths)+1)
	for _, p := range cfg.Middleware.AccessLog.SkipPaths {
		skip[p] = struct{}{}
	}
	if cfg.Health.Enabled && cfg.Health.Path != "" {
		skip[cfg.Health.Path] = struct{}{}
	}
	return skip
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

// accessLog emits one structured record per request via the project log package.
// The level follows the response status: Warn for 4xx, Error for 5xx, Info
// otherwise, so failures stand out without filtering.
func accessLog(skip map[string]struct{}) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			path := c.Request().URL.Path
			if _, ok := skip[path]; !ok {
				fields := []log.Field{
					log.String("method", c.Request().Method),
					log.String("path", path),
					log.Int("status", c.Response().Status),
					log.Int("size", c.Response().Size),
					log.String("ip", c.RealIP()),
					log.String("latency", time.Since(start).String()),
				}
				if rid := c.Response().Header().Get(echo.HeaderXRequestID); rid != "" {
					fields = append(fields, log.String("request_id", rid))
				}

				ctx := c.Request().Context()
				switch status := c.Response().Status; {
				case status >= http.StatusInternalServerError:
					log.Error(ctx, accessLogTag, fields...)
				case status >= http.StatusBadRequest:
					log.Warn(ctx, accessLogTag, fields...)
				default:
					log.Info(ctx, accessLogTag, fields...)
				}
			}
			return err
		}
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
