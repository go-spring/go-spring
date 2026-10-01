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

package StarterGateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-gateway"

// accessTag is the static log tag for the gateway access log.
var accessTag = log.RegisterAppTag("gateway", "access")

// instrumentSet holds the gateway's instruments. It is built once per process
// (see instruments), so nothing is allocated or initialized per request.
//
// These used to be hand-rolled atomic counters rendered as Prometheus text on a
// private /gateway/metrics endpoint. They now ride the OTel pipeline like every
// other starter's, which is the whole point: the gateway's numbers appear in the
// same scrape and the same OTLP stream as the rest of the application, instead
// of a private subset that no other dashboard reads. The exposition that used to
// be hand-written is served by starter-otel's Prometheus exporter — /metrics on
// the actuator management port, or on metrics.port when that is set.
type instrumentSet struct {
	requests     metric.Int64Counter
	active       metric.Int64UpDownCounter
	reloadErrors metric.Int64Counter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before the gateway serves, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(meterName)
	requests, _ := m.Int64Counter(
		"gateway.requests",
		metric.WithDescription("Requests proxied by the gateway"),
		metric.WithUnit("{request}"),
	)
	active, _ := m.Int64UpDownCounter(
		"gateway.active_requests",
		metric.WithDescription("Requests currently being proxied by the gateway"),
		metric.WithUnit("{request}"),
	)
	reloadErrors, _ := m.Int64Counter(
		"gateway.route_reload_errors",
		metric.WithDescription("Route table reloads that failed and kept the previous table"),
		metric.WithUnit("{event}"),
	)
	return &instrumentSet{requests: requests, active: active, reloadErrors: reloadErrors}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// observer is the container bean the route table reaches its reporting through.
// It carries no state of its own: every reading goes to the process-wide
// instrument set (see instruments), resolved at the use site.
type observer struct{}

func newObserver() *observer { return &observer{} }

// logAccess writes the per-request access log. Its identity keys are the ones
// the metric carries (gateway.route, status), so selecting a failing route on a
// dashboard lands on the lines that explain it; the request's own detail
// (method, path, status code, duration) is the line's payload, under the names
// the HTTP semantics use everywhere else.
func logAccess(ctx context.Context, route string, r *http.Request, code int, status string, dur time.Duration) {
	fields := []log.Field{
		log.String("gateway.route", route),
		log.String("status", status),
		log.String("http.request.method", r.Method),
		log.String("url.path", r.URL.Path),
		log.Int("http.response.status_code", code),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if code >= 500 {
		log.Warn(ctx, accessTag, fields...)
		return
	}
	log.Info(ctx, accessTag, fields...)
}

// statusOf maps the response status onto the ecosystem-wide result axis: a 5xx
// is a gateway failure, anything else is not. The code itself is what carries
// the detail, under http.response.status_code — one axis, two granularities,
// the same split the RPC family uses for status vs rpc.grpc.status_code.
func statusOf(code int) string {
	if code >= 500 {
		return "error"
	}
	return "ok"
}

// instrument wraps a route's handler to stamp the route id into the context
// (for rate-limit keys and downstream correlation) and to record the request:
// the in-flight gauge, the counter and one access-log line.
func (t *RouteTable) instrument(id string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := withRouteID(r.Context(), id)
		attrs := metric.WithAttributes(attribute.String("gateway.route", id))

		instruments().active.Add(ctx, 1, attrs)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(sw, r.WithContext(ctx))

		dur := time.Since(start)
		status := statusOf(sw.status)
		instruments().active.Add(ctx, -1, attrs)
		instruments().requests.Add(ctx, 1, metric.WithAttributes(
			attribute.String("gateway.route", id),
			attribute.String("status", status),
			attribute.Int("http.response.status_code", sw.status),
		))
		logAccess(ctx, id, r, sw.status, status, dur)
	})
}

// reloadError counts one failed route-table reload. The previous table stays in
// service, so this is the only signal that a config edit did not take effect.
func (o *observer) reloadError(ctx context.Context) { instruments().reloadErrors.Add(ctx, 1) }

// newGatewayHealth reports the gateway as a health.Indicator. It stays UP as
// long as the route table is loaded; a route whose lb:// upstream currently
// has zero live instances is a per-route concern surfaced via metrics/logs,
// not a reason to fail the whole gateway's readiness (it may still serve
// other routes).
func newGatewayHealth(tbl *RouteTable) *health.Indicator {
	return &health.Indicator{Name: "gateway", Probe: func(ctx context.Context) error {
		if tbl.compiled.Load() == nil {
			return fmt.Errorf("gateway: route table not loaded")
		}
		return nil
	}}
}
