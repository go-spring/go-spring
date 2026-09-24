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

// observe.go is the broker-neutral instrumentation for the messaging
// contract: a producer span around Publish, a consumer span around each
// Handler call, W3C trace-context propagation through the envelope's headers
// on both directions, the messaging.operation.* metrics, and an access log
// per message. It rides the OTel globals — without an installed provider the
// tracer and meter are no-ops, so the decorator adds negligible overhead.
package messaging

import (
	"context"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/strutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// accessTag is the static log tag for the messaging access log.
var accessTag = log.RegisterAppTag("messaging", "access")

// tracerName names the tracer the per-operation spans open on. The tracer is
// looked up per use (otel.Tracer at call time), never cached in a package
// variable: a package-level otel.Tracer captured before any provider is set
// stops forwarding once the global provider is set, unset and set again.
const tracerName = "go-spring.org/cloud/messaging"

// Operation names, shared by the span/metric/log keys so the three can never
// disagree.
const (
	opPublish = "publish"
	opConsume = "consume"
	statusOK  = "ok"
	statusErr = "error"
)

// maxLogArg bounds the destination captured in the access log.
const maxLogArg = 512

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// default boundaries assume request latencies; message handling routinely
// exceeds them, so the same explicit list the scheduling and refresh
// instruments use is reused here.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// Observe wraps d so every publisher and subscriber it opens is instrumented:
// publish/consume spans, the messaging.operation.* metrics, an access log per
// message, and W3C trace-context propagation through the envelope's headers —
// injected on publish, extracted on consume, so a trace links producer to
// consumer across services. system names the broker ("nats", "rocketmq", ...)
// and lands in the messaging.system span/metric attribute; the destination is
// the address the publisher/subscriber was bound to.
//
// A driver starter applies this once where it constructs its Driver; a Driver
// that already propagates and spans on its own must not be wrapped again, or
// every message would be counted and logged twice.
func Observe(d Driver, system string) Driver {
	m := otel.Meter("go-spring.org/cloud/messaging")
	total, _ := m.Int64Counter("messaging.operation.total",
		metric.WithDescription("Messages published and consumed, by operation and status"),
		metric.WithUnit("{message}"))
	duration, _ := m.Float64Histogram("messaging.operation.duration",
		metric.WithDescription("Duration of a publish or a consume"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	active, _ := m.Int64UpDownCounter("messaging.operation.active",
		metric.WithDescription("Number of in-flight messaging operations"),
		metric.WithUnit("{operation}"))
	o := &observer{system: system, total: total, duration: duration, active: active}
	return &observedDriver{Driver: d, o: o}
}

// observer holds the instruments shared by every operation of one wrapped
// Driver. They are built once in [Observe] and reused: message flows are too
// frequent for a per-message meter lookup. This relies on Observe running
// after the OTel global provider is installed — under the framework it does,
// since driver beans are constructed after RefreshPrepare, where starter-otel
// installs the provider. A driver built before any provider is set records to
// the no-op meter.
type observer struct {
	system   string
	total    metric.Int64Counter
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// observedDriver decorates the [Driver]; publishers and subscribers it opens
// are wrapped, so applications observe the same contract they call.
type observedDriver struct {
	Driver
	o *observer
}

func (d *observedDriver) NewPublisher(ctx context.Context, destination string) (Publisher, error) {
	p, err := d.Driver.NewPublisher(ctx, destination)
	if err != nil {
		return nil, err
	}
	return &observedPublisher{Publisher: p, o: d.o, destination: destination}, nil
}

func (d *observedDriver) NewSubscriber(ctx context.Context, source, group string) (Subscriber, error) {
	s, err := d.Driver.NewSubscriber(ctx, source, group)
	if err != nil {
		return nil, err
	}
	return &observedSubscriber{Subscriber: s, o: d.o, source: source}, nil
}

// observedPublisher spans and counts each Publish, and injects the current
// W3C trace context into the envelope's headers so the consumer's span
// continues the trace.
type observedPublisher struct {
	Publisher
	o           *observer
	destination string
}

func (p *observedPublisher) Publish(ctx context.Context, msg *Message) error {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", p.o.system),
		attribute.String("messaging.operation", opPublish),
		attribute.String("messaging.destination.name", p.destination),
	}
	ctx, sp := otel.Tracer(tracerName).Start(ctx, opPublish,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attrs...))
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier{msg: msg})
	p.o.begin(ctx, opPublish)

	// end runs on the defer path, so a panicking publish still closes its span
	// and brings the in-flight gauge back down. The panic itself is neither
	// swallowed nor recorded as an error — recovery is whoever called Publish.
	start := time.Now()
	var err error
	defer func() { p.o.end(ctx, sp, opPublish, p.destination, start, err) }()
	err = p.Publisher.Publish(ctx, msg)
	return err
}

// observedSubscriber wraps the handler the application subscribes with, so
// each delivered message is spanned, counted, and logged at the contract
// layer no matter which broker delivered it.
type observedSubscriber struct {
	Subscriber
	o      *observer
	source string
}

func (s *observedSubscriber) Subscribe(ctx context.Context, handler Handler) error {
	return s.Subscriber.Subscribe(ctx, func(ctx context.Context, msg *Message) error {
		ctx = otel.GetTextMapPropagator().Extract(ctx, headerCarrier{msg: msg})
		attrs := []attribute.KeyValue{
			attribute.String("messaging.system", s.o.system),
			attribute.String("messaging.operation", opConsume),
			attribute.String("messaging.destination.name", s.source),
		}
		ctx, sp := otel.Tracer(tracerName).Start(ctx, opConsume,
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attrs...))
		s.o.begin(ctx, opConsume)

		// end runs on the defer path, so a panicking handler still closes its
		// span and brings the in-flight gauge back down (the panic-guard
		// wrapper in recover.go is exactly the case this must survive). The
		// panic itself is neither swallowed nor recorded as an error —
		// recovery is the driver's or the panic-guard's.
		start := time.Now()
		var err error
		defer func() { s.o.end(ctx, sp, opConsume, s.source, start, err) }()
		err = handler(ctx, msg)
		return err
	})
}

