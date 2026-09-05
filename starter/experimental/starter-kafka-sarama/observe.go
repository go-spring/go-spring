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

// observe.go is this starter's own kafka instrumentation: a messaging span, a
// duration/in-flight metric pair, and an access log per publish/consume
// operation, emitted on the OTel globals. Without starter-otel the global
// providers are no-ops, so the observation adds negligible overhead.
package StarterKafkaSarama

import (
	"context"
	"go-spring.org/stdlib/strutil"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentScope = "go-spring.org/starter-kafka-sarama"

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
var (
	// accessTag is the static log tag for the kafka access log.
	accessTag = log.RegisterAppTag("kafka", "access")

	// tracer opens the publish/consume spans.
	tracer = otel.Tracer(instrumentScope)
)

// observer emits the span + metrics + access log for one operation direction
// ("publish" or "consume").
type observer struct {
	op       string
	kind     trace.SpanKind
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// newObserver builds the direction's instruments from whatever meter provider
// is current — called lazily at first use, not at package init, so an SDK
// installed later than this package's init still receives the records.
func newObserver(op string, kind trace.SpanKind) *observer {
	m := otel.Meter(instrumentScope)
	duration, _ := m.Float64Histogram("messaging.client.operation.duration",
		metric.WithDescription("Duration of kafka client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	active, _ := m.Int64UpDownCounter("messaging.client.active_requests",
		metric.WithDescription("Number of in-flight kafka client operations"),
		metric.WithUnit("{request}"))
	return &observer{op: op, kind: kind, duration: duration, active: active}
}

// Span is the handle returned by observer.Start. End records the operation's
// outcome; it must be called exactly once.
type Span struct {
	o        *observer
	ctx      context.Context
	span     trace.Span
	arg      string
	start    time.Time
	inflight metric.MeasurementOption
}

// Start opens the operation's span, bumps the in-flight gauge, and opens the
// access record. arg is the destination topic, captured on the span and in the
// log when non-empty.
func (o *observer) Start(ctx context.Context, arg string) (context.Context, *Span) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.operation", o.op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("messaging.destination.name", strutil.Truncate(arg, 512)))
	}
	ctx, span := tracer.Start(ctx, o.op, trace.WithSpanKind(o.kind), trace.WithAttributes(attrs...))
	inflight := metric.WithAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.operation", o.op),
	)
	o.active.Add(ctx, 1, inflight)
	return ctx, &Span{o: o, ctx: ctx, span: span, arg: arg, start: time.Now(), inflight: inflight}
}

// End records the duration histogram, balances the in-flight gauge, ends the
// span (recording err if non-nil), and emits the access log. The log level
// carries the outcome: an error at Warn, a success with a destination at Debug
// (the per-message record is frequent and uninteresting until it fails), a
// success without a destination at Info.
func (s *Span) End(err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	dur := time.Since(s.start)
	s.o.duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.operation", s.o.op),
		attribute.String("status", status),
	))
	s.o.active.Add(s.ctx, -1, s.inflight)
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
		s.span.RecordError(err)
	}
	s.span.End()

	fields := func() []log.Field {
		f := []log.Field{
			log.String("operation", s.o.op),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
		}
		if s.arg != "" {
			f = append(f, log.String("destination", strutil.Truncate(s.arg, 512)))
		}
		return f
	}
	if err != nil {
		log.Warn(s.ctx, accessTag, append(fields(), log.Any("error", err))...)
		return
	}
	if s.arg != "" {
		log.Debug(s.ctx, accessTag, fields)
		return
	}
	log.Info(s.ctx, accessTag, fields()...)
}
