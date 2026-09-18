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

package StarterHTTPServer

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "go-spring.org/starter-http-server"

const meterName = "go-spring.org/starter-http-server"

// accessTag is the static log tag for this server's access log. The tag name
// uses an underscore because a tag is a syntax marker, not a path.
var accessTag = log.RegisterAppTag("http_server", "access")

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// semconv recommended set, the same one the other HTTP servers use.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// instruments holds this middleware's instruments. They are built once, when
// the middleware is constructed, so nothing is allocated per request.
type instruments struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

func newInstruments() *instruments {
	m := otel.GetMeterProvider().Meter(meterName)
	duration, _ := m.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of inbound HTTP requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	active, _ := m.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Inbound HTTP requests currently being served"),
		metric.WithUnit("{request}"),
	)
	return &instruments{duration: duration, active: active}
}

// Observe returns the server-side observability middleware: one span per
// request, the request duration histogram and in-flight gauge, and one
// access-log line.
//
// It is OPT-IN, unlike the gin/echo/hertz starters where instrumentation is
// built in: this package decorates whatever handler the application passed to
// the framework's own server, so it has no place to install itself. Compose it
// into the chain like any other decorator:
//
//	handler = StarterHTTPServer.Chain(Observe(), CORS(cfg), Authenticate(v, true))(handler)
//
// The names are the HTTP family's — http.server.request.duration,
// http.server.active_requests, http.request.method, http.response.status_code —
// so an application that switches between the stdlib server and gin keeps the
// same dashboards. The span rides the OTel globals starter-otel installs;
// without it they are no-ops and only the access log is emitted.
func Observe() Middleware {
	ins := newInstruments()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ins.active.Add(ctx, 1, metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
			))
			start := time.Now()

			ctx, span := otel.Tracer(tracerName).Start(ctx, r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.request.method", r.Method),
					attribute.String("url.path", r.URL.Path),
				),
			)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r.WithContext(ctx))

			dur := time.Since(start)
			ins.active.Add(ctx, -1, metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
			))
			ins.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.response.status_code", strconv.Itoa(sw.status)),
			))

			span.SetAttributes(
				attribute.Int("http.response.status_code", sw.status),
				attribute.String("status", statusOf(sw.status)),
			)
			if sw.status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(sw.status))
			}
			span.End()

			logAccess(ctx, r, sw.status, dur)
		})
	}
}

// logAccess writes the per-request access log. Its keys are the ones the metric
// carries (http.request.method, http.response.status_code), so selecting a
// failing method on a dashboard lands on the lines that explain it; url.path
// and duration_ms identify the line itself.
func logAccess(ctx context.Context, r *http.Request, code int, dur time.Duration) {
	fields := []log.Field{
		log.String("http.request.method", r.Method),
		log.String("url.path", r.URL.Path),
		log.Int("http.response.status_code", code),
		log.String("status", statusOf(code)),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if code >= 500 {
		log.Warn(ctx, accessTag, fields...)
		return
	}
	log.Info(ctx, accessTag, fields...)
}

// statusOf maps the response status onto the ecosystem-wide result axis: a 5xx
// is a server failure, anything else is not. The code itself stays on
// http.response.status_code — one axis, two granularities.
func statusOf(code int) string {
	if code >= 500 {
		return "error"
	}
	return "ok"
}
