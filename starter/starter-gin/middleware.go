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

package StarterGin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// accessLogTag categorizes the structured access records emitted by the
// Observe middleware (registered as the "_app_gin_access" tag).
var accessLogTag = log.RegisterAppTag("gin", "access")

// requestIDCtxKey is the context key under which the RequestID middleware
// stores the request id on the request context, so business code and the log
// package's FieldsFromContext hook can correlate logs to a request.
type requestIDCtxKey struct{}

// RequestIDFromContext returns the request id propagated by the starter's
// RequestID middleware, or "" when none is present. Pair it with the log
// package's FieldsFromContext hook to stamp the id onto every business log:
//
//	log.FieldsFromContext = func(ctx context.Context) []log.Field {
//	    if rid := StarterGin.RequestIDFromContext(ctx); rid != "" {
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

// applyMiddlewares installs the built-in middlewares onto the engine in a fixed,
// safe order, all before the application's RouterRegister runs:
//
//	Observe -> RequestID -> SecureHeaders -> CORS -> Gzip -> BodyLimit
//
// Observe (outermost) bundles Recovery + Tracing + Metrics + AccessLog into one
// per-request lifecycle so a single deferred finalize owns every signal's
// end-of-request work - including on a handler panic, where the old
// separate-middlewares design leaked spans and in-flight gauges. These four are
// mandatory and always on for the built-in gin server. RequestID runs inside
// Observe so each access record carries the request id and stays within the
// recovered span. The policy middlewares (SecureHeaders/CORS/Gzip/BodyLimit) sit
// inside the chain so short-circuit responses (413, 204, 403) are still
// observed; BodyLimit is innermost so an over-limit 413 is handled like any
// other response.
func applyMiddlewares(e *gin.Engine, cfg Config) error {
	mw := cfg.Middleware

	// Observe is outermost: it recovers panics, starts the OTel server span,
	// records HTTP metrics, and emits the access log. Always on - no toggle.
	// The health endpoint path is folded into the access-log skip list so
	// liveness/readiness probes don't flood the log.
	accessCfg := mw.AccessLog
	if cfg.Health.Enabled && cfg.Health.Path != "" {
		accessCfg.SkipPaths = append(append([]string{}, accessCfg.SkipPaths...), cfg.Health.Path)
	}
	e.Use(observe(accessCfg))

	if mw.RequestID.Enabled {
		header := mw.RequestID.Header
		if header == "" {
			header = "X-Request-Id"
		}
		e.Use(requestid.New(requestid.WithCustomHeaderStrKey(requestid.HeaderStrKey(header))))
		e.Use(propagateRequestID())
	}
	if mw.SecureHeaders.Enabled {
		e.Use(secureHeaders(mw.SecureHeaders))
	}
	if mw.CORS.Enabled {
		h, err := corsMiddleware(mw.CORS)
		if err != nil {
			return errutil.Explain(err, "gin: invalid cors config")
		}
		e.Use(h)
	}
	if mw.Gzip.Enabled {
		e.Use(gzipMiddleware(mw.Gzip))
	}
	if cfg.MaxBodySize > 0 {
		e.Use(bodyLimit(cfg.MaxBodySize))
	}
	return nil
}

// propagateRequestID copies the id set by gin-contrib/requestid onto the request
// context so downstream handlers and the project log package can read it.
func propagateRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rid := requestid.Get(c); rid != "" {
			ctx := context.WithValue(c.Request.Context(), requestIDCtxKey{}, rid)
			c.Request = c.Request.WithContext(ctx)
		}
	}
}

// bodyLimit caps the request body size. It sits inside the middleware chain so
// an over-limit 413 is observed - logged, recovered, and metered - like any
// other response, rather than short-circuiting around Observe.
func bodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
	}
}

// secureHeaders sets a small, safe set of response headers. HSTS is emitted
// only on TLS connections, when the operator explicitly opts in and a max-age
// is configured. Checking c.Request.TLS per request is equivalent to gating on
// the server's tls.enabled flag, since a single http.Server is either all-TLS
// or all-plain - so the flag need not be threaded in as a separate argument.
func secureHeaders(cfg SecureHeadersConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		if cfg.HSTS.Enabled && c.Request.TLS != nil && cfg.HSTS.MaxAge > 0 {
			v := "max-age=" + strconv.FormatInt(int64(cfg.HSTS.MaxAge.Seconds()), 10)
			if cfg.HSTS.IncludeSubDomains {
				v += "; includeSubDomains"
			}
			if cfg.HSTS.Preload {
				v += "; preload"
			}
			h.Set("Strict-Transport-Security", v)
		}
	}
}

// corsMiddleware builds a gin-contrib/cors handler from the starter config,
// validating up front so a misconfigured policy fails the server at startup
// with a clear error rather than panicking inside gin-contrib on the first
// request.
func corsMiddleware(cfg CORSConfig) (gin.HandlerFunc, error) {
	c := cors.Config{
		AllowAllOrigins:  cfg.AllowAllOrigins,
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		ExposeHeaders:    cfg.ExposeHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	}
	if len(c.AllowMethods) == 0 {
		c.AllowMethods = []string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
			http.MethodDelete, http.MethodHead, http.MethodOptions,
		}
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return cors.New(c), nil
}

// gzipMiddleware builds a gin-contrib/gzip handler from the starter config.
func gzipMiddleware(cfg GzipConfig) gin.HandlerFunc {
	var opts []gzip.Option
	if cfg.MinLength > 0 {
		opts = append(opts, gzip.WithMinLength(cfg.MinLength))
	}
	return gzip.Gzip(cfg.Level, opts...)
}
