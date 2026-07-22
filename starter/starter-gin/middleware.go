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
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// accessLogTag categorizes the structured access records emitted by the
// starter's AccessLog middleware (registered as the "_app_gin_access" tag).
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
func applyMiddlewares(e *gin.Engine, cfg Config) error {
	mw := cfg.Middleware

	if mw.Recovery.Enabled {
		e.Use(gin.Recovery())
	}
	if mw.RequestID.Enabled {
		header := mw.RequestID.Header
		if header == "" {
			header = "X-Request-Id"
		}
		e.Use(requestid.New(requestid.WithCustomHeaderStrKey(requestid.HeaderStrKey(header))))
		e.Use(propagateRequestID)
	}
	if mw.AccessLog.Enabled {
		e.Use(accessLog(accessLogSkipSet(cfg)))
	}
	if mw.SecureHeaders.Enabled {
		e.Use(secureHeaders(mw.SecureHeaders, cfg.TLS.Enabled))
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

// propagateRequestID copies the id set by gin-contrib/requestid onto the request
// context so downstream handlers and the project log package can read it.
func propagateRequestID(c *gin.Context) {
	if rid := requestid.Get(c); rid != "" {
		ctx := context.WithValue(c.Request.Context(), requestIDCtxKey{}, rid)
		c.Request = c.Request.WithContext(ctx)
	}
	c.Next()
}

// accessLog emits one structured record per request via the project log package.
// The level follows the response status: Warn for 4xx, Error for 5xx, Info
// otherwise, so failures stand out without filtering.
func accessLog(skip map[string]struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if _, ok := skip[path]; ok {
			return
		}

		fields := []log.Field{
			log.String("method", c.Request.Method),
			log.String("path", path),
			log.Int("status", c.Writer.Status()),
			log.Int("size", c.Writer.Size()),
			log.String("ip", c.ClientIP()),
			log.String("latency", time.Since(start).String()),
		}
		if rid := requestid.Get(c); rid != "" {
			fields = append(fields, log.String("request_id", rid))
		}

		ctx := c.Request.Context()
		switch status := c.Writer.Status(); {
		case status >= http.StatusInternalServerError:
			log.Error(ctx, accessLogTag, fields...)
		case status >= http.StatusBadRequest:
			log.Warn(ctx, accessLogTag, fields...)
		default:
			log.Info(ctx, accessLogTag, fields...)
		}
	}
}

// bodyLimit caps the request body size. It replaces the previous
// http.MaxBytesHandler wrapper that sat outside the gin chain and let an
// over-limit 413 bypass Recovery/AccessLog; in-chain, the 413 is logged and
// recovered like any other response.
func bodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// secureHeaders sets a small, safe set of response headers. HSTS is emitted
// only when TLS is enabled, the operator explicitly opts in, and a max-age is
// configured.
func secureHeaders(cfg SecureHeadersConfig, tlsEnabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		if cfg.HSTS.Enabled && tlsEnabled && cfg.HSTS.MaxAge > 0 {
			v := "max-age=" + strconv.FormatInt(int64(cfg.HSTS.MaxAge.Seconds()), 10)
			if cfg.HSTS.IncludeSubDomains {
				v += "; includeSubDomains"
			}
			if cfg.HSTS.Preload {
				v += "; preload"
			}
			h.Set("Strict-Transport-Security", v)
		}
		c.Next()
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
