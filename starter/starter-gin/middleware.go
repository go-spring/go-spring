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
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-spring.org/stdlib/errutil"
)

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
//	RequestID -> Observe -> SecureHeaders -> CORS -> Gzip -> responseCapture
//
// RequestID is outermost so the request id is on the request context from the
// very start. Observe can then read it via RequestIDFromContext at any point -
// at entry, mid-request, or in its end-of-request finalize - rather than only in
// the defer. That decouples id availability from Observe's internal read
// timing, so future changes inside Observe (e.g. stamping the id onto the span
// at start) can't break it. (Running outside Observe means a panic in RequestID
// itself would escape recovery - but its body is trivial and cannot panic;
// handler panics are still recovered, since Observe's defer still catches them.)
//
// Observe bundles Recovery + Tracing + Metrics + AccessLog into one per-request
// lifecycle so a single deferred finalize owns every signal's end-of-request
// work - including on a handler panic, where the old separate-middlewares
// design leaked spans and in-flight gauges. These four are mandatory and always
// on whenever the built-in set is enabled (the default). The policy middlewares
// (SecureHeaders/CORS/Gzip) sit inside the chain so short-circuit responses
// (204, 403) are still observed. responseCapture is innermost (always installed;
// body capture is gated by the payload flag, but the SSE per-event hook is on
// regardless): it wraps the response writer inside gzip, so it records the
// UNCOMPRESSED bytes the handler writes - not the compressed wire bytes - and
// publishes them for Observe's access log. Splitting capture out of Observe
// keeps it inside gzip (Observe is outer, outside gzip); were capture still in
// Observe, gzip would sit inside it and resp.body would be compressed garbage.
func applyMiddlewares(e *gin.Engine, cfg Config) error {
	mw := cfg.Middleware

	// RequestID is outermost: it stamps the request id onto the request context
	// (and the response header) before anything else runs, so Observe and every
	// inner middleware can read it at any point via RequestIDFromContext.
	if mw.RequestID.Enabled {
		header := mw.RequestID.Header
		if header == "" {
			header = "X-Request-Id"
		}
		e.Use(requestID(header))
	}

	// Observe: recovers panics, starts the OTel server span, records HTTP
	// metrics, and emits the access log. Always on - no toggle. The health
	// endpoint path is folded into the access-log skip list so liveness/
	// readiness probes don't flood the log.
	accessCfg := mw.AccessLog
	if cfg.Health.Enabled && cfg.Health.Path != "" {
		accessCfg.SkipPaths = append(append([]string{}, accessCfg.SkipPaths...), cfg.Health.Path)
	}
	e.Use(observe(accessCfg))

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

	// responseCapture is innermost so it sees the uncompressed bytes the handler
	// writes - inside gzip (and any response transformer). It is always
	// installed: the SSE per-event hook (the http.server.sse.events counter,
	// plus real-time per-event logging when payload capture is on) runs
	// regardless of payload capture, so SSE observability stays on in production
	// where payload capture is typically off. Body capture for the access log's
	// resp.body is gated inside by the payload flag, so turning payload capture
	// off drops only the body-copy cost. Observe reads its capture via the gin
	// context.
	e.Use(responseCapture(
		mw.AccessLog.Payload.Enabled,
		mw.AccessLog.Payload.Limit,
		mw.AccessLog.Metrics.SSEDistributions,
	))
	return nil
}

// requestID installs the per-request id. It honors an incoming id on the
// configured header (so a caller or upstream proxy can supply one), generates a
// UUID v4 otherwise, echoes the id on the response header so callers and logs
// can correlate the request end to end, and stores it on the request context
// (requestIDCtxKey) so business code, the log package's FieldsFromContext hook,
// and the Observe middleware all read the same value via RequestIDFromContext.
//
// It is installed outermost (before Observe), so the id is on the request
// context from the very start and Observe can read it at any point - not only in
// its end-of-request finalize; future changes inside Observe can't break id
// availability. It wraps c.Request.Context() rather than a fresh context, so
// any value an outer layer attaches is preserved (and Observe in turn wraps
// this id-bearing context when it attaches its span, so the id survives into the
// span's context). The response header is set before c.Next so short-circuit
// responses (403) still carry the id.
func requestID(header string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(header)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Header(header, rid)
		ctx := context.WithValue(c.Request.Context(), requestIDCtxKey{}, rid)
		c.Request = c.Request.WithContext(ctx)
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
		if cfg.FrameOptions != "" {
			h.Set("X-Frame-Options", cfg.FrameOptions)
		}
		if cfg.ReferrerPolicy != "" {
			h.Set("Referrer-Policy", cfg.ReferrerPolicy)
		}

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
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete,
			http.MethodHead, http.MethodOptions,
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
