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
	"time"

	"go-spring.org/spring/experimental/cloud/tlsconf"
)

// Config defines Gin server configuration, bound from ${spring.gin.server}.
// Address must be explicitly configured; the server won't start without it.
type Config struct {
	Address      string            `value:"${addr}"`
	ReadTimeout  time.Duration     `value:"${readTimeout:=5s}"`
	WriteTimeout time.Duration     `value:"${writeTimeout:=5s}"`
	IdleTimeout  time.Duration     `value:"${idleTimeout:=60s}"`
	TLS          tlsconf.TLSConfig `value:"${tls}"`
	Health       HealthConfig      `value:"${health}"`
	Middleware   MiddlewareConfig  `value:"${middleware}"`
}

// HealthConfig exposes an optional liveness/readiness endpoint served by the
// starter. It is disabled by default so applications opt in explicitly.
type HealthConfig struct {
	Enabled bool   `value:"${enabled:=false}"`
	Path    string `value:"${path:=/healthz}"`
}

// MiddlewareConfig groups the built-in middlewares the starter can install on
// the *gin.Engine before the application's RouterRegister runs.
//
// Enabled is the master switch (default true). When false the starter installs
// none of its built-in middlewares and the application's RouterRegister owns the
// whole chain - including Recovery, which is otherwise mandatory. Use it as the
// "full control" escape hatch; to swap a single piece, prefer the per-middleware
// toggles below rather than disabling the entire set.
//
// When Enabled is true, Observe bundles Recovery + Tracing + Metrics + AccessLog
// into one always-on middleware (see observe.go) so a single deferred finalize
// owns every signal's end-of-request work, including on a handler panic.
// RequestID is on by default; CORS, Gzip and SecureHeaders change
// request/response behavior or carry security trade-offs, so they stay off until
// an operator opts in.
//
// When Enabled is false the starter installs none of its built-in middlewares
// and the application's RouterRegister owns the whole chain - including
// Recovery. The exported ApplyMiddlewares and the individual constructors
// (RequestID, Observe, SecureHeaders, CORS, Gzip, ResponseCapture) let the
// application compose the standard set at a chosen point (e.g. running its own
// middleware before/after/between the built-ins) by injecting Config via the
// the same ${spring.gin.server} tag the starter uses. The EngineMiddleware hook
// (the easy path) is ignored in this mode, since the application owns the chain.
type MiddlewareConfig struct {
	Enabled       bool                `value:"${enabled:=true}"`
	RequestID     RequestIDConfig     `value:"${requestId}"`
	AccessLog     AccessLogConfig     `value:"${accessLog}"`
	CORS          CORSConfig          `value:"${cors}"`
	Gzip          GzipConfig          `value:"${gzip}"`
	SecureHeaders SecureHeadersConfig `value:"${secureHeaders}"`
}

// RequestIDConfig toggles per-request id generation and propagation. It is on
// by default; the id is read from (or generated for) the configured header and
// echoed on the response so callers and logs can correlate a single request
// end to end. The id is also stored on the request context so business code
// and the log package's FieldsFromContext hook can pick it up.
type RequestIDConfig struct {
	Enabled bool   `value:"${enabled:=true}"`
	Header  string `value:"${header:=X-Request-Id}"`
}

// AccessLogConfig holds options for the always-on access log emitted by the
// Observe middleware (one structured record per request via the project log
// package). Records are emitted at Warn for 4xx and Error for 5xx so failures
// stand out.
type AccessLogConfig struct {
	// SkipPaths suppresses every signal (span, metric, access log) for matching
	// requests, so chatty endpoints (health probes, high-frequency polls) don't
	// flood the backends. An entry matches the concrete request path (e.g.
	// /healthz) OR the matched gin route pattern (e.g. /users/:id), so a RESTful
	// route can be skipped without listing every concrete id. The configured
	// health endpoint path is auto-appended. A panic on a skipped path is still
	// recorded - skip never silences failures.
	SkipPaths []string      `value:"${skipPaths:=}"`
	Payload   PayloadConfig `value:"${payload}"`
	Metrics   MetricsConfig `value:"${metrics}"`
}

