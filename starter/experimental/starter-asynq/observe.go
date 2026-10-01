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

// observe.go is the enqueue path's own instrumentation: a producer span, the
// messaging duration and in-flight metrics, and the access log for every
// guarded enqueue. It rides the global OTel providers; without starter-otel
// every signal is a no-op.
package StarterAsynq

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
const tracerName = "go-spring.org/starter-asynq"

// accessTag is the static log tag for the Asynq access log.
var accessTag = log.RegisterAppTag("asynq", "access")

// tracer starts the producer spans of the enqueue path.

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever providers are current then.
type instrumentSet struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before any bean is built, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

// buildInstruments builds the messaging.client.operation.duration histogram and
// the messaging.client.active_requests up-down counter.
func buildInstruments() *instrumentSet {
	m := otel.Meter(tracerName)
	in := &instrumentSet{}
	in.duration, _ = m.Float64Histogram("messaging.client.operation.duration",
		metric.WithDescription("Duration of Asynq enqueue operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	in.active, _ = m.Int64UpDownCounter("messaging.client.active_requests",
		metric.WithDescription("In-flight Asynq operations"),
		metric.WithUnit("{request}"))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// observer holds the enqueue path's instrument set. newObserver takes the
// shared set at wiring time (Client.Init), not at package init, so an SDK
// installed later still receives the records.
type observer struct {
	ins *instrumentSet
}

// newObserver takes the shared instrument set.
func newObserver() *observer {
	return &observer{ins: instruments()}
}

// start opens a producer observation for one enqueue. taskType is the
// enqueued task's type name; it may be empty, in which case no destination
// attribute is recorded and the access log entry is logged at Info instead of
// Debug.
func (o *observer) start(ctx context.Context, op, taskType string) (context.Context, *span) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "asynq"),
		attribute.String("messaging.operation", op),
	}
	if taskType != "" {
		attrs = append(attrs, attribute.String("messaging.destination.name", taskType))
	}
	ctx, inner := otel.Tracer(tracerName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attrs...))
	o.ins.active.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", "asynq"),
		attribute.String("messaging.operation", op),
	))
	return ctx, &span{ctx: ctx, ins: o.ins, Span: inner, op: op, dest: taskType, start: time.Now()}
}

// span is one in-flight observation opened by observer.start.
type span struct {
	ctx context.Context
	ins *instrumentSet
	trace.Span
	op    string
	dest  string
	start time.Time
}

// End closes the observation: it ends the span, records the duration metric,
// drops the in-flight count, and emits the access log entry. The log level
// carries the outcome: an error at Warn, a success with a task type at
// Debug, a success without one at Info.
func (s *span) End(err error) {
	if err != nil {
		s.Span.RecordError(err)
		s.Span.SetStatus(codes.Error, err.Error())
	}
	s.Span.End()

	status := "ok"
	if err != nil {
		status = "error"
	}
	elapsed := time.Since(s.start)
	s.ins.duration.Record(s.ctx, elapsed.Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", "asynq"),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	))
	s.ins.active.Add(s.ctx, -1, metric.WithAttributes(
		attribute.String("messaging.system", "asynq"),
		attribute.String("messaging.operation", s.op),
	))

	fields := []log.Field{
		log.String("messaging.operation", s.op),
		log.String("status", status),
		log.Float("duration_ms", float64(elapsed.Nanoseconds())/1e6),
	}
	if s.dest != "" {
		fields = append(fields, log.String("messaging.destination.name", strutil.Truncate(s.dest, 512)))
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(fields, log.Err(err))...)
	case s.dest != "":
		log.Debug(s.ctx, accessTag, func() []log.Field { return fields })
	default:
		log.Info(s.ctx, accessTag, fields...)
	}
}
