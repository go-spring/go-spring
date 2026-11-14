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

package observe

import (
	"context"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/cloud/observability/instrument"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics receives the inbound request's METRIC events. It is the seam for the
// metric plane only: the request span and the access log are the STANDARD
// capabilities (OTel / the log package) and stay in [ServerObserver]. A nil
// Metrics disables the metrics.
type Metrics interface {
	// Active moves a declared route's in-flight gauge by delta. Only called for
	// a route that declared per-operation metrics.
	Active(ctx context.Context, r Request, delta int64)

	// Request records one finished request: the resilience.server.* duration
	// and counter, plus the declared route family's duration when it has one.
	Request(ctx context.Context, r Request)

	// Fault records the request's fault decision.
	Fault(ctx context.Context, r Request, f Fault)
}

// Request is one inbound request as the metric seam reports it.
type Request struct {
	// System and Service identify the route instance.
	System, Service string
	// Operation is the declared metric prefix; "" means the route declared no
	// per-operation metrics.
	Operation string
	// Attrs are the route's declared bounded labels; ResponseAttrs are what the
	// handler recorded while answering.
	Attrs, ResponseAttrs []attribute.KeyValue
	// Status is ok | error; ErrorType names a refusal's class.
	Status, ErrorType string
	Duration          time.Duration
}

// Fault is the request's fault decision, as the metric seam reports it.
type Fault struct {
	// Rule names the rule; Status is the response status it declared.
	Rule   string
	Status int
	// Inject is whether a response was injected; Delay is the added latency.
	Inject bool
	Delay  time.Duration
}

// NewOTelMetrics resolves the inbound instruments and returns the default
// OTel-backed [Metrics]. It is registered as the process's default bean.
func NewOTelMetrics() (Metrics, error) {
	ins, err := buildServerInstruments()
	if err != nil {
		return nil, err
	}
	return &otelMetrics{ins: ins}, nil
}

// otelMetrics is the default [Metrics].
type otelMetrics struct {
	ins *serverInstrumentSet
}

// Active implements [Metrics].
func (o *otelMetrics) Active(ctx context.Context, r Request, delta int64) {
	if r.Operation == "" {
		return
	}
	serverOperationInstruments(r.Operation).active.Add(ctx, delta,
		metric.WithAttributes(serverLabels(r.Attrs, "", nil)...))
}

// Request implements [Metrics]. The resilience.server.* labels keep their shape
// independent of the route family's metrics.
func (o *otelMetrics) Request(ctx context.Context, r Request) {
	labels := []attribute.KeyValue{
		attribute.String("system", r.System),
		attribute.String("service", r.Service),
		attribute.String("status", r.Status),
	}
	if r.ErrorType != "" {
		labels = append(labels, attribute.String("resilience.error_type", r.ErrorType))
	}
	opt := metric.WithAttributes(labels...)
	o.ins.duration.Record(ctx, r.Duration.Seconds(), opt)
	o.ins.calls.Add(ctx, 1, opt)

	if r.Operation == "" {
		return
	}
	// The declared family metrics: the route's own bounded vocabulary, the
	// response half, and the shared status axis. The family names are the HTTP
	// family's — `<prefix>.request.duration` / `<prefix>.active_requests` —
	// because a server's signal is a request, and those are the names every
	// dashboard already reads.
	serverOperationInstruments(r.Operation).duration.Record(ctx, r.Duration.Seconds(),
		metric.WithAttributes(serverLabels(r.Attrs, r.Status, r.ResponseAttrs)...))
}

// Fault implements [Metrics].
func (o *otelMetrics) Fault(ctx context.Context, r Request, f Fault) {
	base := []attribute.KeyValue{
		attribute.String("system", r.System),
		attribute.String("service", r.Service),
		attribute.String("rule", f.Rule),
	}
	if f.Inject {
		faultLabels := append(append([]attribute.KeyValue{}, base...),
			attribute.Int("status", f.Status))
		o.ins.faultInjected.Add(ctx, 1, metric.WithAttributes(faultLabels...))
	}
	if f.Delay > 0 {
		o.ins.faultDelayed.Add(ctx, 1, metric.WithAttributes(base...))
	}
}

// serverLabels builds an inbound family metric's labels: the declared bounded
// attrs, then whatever the handler recorded as it answered, then the shared
// status axis. Unlike the client side there is no service label — a server's
// identity is its route, and adding one would change the label set every
// existing HTTP dashboard groups by.
//
// A fresh slice, for the same reason the client side needs one: attrs comes off
// the context and is shared.
func serverLabels(attrs []attribute.KeyValue, status string, resp []attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs)+len(resp)+1)
	out = append(out, attrs...)
	out = append(out, resp...)
	if status == "" {
		return out
	}
	return append(out, attribute.String("status", status))
}

// serverInstrumentSet is the inbound REQUEST plane's always-on signals: the
// per-request duration and status counter. One set per Metrics, resolved when
// the metrics are constructed, and immutable afterwards. It holds no per-route
// state — the system and service labels travel with each request, not here.
//
// Governance organs' own state (rate budget, bulkhead) is deliberately absent:
// each organ publishes its own state from its own package, by pull.
// This plane answers only what happened to one request.
//
// It is a separate set from the outbound (resilience.client.*) family rather
// than one family with a direction attribute: the outcome sets genuinely differ
// (inbound has no retry, so no retry_exceeded and no per-attempt timeout),
// and keeping them apart leaves existing outbound dashboards untouched.
type serverInstrumentSet struct {
	duration      metric.Float64Histogram
	calls         metric.Int64Counter
	faultInjected metric.Int64Counter
	faultDelayed  metric.Int64Counter
}

// buildServerInstruments resolves the inbound request-plane instruments against
// the meter provider the otel starter installs before any bean is constructed.
// The metrics own the set, so a failed registration fails construction instead
// of silently dropping the whole metric family.
func buildServerInstruments() (*serverInstrumentSet, error) {
	m := otel.Meter(instrument.Scope)
	in := &serverInstrumentSet{}
	var err error
	in.duration, err = m.Float64Histogram("resilience.server.duration",
		metric.WithDescription("Duration of inbound requests under resilience governance"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "observe: register resilience.server.duration")
	}
	in.calls, err = m.Int64Counter("resilience.server.calls",
		metric.WithDescription("Number of inbound requests by inbound status"),
		metric.WithUnit("{request}"))
	if err != nil {
		return nil, errutil.Explain(err, "observe: register resilience.server.calls")
	}
	// Fault counters are the experiment's per-rule result signal. run_id stays on
	// spans/logs — it is high-cardinality and must not become a metric label.
	in.faultInjected, err = m.Int64Counter("resilience.server.fault.injected",
		metric.WithDescription("Requests whose error was injected by a fault rule"),
		metric.WithUnit("{request}"))
	if err != nil {
		return nil, errutil.Explain(err, "observe: register resilience.server.fault.injected")
	}
	in.faultDelayed, err = m.Int64Counter("resilience.server.fault.delayed",
		metric.WithDescription("Requests delayed by a fault rule"),
		metric.WithUnit("{request}"))
	if err != nil {
		return nil, errutil.Explain(err, "observe: register resilience.server.fault.delayed")
	}
	return in, nil
}