// begin bumps the in-flight gauge for one starting operation; the matching
// end call brings it back down.
func (o *observer) begin(ctx context.Context, op string) {
	o.active.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", o.system),
		attribute.String("messaging.operation", op),
	))
}

// end records one operation's outcome: the exclusive status on
// messaging.operation.total, the elapsed time on messaging.operation.duration,
// the in-flight gauge back down, the span ended (with err recorded when
// non-nil), and the access log. The handler's error is returned unchanged by
// the callers — this is instrumentation, not error policy: how a non-nil
// error surfaces (nack, redelivery, log) stays the driver's contract.
//
// Log level follows the outcome: an error at Warn; a success at Debug (a
// healthy message flow is frequent and uninteresting until it fails).
func (o *observer) end(ctx context.Context, sp trace.Span, op, destination string, start time.Time, err error) {
	dur := time.Since(start)
	status := statusOK
	if err != nil {
		status = statusErr
	}
	attrs := metric.WithAttributes(
		attribute.String("messaging.system", o.system),
		attribute.String("messaging.operation", op),
		attribute.String("status", status),
	)
	o.total.Add(ctx, 1, attrs)
	o.duration.Record(ctx, dur.Seconds(), attrs)
	o.active.Add(ctx, -1, metric.WithAttributes(
		attribute.String("messaging.system", o.system),
		attribute.String("messaging.operation", op),
	))
	if err != nil {
		sp.RecordError(err)
		sp.SetStatus(codes.Error, err.Error())
	}
	sp.End()

	fields := []log.Field{
		log.String("messaging.operation", op),
		log.String("status", status),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if destination != "" {
		fields = append(fields, log.String("messaging.destination.name", strutil.Truncate(destination, maxLogArg)))
	}
	if err != nil {
		log.Warn(ctx, accessTag, append(fields, log.Err(err), log.Msg("messaging operation failed"))...)
		return
	}
	log.Debug(ctx, accessTag, func() []log.Field { return fields })
}

// headerCarrier adapts a [Message] to otel's [propagation.TextMapCarrier], so
// the W3C trace context rides the envelope's headers through any broker that
// maps them onto its native message properties.
type headerCarrier struct{ msg *Message }

var _ propagation.TextMapCarrier = headerCarrier{}

func (c headerCarrier) Get(key string) string { return c.msg.Header(key) }

func (c headerCarrier) Set(key, value string) { c.msg.SetHeader(key, value) }

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Headers))
	for k := range c.msg.Headers {
		keys = append(keys, k)
	}
	return keys
}
