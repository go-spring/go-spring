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
var (
	// accessTag is the static log tag for the memcached access log.
	accessTag = log.RegisterAppTag("memcached", "access")

	tracer = otel.Tracer("go-spring.org/starter-memcached")
)

// newDuration builds the db.client.operation.duration histogram from whatever
// meter provider is current — created per observer (client construction), not
// at package init, so an SDK installed later still receives the records.
func newDuration() metric.Float64Histogram {
	h, _ := otel.Meter("go-spring.org/starter-memcached").Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of memcached client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	return h
}

// observer emits the span/metric/access-log triple for one client instance.
// When starter-otel is not imported the global OTel providers are no-ops, so
// span+metric add negligible overhead; the access log always emits through the
// project log package.
type observer struct {
	duration metric.Float64Histogram
}

func newObserver() *observer {
	return &observer{duration: newDuration()}
}

// Start opens the operation's client span and returns it; the caller ends it
// with the operation's error via End.
func (o *observer) Start(ctx context.Context, op, arg string) obsSpan {
	ctx, sp := tracer.Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "memcached"),
			attribute.String("db.operation", op),
			attribute.String("db.statement", strutil.Truncate(arg, 512)),
		))
	return obsSpan{ctx: ctx, span: sp, op: op, arg: arg, start: time.Now(), duration: o.duration}
}

// obsSpan is one in-flight observed operation: end emits the span status, the
// duration metric and the access log.
type obsSpan struct {
	ctx      context.Context
	span     trace.Span
	op       string
	arg      string
	start    time.Time
	duration metric.Float64Histogram
}

func (s obsSpan) End(err error) {
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()

	dur := float64(time.Since(s.start).Nanoseconds()) / 1e6
	s.duration.Record(s.ctx, time.Since(s.start).Seconds(), metric.WithAttributes(
		attribute.String("db.system", "memcached"),
		attribute.String("db.operation", s.op),
	))

	common := []log.Field{
		log.String("operation", s.op),
		log.Float("duration_ms", dur),
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(common, log.Any("error", err))...)
	case s.arg != "":
		// Success carrying a key/statement: high-frequency and uninteresting
		// until it fails, so Debug — and built lazily, truncation included.
		log.Debug(s.ctx, accessTag, func() []log.Field {
			return append(common, log.String("statement", strutil.Truncate(s.arg, 512)))
		})
	default:
		log.Info(s.ctx, accessTag, common...)
	}
}
