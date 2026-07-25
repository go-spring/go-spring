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
	"time"

	"go-spring.org/spring/cloud/tlsconf"
)

// HealthConfig exposes an optional liveness/readiness endpoint served by the
// starter. It is disabled by default so applications opt in explicitly.
type HealthConfig struct {
	Enabled bool   `value:"${enabled:=false}"`
	Path    string `value:"${path:=/healthz}"`
}

// Config defines Echo server configuration, bound from ${spring.echo.server}.
// Address must be explicitly configured; the server won't start without it.
type Config struct {
	Address      string            `value:"${addr}"`
	ReadTimeout  time.Duration     `value:"${readTimeout:=5s}"`
	WriteTimeout time.Duration     `value:"${writeTimeout:=5s}"`
	IdleTimeout  time.Duration     `value:"${idleTimeout:=60s}"`
	MaxBodySize  int64             `value:"${maxBodySize:=0}"`
	TLS          tlsconf.TLSConfig `value:"${tls}"`
	Health       HealthConfig      `value:"${health}"`
	Middleware   MiddlewareConfig  `value:"${middleware}"`
}

// MiddlewareConfig groups the built-in middlewares the starter can install on
// the *echo.Echo before the application's RouterRegister runs. Each block is
// independently toggleable so an application can opt out of any default.
//
// Only Recovery, RequestID and AccessLog are on by default - the three that are
// universally safe and expected of a production server. CORS, Gzip and
// SecureHeaders change request/response behavior or carry security trade-offs,
// so they stay off until an operator opts in. Unlike gin, echo ships all of
// these (and BodyLimit) in its official middleware package, so only AccessLog is
// self-implemented.
//
// Tracing and Metrics ride the OTel globals that starter-otel installs; they
// are off by default because they only produce output when starter-otel is also
// imported and configured.
type MiddlewareConfig struct {
	Recovery      RecoveryConfig      `value:"${recovery}"`
	RequestID     RequestIDConfig     `value:"${requestId}"`
	AccessLog     AccessLogConfig     `value:"${accessLog}"`
	Tracing       TracingConfig       `value:"${tracing}"`
	Metrics       MetricsConfig       `value:"${metrics}"`
	CORS          CORSConfig          `value:"${cors}"`
	Gzip          GzipConfig          `value:"${gzip}"`
	SecureHeaders SecureHeadersConfig `value:"${secureHeaders}"`
}

// RecoveryConfig toggles middleware.Recover. It is on by default: an unrecovered
// panic in a request goroutine would otherwise crash the whole process.
type RecoveryConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// RequestIDConfig toggles per-request id generation and propagation. It is on
// by default; the id is read from (or generated for) the X-Request-Id header
// and echoed on the response so callers and logs can correlate a single request
// end to end. The id is also stored on the request context (see
// RequestIDFromContext) so business logs can pick it up.
type RequestIDConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// AccessLogConfig toggles structured access logging through the project log
// package (not echo's own logger). It is on by default; the configured health
// endpoint path is auto-skipped so probes do not flood the log. Records are
// emitted at Warn for 4xx and Error for 5xx so failures stand out.
type AccessLogConfig struct {
	Enabled   bool     `value:"${enabled:=true}"`
	SkipPaths []string `value:"${skipPaths:=}"`
}

// CORSConfig enables middleware.CORS. It is off by default: cross-origin policy
// has no safe universal default, so an application must opt in and supply
// origins (or allow all for development).
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

// GzipConfig enables middleware.Gzip response compression. It is off by default.
// Level follows compress/gzip semantics (1=BestSpeed .. 9=BestCompression,
// -1=DefaultCompression).
type GzipConfig struct {
	Enabled bool `value:"${enabled:=false}"`
	Level   int  `value:"${level:=5}"`
}

// SecureHeadersConfig toggles middleware.Secure. It is off by default. The safe
// headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy) are always
// set when enabled; HSTS is emitted only when TLS is enabled and explicitly
// opted in, since sending Strict-Transport-Security over plain HTTP is a no-op
// that can mislead operators.
type SecureHeadersConfig struct {
	Enabled bool       `value:"${enabled:=false}"`
	HSTS    HSTSConfig `value:"${hsts}"`
}

// HSTSConfig controls the Strict-Transport-Security header, emitted only on
// HTTPS connections.
type HSTSConfig struct {
	Enabled           bool          `value:"${enabled:=false}"`
	MaxAge            time.Duration `value:"${maxAge:=0s}"`
	IncludeSubDomains bool          `value:"${includeSubDomains:=false}"`
	Preload           bool          `value:"${preload:=false}"`
}

// TracingConfig toggles the per-request tracing middleware that starts and ends
// an OTel server span on every request. It is off by default: tracing only
// produces output when starter-otel is also imported and configured with a
// non-none exporter.
type TracingConfig struct {
	Enabled bool `value:"${enabled:=false}"`
}

// MetricsConfig toggles the per-request HTTP metrics middleware that records
// request count, duration, and in-flight gauge through the global MeterProvider.
// It is off by default: metrics only produce output when starter-otel is also
// imported and configured with a non-none exporter.
type MetricsConfig struct {
	Enabled bool `value:"${enabled:=false}"`
}
