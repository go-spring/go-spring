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
	"net/http"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-gateway"

// accessTag is the static log tag for the gateway access log.
var accessTag = log.RegisterAppTag("gateway", "access")

// Observer is the container bean the route table reaches its METRIC reporting
// through. It is the seam: the default is OTel, and an application may
// contribute its own adapter. Tracing is the STANDARD OTel capability (see
// proxySpan) and is not part of this contract.
type Observer interface {
	// Active moves the in-flight gauge for one route.
	Active(ctx context.Context, route string, delta int64)
	// Request records one finished request.
	Request(ctx context.Context, route string, code int)
	// ReloadError records one failed route-table reload.
	ReloadError(ctx context.Context)
}

// NewOTelObserver resolves the gateway's instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean, and
// builds the instruments once per process, so nothing is allocated or
// initialized per request. A failed registration fails startup instead of
// silently dropping the whole metric family.
//
// These metrics used to be hand-rolled atomic counters rendered as Prometheus
// text on a private /gateway/metrics endpoint. They now ride the OTel pipeline
// like every other starter's, which is the whole point: the gateway's numbers
// appear in the same scrape and the same OTLP stream as the rest of the
// application, instead of a private subset that no other dashboard reads. The
// exposition that used to be hand-written is served by starter-otel's
// Prometheus exporter — /metrics on the actuator management port, or on
// metrics.port when that is set.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.requests, err = m.Int64Counter(
		"gateway.requests",
		metric.WithDescription("Requests proxied by the gateway"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "gateway: register gateway.requests")
	}
	o.active, err = m.Int64UpDownCounter(
		"gateway.active_requests",
		metric.WithDescription("Requests currently being proxied by the gateway"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "gateway: register gateway.active_requests")
	}
	o.reloadErrors, err = m.Int64Counter(
		"gateway.route_reload_errors",
		metric.WithDescription("Route table reloads that failed and kept the previous table"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return nil, errutil.Explain(err, "gateway: register gateway.route_reload_errors")
	}
	return o, nil
}

// otelObserver is the default [Observer].
type otelObserver struct {
	requests     metric.Int64Counter
	active       metric.Int64UpDownCounter
	reloadErrors metric.Int64Counter
}

// Active implements [Observer].
func (o *otelObserver) Active(ctx context.Context, route string, delta int64) {
	o.active.Add(ctx, delta, metric.WithAttributes(attribute.String("gateway.route", route)))
}

// Request implements [Observer].
func (o *otelObserver) Request(ctx context.Context, route string, code int) {
	o.requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("gateway.route", route),
		attribute.String("status", statusOf(code)),
		attribute.Int("http.response.status_code", code),
	))
}

// ReloadError implements [Observer].
func (o *otelObserver) ReloadError(ctx context.Context) { o.reloadErrors.Add(ctx, 1) }

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
		log.Warn(ctx, accessTag, "gateway request failed", fields...)
		return
	}
	log.Info(ctx, accessTag, "gateway request completed", fields...)
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
		t.obs.Active(ctx, id, 1)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(sw, r.WithContext(ctx))

		dur := time.Since(start)
		status := statusOf(sw.status)
		t.obs.Active(ctx, id, -1)
		t.obs.Request(ctx, id, sw.status)
		logAccess(ctx, id, r, sw.status, status, dur)
	})
}

// reloadError counts one failed route-table reload. The previous table stays in
// service, so this is the only signal that a config edit did not take effect.

// newGatewayHealth reports the gateway as a health.Indicator. It stays UP as
// long as the route table is loaded; a route whose lb:// upstream currently
// has zero live instances is a per-route concern surfaced via metrics/logs,
// not a reason to fail the whole gateway's readiness (it may still serve
// other routes).
func newGatewayHealth(tbl *RouteTable) *health.Indicator {
	return &health.Indicator{Name: "gateway", Probe: func(ctx context.Context) error {
		if tbl.compiled.Load() == nil {
			return errutil.Explain(nil, "gateway: route table not loaded")
		}
		return nil
	}}
}
