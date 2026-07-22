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

package StarterHertz

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/hertz-contrib/cors"
	"github.com/hertz-contrib/gzip"
	"github.com/hertz-contrib/requestid"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// accessLogTag categorizes the structured access records emitted by the
// starter's AccessLog middleware (registered as the "_app_hertz_access" tag).
var accessLogTag = log.RegisterAppTag("hertz", "access")

// requestIDCtxKey is the context key under which the RequestID middleware
// stores the request id on the request context, so business code and the log
// package's FieldsFromContext hook can correlate logs to a request.
type requestIDCtxKey struct{}

// RequestIDFromContext returns the request id propagated by the starter's
// RequestID middleware, or "" when none is present. Pair it with the log
// package's FieldsFromContext hook to stamp the id onto every business log:
//
//	log.FieldsFromContext = func(ctx context.Context) []log.Field {
//	    if rid := StarterHertz.RequestIDFromContext(ctx); rid != "" {
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
//	Recovery -> RequestID -> AccessLog -> SecureHeaders -> CORS -> Gzip
//
// Recovery is outermost so it catches panics from every later layer; RequestID
// runs before AccessLog so each access record carries the request id; AccessLog
// wraps the policy middlewares so short-circuit responses (204, 403) are still
// logged. Body limiting is handled by the engine option WithMaxRequestBodySize,
// not a middleware, so an over-limit 413 is likewise logged.
func applyMiddlewares(h *server.Hertz, cfg Config) error {
	mw := cfg.Middleware

	if mw.Recovery.Enabled {
		h.Use(recovery.Recovery())
	}
	if mw.RequestID.Enabled {
		h.Use(requestid.New())
		h.Use(propagateRequestID)
	}
	if mw.AccessLog.Enabled {
		h.Use(accessLog(accessLogSkipSet(cfg)))
	}
	if mw.SecureHeaders.Enabled {
		h.Use(secureHeaders(mw.SecureHeaders, cfg.TLS.Enabled))
	}
	if mw.CORS.Enabled {
		corsHandler, err := corsMiddleware(mw.CORS)
		if err != nil {
			return errutil.Explain(err, "hertz: invalid cors config")
		}
		h.Use(corsHandler)
	}
	if mw.Gzip.Enabled {
		h.Use(gzip.Gzip(mw.Gzip.Level))
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

// propagateRequestID copies the id set by hertz-contrib/requestid onto the
// request context so downstream handlers and the project log package can read
// it. The id is already on the response header by the time this runs.
func propagateRequestID(ctx context.Context, c *app.RequestContext) {
	if rid := requestid.Get(c); rid != "" {
		ctx = context.WithValue(ctx, requestIDCtxKey{}, rid)
	}
	c.Next(ctx)
}

// accessLog emits one structured record per request via the project log package.
// The level follows the response status: Warn for 4xx, Error for 5xx, Info
// otherwise, so failures stand out without filtering.
func accessLog(skip map[string]struct{}) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)

		path := string(c.URI().Path())
		if _, ok := skip[path]; ok {
			return
		}

		fields := []log.Field{
			log.String("method", string(c.Method())),
			log.String("path", path),
			log.Int("status", c.Response.StatusCode()),
			log.Int("size", len(c.Response.Body())),
			log.String("ip", c.ClientIP()),
			log.String("latency", time.Since(start).String()),
		}
		if rid := requestid.Get(c); rid != "" {
			fields = append(fields, log.String("request_id", rid))
		}

		switch status := c.Response.StatusCode(); {
		case status >= http.StatusInternalServerError:
			log.Error(ctx, accessLogTag, fields...)
		case status >= http.StatusBadRequest:
			log.Warn(ctx, accessLogTag, fields...)
		default:
			log.Info(ctx, accessLogTag, fields...)
		}
	}
}

// secureHeaders sets a small, safe set of response headers. It is self-implemented
// rather than using hertz-contrib/secure, whose defaults (a 10-year HSTS plus an
// SSL redirect) conflict with the starter's safe-by-default stance. HSTS is
// emitted only when TLS is enabled, the operator opts in, and a max-age is set.
func secureHeaders(cfg SecureHeadersConfig, tlsEnabled bool) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set("X-Content-Type-Options", "nosniff")
		c.Response.Header.Set("X-Frame-Options", "DENY")
		c.Response.Header.Set("Referrer-Policy", "no-referrer")

		if cfg.HSTS.Enabled && tlsEnabled && cfg.HSTS.MaxAge > 0 {
			v := "max-age=" + strconv.FormatInt(int64(cfg.HSTS.MaxAge.Seconds()), 10)
			if cfg.HSTS.IncludeSubDomains {
				v += "; includeSubDomains"
			}
			if cfg.HSTS.Preload {
				v += "; preload"
			}
			c.Response.Header.Set("Strict-Transport-Security", v)
		}
		c.Next(ctx)
	}
}

// corsMiddleware builds a hertz-contrib/cors handler from the starter config,
// validating up front so a misconfigured policy fails the server at startup
// with a clear error rather than panicking inside hertz-contrib.
func corsMiddleware(cfg CORSConfig) (app.HandlerFunc, error) {
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
