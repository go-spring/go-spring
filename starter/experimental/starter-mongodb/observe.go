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

// observe.go is this starter's own mongodb instrumentation: a per-operation
// span, the db.client.* duration/in-flight metrics, and an access log riding
// the log package's native levels. It emits the same shape and vocabulary as
// the framework's unified emitter (cloud/governance/dataplane/outbound/resilience/observe.go) —
// bounded labels db.system + db.operation, unbounded detail (db.statement)
// reaching only the span and the log, an internal span named after the
// command, and the status/duration_ms access-log fields — but it emits
// LOCALLY, and it must.
//
// This is the one client starter that does not declare an
// chain.Operation for the resilience executor to apply: MongoDB's
// per-command observation has nowhere else to live. The mongo driver v2
// exposes no per-command execute hook (only SetDialer, SetMonitor and the
// server/pool monitors), so the resilience executor is reachable only at DIAL
// time — and connection pooling makes dials rare, which would make per-command
// signals vanish. Observation therefore rides the driver's CommandMonitor (a
// pure observer that cannot return an error), while protection rides the dial
// seam ([newDialer], see dial.go). See the package docs on
// [dbObserver] for the status vocabulary this forces.

package StarterMongoDB

import (
	"context"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-mongodb"

// mongodbSystem is the value the db.system label carries for this backend.
const mongodbSystem = "mongodb"

// maxArg bounds the operation argument captured on span attributes and the
// access log.
const maxArg = 512

// accessTag is the static log tag for the mongodb access log.
var accessTag = log.RegisterAppTag("mongodb", "access")

// Observer receives mongodb's METRIC events: the operation duration and the
// in-flight gauge. Spans are the STANDARD OTel capability and are opened by
// [dbObserver.Start], not by this seam. A nil Observer disables the metrics.
type Observer interface {
	// Operation records one finished operation. status is ok | error.
	Operation(ctx context.Context, operation, status string, duration time.Duration)

	// Active moves the in-flight gauge for operation by delta: +1 when it
	// starts, -1 when it finishes.
	Active(ctx context.Context, operation string, delta int64)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer].
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.duration, err = m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of "+mongodbSystem+" client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "mongodb: register db.client.operation.duration")
	}
	o.active, err = m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight "+mongodbSystem+" client operations"),
		metric.WithUnit("{request}"))
	if err != nil {
		return nil, errutil.Explain(err, "mongodb: register db.client.active_requests")
	}
	return o, nil
}

// otelObserver is the default [Observer]. The instruments are resolved when the
// client is constructed. It holds no per-instance state — the db.system /
// db.operation / status labels travel with each record, not here.
type otelObserver struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// Operation implements [Observer].
func (o *otelObserver) Operation(ctx context.Context, op, status string, duration time.Duration) {
	o.duration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("db.system", mongodbSystem),
		attribute.String("db.operation", op),
		attribute.String("status", status),
	))
}

// Active implements [Observer].
func (o *otelObserver) Active(ctx context.Context, op string, delta int64) {
	o.active.Add(ctx, delta, metric.WithAttributes(
		attribute.String("db.system", mongodbSystem),
		attribute.String("db.operation", op),
	))
}

// dbObserver emits the mongodb signals for one instance. Its records go through
// the injected [Observer]; only the log line stays here.
type dbObserver struct {
	system string
	obs    Observer
}

// newDBObserverWith is the injection point: obs may be nil, which disables
// observation.
func newDBObserverWith(obs Observer) *dbObserver {
	return &dbObserver{system: mongodbSystem, obs: obs}
}

// Start begins one operation: it bumps the in-flight gauge, opens the operation
// span, and records the start time; the returned span's End records the
// duration histogram, balances the gauge, ends the span, and emits the access
// log. op names the operation (span name, db.operation); arg is the optional
// operation argument (statement, URL path), bounded by maxArg.
//
// The span is SpanKindInternal — the framework-wide shape for a client call,
// matching the unified emitter — because from this process's point of view the
// call is an internal client operation, not a server-side handler.
func (o *dbObserver) Start(ctx context.Context, op, arg string) (context.Context, *dbSpan) {
	if o.obs != nil {
		o.obs.Active(ctx, op, 1)
	}
	attrs := []attribute.KeyValue{
		attribute.String("db.system", mongodbSystem),
		attribute.String("db.operation", op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("db.statement", strutil.Truncate(arg, maxArg)))
	}
	ctx, span := otel.Tracer(componentName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...))
	return ctx, &dbSpan{o: o, ctx: ctx, span: span, op: op, arg: arg, start: time.Now()}
}

// dbSpan is the handle returned by dbObserver.Start; End must be called
// exactly once.
type dbSpan struct {
	o     *dbObserver
	ctx   context.Context
	span  trace.Span
	op    string
	arg   string
	start time.Time
}

// End records the operation's outcome: the duration histogram, the in-flight
// gauge balance, the span (with err, if any), and the access log — an error
// at Warn, a success carrying an operation argument at Debug (built lazily),
// a plain success at Info — the same level rule the unified emitter applies.
//
// Status vocabulary: `ok` / `error`, the axis every family uses. The unified
// emitter additionally carries `resilience.error_type` when protection rejected a
// call, which it can see because its executor is where the rejection happens. A
// CommandMonitor sees only the driver's own error — it runs after the driver
// decided the outcome — so it has no outcome to report and correctly reports
// none, rather than an empty label.
func (s *dbSpan) End(err error) {
	o := s.o
	dur := time.Since(s.start)
	status := "ok"
	if err != nil {
		status = "error"
	}
	if o.obs != nil {
		o.obs.Operation(s.ctx, s.op, status, dur)
		o.obs.Active(s.ctx, s.op, -1)
	}
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
		s.span.RecordError(err)
	}
	s.span.End()

	common := func() []log.Field {
		fields := []log.Field{
			log.String("db.system", o.system),
			log.String("db.operation", s.op),
		}
		if s.arg != "" {
			fields = append(fields, log.String("db.statement", strutil.Truncate(s.arg, maxArg)))
		}
		return append(fields, log.String("status", status), log.Float("duration_ms", float64(dur.Nanoseconds())/1e6))
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, "mongo command failed", append(common(), log.Err(err))...)
	case s.arg != "":
		log.Debug(s.ctx, accessTag, "mongo command completed", func() []log.Field {
			return common()
		})
	default:
		log.Info(s.ctx, accessTag, "mongo command completed", common()...)
	}
}

// The package's default [Observer] bean: an application that contributes its
// own observer takes precedence.
func init() {
	gs.Provide(NewOTelObserver).Condition(gs.OnMissingBean[Observer]()).Caller(1)
}
