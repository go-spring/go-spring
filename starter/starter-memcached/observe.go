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

// observe.go is the module-local observer for the Client wrapper: a client
// span, the db.client.operation.duration metric and an access log per
// operation. gomemcache offers no hook/plugin point, so per-operation traffic
// can only be observed here, at the wrapper.
package StarterMemcached

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
const tracerName = "go-spring.org/starter-memcached"

// memcachedSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const memcachedSystem = "memcached"

var (
	// accessTag is the static log tag for the memcached access log.
	accessTag = log.RegisterAppTag("memcached", "access")
)

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever meter provider is current then,
// and immutable afterwards. It holds no per-client state — the db.system /
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
	m := otel.Meter("go-spring.org/starter-memcached")
	duration, _ := m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of memcached client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	active, _ := m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight memcached client operations"),
		metric.WithUnit("{request}"))
	return &instrumentSet{duration: duration, active: active}
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// observer emits the span/metric/access-log triple for one client instance.
// When starter-otel is not imported the global OTel providers are no-ops, so
// span+metric add negligible overhead; the access log always emits through the
// project log package. Its records go through the process-wide instrument set
// (see [instruments]); only the db.system label is per client.
type observer struct {
	system string
}

// newObserver builds the observer for one client.
func newObserver() *observer {
	return &observer{system: memcachedSystem}
}

// Start bumps the in-flight gauge, opens the operation's client span and
// returns it; the caller ends it with the operation's error via End.
func (o *observer) Start(ctx context.Context, op, arg string) obsSpan {
	inflight := metric.WithAttributes(
		attribute.String("db.system", o.system),
		attribute.String("db.operation", op),
	)
	instruments().active.Add(ctx, 1, inflight)
	ctx, sp := otel.Tracer(tracerName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", o.system),
			attribute.String("db.operation", op),
			attribute.String("db.statement", strutil.Truncate(arg, 512)),
		))
	return obsSpan{ctx: ctx, span: sp, op: op, arg: arg, start: time.Now(), o: o, inflight: inflight}
}

// obsSpan is one in-flight observed operation: End emits the span status, the
// duration metric, balances the in-flight gauge and writes the access log.
type obsSpan struct {
	ctx      context.Context
	span     trace.Span
	op       string
	arg      string
	start    time.Time
	o        *observer
	inflight metric.MeasurementOption
}

func (s obsSpan) End(err error) {
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()

	status := statusOf(err)
	dur := float64(time.Since(s.start).Nanoseconds()) / 1e6
	instruments().duration.Record(s.ctx, time.Since(s.start).Seconds(), metric.WithAttributes(
		attribute.String("db.system", s.o.system),
		attribute.String("db.operation", s.op),
		attribute.String("status", status),
	))
	instruments().active.Add(s.ctx, -1, s.inflight)

	// Log keys are the metric labels' names, so a dashboard selecting failed
	// operations lands on the lines that explain them.
	common := []log.Field{
		log.String("db.operation", s.op),
		log.String("status", status),
		log.Float("duration_ms", dur),
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(common, log.Err(err))...)
	case s.arg != "":
		// Success carrying a key/statement: high-frequency and uninteresting
		// until it fails, so Debug — and built lazily, truncation included.
		log.Debug(s.ctx, accessTag, func() []log.Field {
			return append(common, log.String("db.statement", strutil.Truncate(s.arg, 512)))
		})
	default:
		log.Info(s.ctx, accessTag, common...)
	}
}

// statusOf names the outcome the way the family's metric label and log field
// expect — the same two words the other DB backends use.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
