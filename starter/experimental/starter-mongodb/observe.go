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
// client span, the db.client.* duration/in-flight metrics, and an access
// log riding the log package's native levels. It is deliberately local —
// no shared observer framework — so the emitted vocabulary is all this
// package's own.
package StarterMongoDB

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// tracerName names the tracer the spans open on. The tracer is looked up
// per use (otel.Tracer at call time), never cached in a package variable: a
// package-level otel.Tracer captured before any provider is set stops
// forwarding once the global provider is set, unset and set again.
const tracerName = "go-spring.org/starter-mongodb"

// mongodbSystem is the value the db.system label carries for this backend.
const mongodbSystem = "mongodb"

// maxArg bounds the operation argument captured on span attributes and the
// access log.
const maxArg = 512

// accessTag is the static log tag for the mongodb access log.
var accessTag = log.RegisterAppTag("mongodb", "access")

// tracer opens this starter's client spans through the OTel global
// TracerProvider that starter-otel installs (a no-op when absent).

// dbObserver emits the mongodb client signals for one instance: the
// db.client.operation.duration histogram, the db.client.active_requests
// gauge, a client span per operation, and the access log. Its records go
// through the process-wide instrument set (see [instruments]); only the
// db.system label is per instance.
type dbObserver struct {
	system string
}

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever meter provider is current then,
// and immutable afterwards. It holds no per-instance state — the db.system /
// db.operation / status labels travel with each record, not here.
type instrumentSet struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

// buildInstruments builds the OTel instruments from whatever meter provider is
// current — resolved on first use, not at package init, so an SDK installed
// later than this package's init still receives the records.
func buildInstruments() *instrumentSet {
	m := otel.Meter("go-spring.org/starter-mongodb")
	duration, _ := m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of "+mongodbSystem+" client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	active, _ := m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight "+mongodbSystem+" client operations"),
		metric.WithUnit("{request}"))
	return &instrumentSet{duration: duration, active: active}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// newDBObserver builds the observer for one instance.
func newDBObserver() *dbObserver {
	return &dbObserver{system: mongodbSystem}
}

// Start begins one operation: it bumps the in-flight gauge, opens the client
// span, and records the start time; the returned span's End records the
// duration histogram, balances the gauge, ends the span, and emits the access
// log. op names the operation (span name, db.operation); arg is the optional
// operation argument (statement, URL path), bounded by maxArg.
func (o *dbObserver) Start(ctx context.Context, op, arg string) (context.Context, *dbSpan) {
	inflight := metric.WithAttributes(
		attribute.String("db.system", o.system),
		attribute.String("db.operation", op),
	)
	instruments().active.Add(ctx, 1, inflight)
	attrs := []attribute.KeyValue{
		attribute.String("db.system", o.system),
		attribute.String("db.operation", op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("db.statement", strutil.Truncate(arg, maxArg)))
	}
	ctx, span := otel.Tracer(tracerName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...))
	return ctx, &dbSpan{o: o, ctx: ctx, span: span, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// dbSpan is the handle returned by dbObserver.Start; End must be called
// exactly once.
type dbSpan struct {
	o        *dbObserver
	ctx      context.Context
	span     trace.Span
	op       string
	arg      string
	start    time.Time
	inflight metric.MeasurementOption
}

// End records the operation's outcome: the duration histogram, the in-flight
// gauge balance, the span (with err, if any), and the access log — an error
// at Warn, a success carrying an operation argument at Debug, a plain
// success at Info.
func (s *dbSpan) End(err error) {
	o := s.o
	dur := time.Since(s.start)
	status := "ok"
	if err != nil {
		status = "error"
	}
	instruments().duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("db.system", o.system),
		attribute.String("db.operation", s.op),
		attribute.String("status", status),
	))
	instruments().active.Add(s.ctx, -1, s.inflight)
	if s.span != nil {
		if err != nil {
			s.span.SetStatus(codes.Error, err.Error())
			s.span.RecordError(err)
		}
		s.span.End()
	}

	common := func() []log.Field {
		fields := []log.Field{
			log.String("db.operation", s.op),
			log.String("status", status),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
		}
		if s.arg != "" {
			fields = append(fields, log.String("db.statement", strutil.Truncate(s.arg, maxArg)))
		}
		return fields
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(common(), log.Err(err))...)
	case s.arg != "":
		log.Debug(s.ctx, accessTag, common)
	default:
		log.Info(s.ctx, accessTag, common()...)
	}
}
