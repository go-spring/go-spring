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

// observe.go is the module-local instrumentation for the span helpers in
// command.go: a producer/consumer span, the messaging.client.* metrics, and an
// access log per operation. It rides the OTel globals — without starter-otel
// the tracer and meter providers are no-ops, so the helpers add negligible
// overhead.
package StarterMQTT

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

// accessTag is the static log tag for the mqtt access log.
var accessTag = log.RegisterAppTag("mqtt", "access")

// tracer opens the per-operation producer/consumer spans.
var tracer = otel.Tracer("go-spring.org/starter-mqtt")

// maxLogArg bounds the topic captured in the access log.
const maxLogArg = 512

// Connection-state values the counter and the log lines share.
const (
	connConnected    = "connected"
	connLost         = "lost"
	connReconnecting = "reconnecting"
)

// connStateCounter counts the connection-lifecycle transitions paho reports.
// Those events had only log lines: a client that kept losing its broker was
// visible in text and invisible to every dashboard — and they are the signal
// that precedes publishes starting to fail.
//
// The driver builds it, next to the callbacks it counts for, because paho takes
// the handlers only as construction options: there is no post-construction slot
// to install them from the starter's side, so splitting the pair would mean
// dropping one of them.
type connStateCounter struct {
	changes metric.Int64Counter
}

// newConnStateCounter builds the counter from whatever meter provider is
// current — invoked at wiring time (inside the driver), not at package init, so
// an SDK installed later still receives the records.
func newConnStateCounter() *connStateCounter {
	changes, _ := otel.Meter("go-spring.org/starter-mqtt").Int64Counter("messaging.client.connection.state_changes",
		metric.WithDescription("Connection-state transitions reported by the mqtt client"),
		metric.WithUnit("{event}"))
	return &connStateCounter{changes: changes}
}

// record counts one transition and returns the fields its log line carries.
// Returning them is what keeps the metric's attribute and the log's key from
// being spelled twice — the drift this pairing exists to prevent, and a drifted
// key is silent: the line still looks right and joins nothing.
func (c *connStateCounter) record(ctx context.Context, state string) []log.Field {
	c.changes.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", "mqtt"),
		attribute.String("state", state),
	))
	return []log.Field{
		log.String("messaging.system", "mqtt"),
		log.String("state", state),
	}
}

// observer emits the span + metric + access-log trio for one operation
// direction (publish or consume). kind selects the span kind.
type observer struct {
	kind     trace.SpanKind
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// newObserver builds the observer's instruments from whatever meter provider
// is current — created at wiring time (newClient), not at package init, so an
// SDK installed later than this package's init still receives the records.

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func newObserver(kind trace.SpanKind) *observer {
	m := otel.Meter("go-spring.org/starter-mqtt")
	duration, _ := m.Float64Histogram("messaging.client.operation.duration",
		metric.WithDescription("Duration of mqtt client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	active, _ := m.Int64UpDownCounter("messaging.client.active_requests",
		metric.WithDescription("Number of in-flight mqtt client operations"),
		metric.WithUnit("{request}"))
	return &observer{kind: kind, duration: duration, active: active}
}

// span is the handle returned by observer.Start; End records the outcome.
type span struct {
	o        *observer
	ctx      context.Context
	span     trace.Span
	op       string
	arg      string
	start    time.Time
	inflight metric.MeasurementOption
}

// Start begins an operation: it opens the span, records the start time, and
// bumps the in-flight gauge. op is the operation name ("publish"/"consume"),
// arg the topic (omitted from span attributes when empty). Start must be
// followed by exactly one span.End.
func (o *observer) Start(ctx context.Context, op, arg string) (context.Context, *span) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "mqtt"),
		attribute.String("messaging.operation", op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("messaging.destination.name", arg))
	}
	ctx, sp := tracer.Start(ctx, op,
		trace.WithSpanKind(o.kind),
		trace.WithAttributes(attrs...))
	inflight := metric.WithAttributes(
		attribute.String("messaging.system", "mqtt"),
		attribute.String("messaging.operation", op),
	)
	o.active.Add(ctx, 1, inflight)
	return ctx, &span{o: o, ctx: ctx, span: sp, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// End records the duration histogram, balances the in-flight gauge, ends the
// span (recording err if non-nil), and emits the access log. An error logs at
// Warn; a success with a topic at Debug (publishes/consumes are frequent and
// uninteresting until they fail); a success without a topic at Info.
func (s *span) End(err error) {
	o := s.o
	dur := time.Since(s.start)
	status := "ok"
	if err != nil {
		status = "error"
	}
	o.duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", "mqtt"),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	))
	o.active.Add(s.ctx, -1, s.inflight)
	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()

	fields := func() []log.Field {
		f := []log.Field{
			log.String("messaging.operation", s.op),
			log.String("status", status),
			log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
		}
		if s.arg != "" {
			f = append(f, log.String("messaging.destination.name", strutil.Truncate(s.arg, maxLogArg)))
		}
		return f
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(fields(), log.Any("error", err))...)
	case s.arg != "":
		log.Debug(s.ctx, accessTag, fields)
	default:
		log.Info(s.ctx, accessTag, fields()...)
	}
}
