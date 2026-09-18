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
	"go-spring.org/stdlib/strutil"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// memcachedSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const memcachedSystem = "memcached"

var (
	// accessTag is the static log tag for the memcached access log.
	accessTag = log.RegisterAppTag("memcached", "access")

	tracer = otel.Tracer("go-spring.org/starter-memcached")
)

// newObserver builds the OTel instruments from whatever meter provider is
// current — created per observer (client construction), not at package init,
// so an SDK installed later still receives the records.
func newObserver() *observer {
	m := otel.Meter("go-spring.org/starter-memcached")
	duration, _ := m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of memcached client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	active, _ := m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight memcached client operations"),
		metric.WithUnit("{request}"))
	return &observer{system: memcachedSystem, duration: duration, active: active}
}

// observer emits the span/metric/access-log triple for one client instance.
// When starter-otel is not imported the global OTel providers are no-ops, so
// span+metric add negligible overhead; the access log always emits through the
// project log package.
type observer struct {
	system   string
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

// Start bumps the in-flight gauge, opens the operation's client span and
// returns it; the caller ends it with the operation's error via End.
func (o *observer) Start(ctx context.Context, op, arg string) obsSpan {
	inflight := metric.WithAttributes(
		attribute.String("db.system", o.system),
		attribute.String("db.operation", op),
	)
	o.active.Add(ctx, 1, inflight)
	ctx, sp := tracer.Start(ctx, op,
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
	s.o.duration.Record(s.ctx, time.Since(s.start).Seconds(), metric.WithAttributes(
		attribute.String("db.system", s.o.system),
		attribute.String("db.operation", s.op),
		attribute.String("status", status),
	))
	s.o.active.Add(s.ctx, -1, s.inflight)

	// Log keys are the metric labels' names, so a dashboard selecting failed
	// operations lands on the lines that explain them.
	common := []log.Field{
		log.String("db.operation", s.op),
		log.String("status", status),
		log.Float("duration_ms", dur),
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(common, log.Any("error", err))...)
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