// PayloadConfig controls request/response payload capture in the access log for
// troubleshooting: the request body + query + headers and the response body.
// Redaction is handled by the log layer (key-based masking); here we only cap
// the captured size and skip binary/streaming content. On by default.
type PayloadConfig struct {
	Enabled bool `value:"${enabled:=true}"`
	// Limit caps how many bytes of the request body and of the response body are
	// captured per request (each, independently). With payload capture on, a
	// single access record is therefore bounded to ~2x Limit. Defaults to 512 KiB,
	// enough for typical JSON/form payloads without letting a runaway body exhaust
	// memory. Set higher only if you need to capture large bodies for debugging.
	Limit int `value:"${limit:=524288}"`
}

// MetricsConfig toggles optional metrics emitted by the Observe and
// responseCapture middlewares. The http.server.request.duration histogram is
// always on (disabling it would blind the server's basic latency view);
// middleware.enabled=false is the escape hatch for opting out entirely. The
// metrics below are individually toggleable.
type MetricsConfig struct {
	// SSEDistributions toggles the per-event http.server.sse.event.size and
	// http.server.sse.event.interval histograms. They are sampling-independent
	// sources for "p99 event size" and "stall/alert" dashboards that the sampled
	// per-event child span can't back. On by default; turn off when SSE event-size
	// or interval distributions aren't needed to spare the (small) per-event
	// record cost on high-event-rate streams.
	SSEDistributions bool `value:"${sseDistributions:=true}"`

	// ActiveRequests toggles the http.server.active_requests in-flight gauge.
	// Off by default: for short-lived HTTP requests the in-flight count is
	// derivable from QPS and latency (Little's law) and duplicates lower-level
	// runtime gauges (goroutine count), so the per-request +1/-1 bookkeeping is
	// not worth it. Turn it on for long-lived connections - notably SSE streams,
	// where each in-flight request holds a connection/goroutine for seconds to
	// minutes and the in-flight count is a capacity-planning essential that
	// latency/QPS alone can't surface.
	ActiveRequests bool `value:"${activeRequests:=false}"`
}

// CORSConfig enables gin-contrib/cors. It is off by default: cross-origin
// policy has no safe universal default, so an application must opt in and
// supply origins (or set allowAllOrigins for development).
type CORSConfig struct {
	Enabled          bool          `value:"${enabled:=false}"`
	AllowAllOrigins  bool          `value:"${allowAllOrigins:=false}"`
	AllowedOrigins   []string      `value:"${allowedOrigins:=}"`
	AllowedMethods   []string      `value:"${allowedMethods:=}"`
	AllowedHeaders   []string      `value:"${allowedHeaders:=}"`
	ExposeHeaders    []string      `value:"${exposeHeaders:=}"`
	AllowCredentials bool          `value:"${allowCredentials:=false}"`
	MaxAge           time.Duration `value:"${maxAge:=0s}"`
}

// GzipConfig enables gin-contrib/gzip response compression. It is off by
// default. Level follows compress/gzip semantics (1=BestSpeed ..
// 9=BestCompression, -1=DefaultCompression); MinLength bounds the response
// size below which compression is skipped (0 = compress everything).
type GzipConfig struct {
	Enabled   bool `value:"${enabled:=false}"`
	Level     int  `value:"${level:=5}"`
	MinLength int  `value:"${minLength:=0}"`
}

// SecureHeadersConfig toggles a small set of safe response headers. It is off
// by default. When enabled, X-Content-Type-Options:nosniff is always set (it
// has no downside); frameOptions (default DENY) and referrerPolicy (default
// no-referrer) are configurable - set either to "" to omit that header, e.g.
// frameOptions=SAMEORIGIN when the app needs same-origin iframing, or
// referrerPolicy=strict-origin-when-cross-origin to preserve referrer-based
// attribution. HSTS is emitted only when TLS is enabled and explicitly opted in,
// since sending Strict-Transport-Security over plain HTTP is a no-op that can
// mislead operators into thinking transport is pinned.
type SecureHeadersConfig struct {
	Enabled        bool       `value:"${enabled:=false}"`
	FrameOptions   string     `value:"${frameOptions:=DENY}"`
	ReferrerPolicy string     `value:"${referrerPolicy:=no-referrer}"`
	HSTS           HSTSConfig `value:"${hsts}"`
}

// HSTSConfig controls the Strict-Transport-Security header, emitted only on
// HTTPS connections.
type HSTSConfig struct {
	Enabled           bool          `value:"${enabled:=false}"`
	MaxAge            time.Duration `value:"${maxAge:=0s}"`
	IncludeSubDomains bool          `value:"${includeSubDomains:=false}"`
	Preload           bool          `value:"${preload:=false}"`
}
