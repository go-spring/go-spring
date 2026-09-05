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

// observe.go is the module-local observer for the driver path: a
// producer/consumer span, the messaging.client.operation.duration metric and
// an access log per publish/consume. amqp091-go offers no per-message hook, so
// the driver observes here, around the publish/handler call.
package StarterRabbitMQ

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

// accessTag is the static log tag for the rabbitmq access log.
var accessTag = log.RegisterAppTag("rabbitmq", "access")

// tracer starts this module's messaging spans; instrument handles live in the
// observers so they bind to whatever OTel provider is current at construction.
var tracer = otel.Tracer(tracerName)

// observer emits the span/metric/access-log triple for one direction of the
// driver path. kind selects producer or consumer spans. When starter-otel is
// not imported the global OTel providers are no-ops, so span+metric add
// negligible overhead; the access log always emits through the project log
// package.
type observer struct {
	kind     trace.SpanKind
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// newObserver builds the messaging.client instruments from whatever meter
// provider is current — invoked at wiring time, not at package init, so an SDK
// installed later still receives the records.

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func newObserver(kind trace.SpanKind) *observer {
	m := otel.Meter(tracerName)
	duration, _ := m.Float64Histogram("messaging.client.operation.duration",
		metric.WithDescription("Duration of rabbitmq client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	active, _ := m.Int64UpDownCounter("messaging.client.active_requests",
		metric.WithDescription("Number of in-flight rabbitmq client operations"),
		metric.WithUnit("{request}"))
	return &observer{kind: kind, duration: duration, active: active}
}

// Start opens the operation's span, bumps the in-flight gauge, and returns the
// in-flight observation; the caller ends it with the operation's error via End.
func (o *observer) Start(ctx context.Context, op, arg string) (context.Context, obsSpan) {
	spanAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.operation", op),
	}
	if arg != "" {
		spanAttrs = append(spanAttrs, attribute.String("messaging.destination.name", strutil.Truncate(arg, 512)))
	}
	ctx, span := tracer.Start(ctx, op,
		trace.WithSpanKind(o.kind),
		trace.WithAttributes(spanAttrs...))
	inflight := metric.WithAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.operation", op),
	)
	o.active.Add(ctx, 1, inflight)
	return ctx, obsSpan{ctx: ctx, span: span, op: op, arg: arg, start: time.Now(), duration: o.duration, active: o.active, inflight: inflight}
}

// obsSpan is one in-flight observed operation: End emits the span status, the
// duration metric, balances the in-flight gauge, and writes the access log.
type obsSpan struct {
	ctx      context.Context
	span     trace.Span
	op       string
	arg      string
	start    time.Time
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
	inflight metric.MeasurementOption
}

func (s obsSpan) End(err error) {
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()

	status := "ok"
	if err != nil {
		status = "error"
	}
	dur := time.Since(s.start)
	s.duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	))
	s.active.Add(s.ctx, -1, s.inflight)

	common := []log.Field{
		log.String("operation", s.op),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	switch {
	case err != nil:
		fields := common
		if s.arg != "" {
			fields = append(fields, log.String("destination", strutil.Truncate(s.arg, 512)))
		}
		log.Warn(s.ctx, accessTag, append(fields, log.Any("error", err))...)
	case s.arg != "":
		// Success carrying an exchange/routing key: high-frequency and
		// uninteresting until it fails, so Debug — and built lazily, truncation
		// included.
		log.Debug(s.ctx, accessTag, func() []log.Field {
			return append(common, log.String("destination", strutil.Truncate(s.arg, 512)))
		})
	default:
		log.Info(s.ctx, accessTag, common...)
	}
}
