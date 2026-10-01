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

// observe.go is the module-local instrumentation for publishes and consumes:
// a producer/consumer span, the messaging.client.* metrics, and an access log
// per operation. It rides the OTel globals — without starter-otel the tracer
// and meter providers are no-ops, so the wrapper adds negligible overhead.
package StarterNats

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

// accessTag is the static log tag for the nats access log.
var accessTag = log.RegisterAppTag("nats", "access")

// tracerName names the tracer the per-operation spans open on. The tracer is
// looked up per use (otel.Tracer at call time), never cached in a package
// variable: a package-level otel.Tracer captured before any provider is set
// stops forwarding once the global provider is set, unset and set again.
const tracerName = "go-spring.org/starter-nats"

// maxLogArg bounds the subject captured in the access log.
const maxLogArg = 512

// Connection-state values the counter and the log lines share.
const (
	connDisconnected = "disconnected"
	connReconnected  = "reconnected"
	connClosed       = "closed"
)

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever providers are current then, and
// shared by every connection in the process.
type instrumentSet struct {
	duration    metric.Float64Histogram
	active      metric.Int64UpDownCounter
	connChanges metric.Int64Counter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before any bean is built, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(tracerName)
	in := &instrumentSet{}
	in.duration, _ = m.Float64Histogram("messaging.client.operation.duration",
		metric.WithDescription("Duration of nats client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	in.active, _ = m.Int64UpDownCounter("messaging.client.active_requests",
		metric.WithDescription("Number of in-flight nats client operations"),
		metric.WithUnit("{request}"))
	in.connChanges, _ = m.Int64Counter("messaging.client.connection.state_changes",
		metric.WithDescription("Connection-state transitions reported by the nats client"),
		metric.WithUnit("{event}"))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// connStateCounter counts the connection-state transitions the NATS client's own
// handlers report. Those events had only log lines: a connection that flapped or
// died was visible in text and invisible to every dashboard — and they are the
// signal that precedes operations starting to fail.
//
// The driver builds it, next to the handlers it counts for, because nats.Conn's
// Set*Handler methods REPLACE a slot rather than chaining: a counter installed
// from the starter's lifecycle would silently displace whatever handlers a
// custom Driver had set. Keeping the pair in one place is what lets both survive.
type connStateCounter struct {
	ins *instrumentSet
}

// newConnStateCounter takes the shared instrument set — invoked at wiring time
// (inside the driver), not at package init, so an SDK installed later still
// receives the records.
func newConnStateCounter() *connStateCounter {
	return &connStateCounter{ins: instruments()}
}

// record counts one transition and returns the fields its log line carries.
// Returning them is what keeps the metric's attribute and the log's key from
// being spelled twice — the drift this pairing exists to prevent, and a drifted
// key is silent: the line still looks right and joins nothing.
func (c *connStateCounter) record(ctx context.Context, state string) []log.Field {
	c.ins.connChanges.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", "nats"),
		attribute.String("state", state),
	))
	return []log.Field{
		log.String("messaging.system", "nats"),
		log.String("state", state),
	}
}

// observer emits the span + metric + access-log trio for one operation
// direction (publish or consume). kind selects the span kind.
type observer struct {
	kind trace.SpanKind
	ins  *instrumentSet
}

// newObserver takes the shared instrument set and the span kind of one
// direction — created at wiring time (newConn), not at package init, so an SDK
// installed later than this package's init still receives the records.
func newObserver(kind trace.SpanKind) *observer {
	return &observer{kind: kind, ins: instruments()}
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
// arg the subject (omitted from span attributes when empty). Start must be
// followed by exactly one span.End.
func (o *observer) Start(ctx context.Context, op, arg string) (context.Context, *span) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation", op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("messaging.destination.name", arg))
	}
	ctx, sp := otel.Tracer(tracerName).Start(ctx, op,
		trace.WithSpanKind(o.kind),
		trace.WithAttributes(attrs...))
	inflight := metric.WithAttributes(
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation", op),
	)
	o.ins.active.Add(ctx, 1, inflight)
	return ctx, &span{o: o, ctx: ctx, span: sp, op: op, arg: arg, start: time.Now(), inflight: inflight}
}

// End records the duration histogram, balances the in-flight gauge, ends the
// span (recording err if non-nil), and emits the access log. An error logs at
// Warn; a success with a subject at Debug (publishes are frequent and
// uninteresting until they fail); a success without a subject at Info.
func (s *span) End(err error) {
	o := s.o
	dur := time.Since(s.start)
	status := "ok"
	if err != nil {
		status = "error"
	}
	o.ins.duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation", s.op),
		attribute.String("status", status),
	))
	o.ins.active.Add(s.ctx, -1, s.inflight)
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
		log.Warn(s.ctx, accessTag, append(fields(), log.Err(err))...)
	case s.arg != "":
		log.Debug(s.ctx, accessTag, fields)
	default:
		log.Info(s.ctx, accessTag, fields()...)
	}
}
