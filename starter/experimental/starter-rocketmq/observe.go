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

// observe.go is the driver path's own instrumentation: a producer/consumer
// span, the messaging duration and in-flight metrics, and the access log for
// every published or consumed message. It rides the global OTel providers;
// without starter-otel every signal is a no-op.
package StarterRocketmq

import (
	"context"
	"go-spring.org/stdlib/strutil"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// accessTag is the static log tag for the RocketMQ access log.
var accessTag = log.RegisterAppTag("rocketmq", "access")

// tracer starts the producer/consumer spans of the driver path.
var tracer = otel.Tracer(tracerName)

// observer holds the driver path's OTel instruments. They are created by
// newObserver at construction time (NewPublisher / NewSubscriber), not at
// package init, so an SDK installed later still receives the records.
type observer struct {
	duration metric.Float64Histogram
	active   metric.Float64UpDownCounter
}

// newObserver builds the messaging.client.operation.duration histogram and
// the messaging.client.active_requests up-down counter from whatever meter
// provider is current.

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func newObserver() *observer {
	m := otel.Meter(tracerName)
	h, _ := m.Float64Histogram("messaging.client.operation.duration",
		metric.WithDescription("Duration of RocketMQ produce/consume operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	a, _ := m.Float64UpDownCounter("messaging.client.active_requests",
		metric.WithDescription("In-flight RocketMQ operations"),
		metric.WithUnit("{request}"))
	return &observer{duration: h, active: a}
}

// startProduce opens a producer observation for a publish to topic and
// injects the W3C trace context into msg's user properties.
func (o *observer) startProduce(ctx context.Context, topic string, msg *primitive.Message) (context.Context, *span) {
	ctx, sp := o.start(ctx, "publish", topic, trace.SpanKindProducer)
	otel.GetTextMapPropagator().Inject(ctx, msgCarrier{msg})
	return ctx, sp
}

// startConsume extracts the upstream trace from the message user properties
// and opens a consumer observation for the push handler.
func (o *observer) startConsume(ctx context.Context, ext *primitive.MessageExt) (context.Context, *span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, msgCarrier{&ext.Message})
	return o.start(ctx, "consume", ext.Topic, trace.SpanKindConsumer)
}

// start opens the operation's span and counts it as in-flight.
func (o *observer) start(ctx context.Context, op, dest string, kind trace.SpanKind) (context.Context, *span) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "rocketmq"),
		attribute.String("messaging.operation", op),
	}
	if dest != "" {
		attrs = append(attrs, attribute.String("messaging.destination.name", dest))
	}
	ctx, inner := tracer.Start(ctx, op, trace.WithSpanKind(kind), trace.WithAttributes(attrs...))
	o.active.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", "rocketmq"),
		attribute.String("messaging.operation", op),
	))
	return ctx, &span{ctx: ctx, observer: o, Span: inner, op: op, dest: dest, start: time.Now()}
}

// span is one in-flight observation opened by observer.start.
type span struct {
	ctx context.Context
	*observer
	trace.Span
	op    string
	dest  string
	start time.Time
}

// End closes the observation: it ends the span, records the duration metric,
// drops the in-flight count, and emits the access log entry. The log level
// carries the outcome: an error at Warn, a success with a destination at
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
	s.duration.Record(s.ctx, elapsed.Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", "rocketmq"),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	))
	s.active.Add(s.ctx, -1, metric.WithAttributes(
		attribute.String("messaging.system", "rocketmq"),
		attribute.String("messaging.operation", s.op),
	))

	fields := []log.Field{
		log.String("operation", s.op),
		log.Float("duration_ms", float64(elapsed.Nanoseconds())/1e6),
	}
	if s.dest != "" {
		fields = append(fields, log.String("destination", strutil.Truncate(s.dest, 512)))
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(fields, log.Any("error", err))...)
	case s.dest != "":
		log.Debug(s.ctx, accessTag, func() []log.Field { return fields })
	default:
		log.Info(s.ctx, accessTag, fields...)
	}
}
