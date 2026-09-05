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

// observe.go is the per-operation observability of this starter: a client
// span, the db.client.operation.duration metric, and an access log for each
// Get/Set/Delete, following the OTel db semantic conventions.
package StarterBigCache

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

// accessTag is the static log tag of the bigcache access log (renders as
// "_app_bigcache_access").
var accessTag = log.RegisterAppTag("bigcache", "access")

// obsTracer names the tracer/meter this starter's instruments register under.
const obsTracer = "go-spring.org/starter-bigcache"

// dbObserver emits the trace span, the duration metric, and the access log
// for one cache's per-operation traffic. bigcache is an in-process heap cache
// with no network, so the spans are root spans (no caller context to link) and
// the durations are sub-microsecond - the value is per-key access visibility
// and a uniform signal vocabulary with the other client starters.
//
// It rides the OTel globals: when starter-otel is not imported the global
// TracerProvider and MeterProvider are no-ops, so trace+metric add negligible
// overhead and the span's SpanContext stays invalid. The access log always
// emits through the project log package.
type dbObserver struct {
	duration metric.Float64Histogram
}

// newDBObserver builds the db.client.operation.duration histogram from
// whatever meter provider is current - created per client, not at package
// init, so an SDK installed later than this package's init still receives
// the records.

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func newDBObserver() *dbObserver {
	h, _ := otel.Meter(obsTracer).Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of cache operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	return &dbObserver{duration: h}
}

// start opens the operation's client span. arg is the cache key, carried as
// db.statement truncated to 512 bytes so a pathological key can't flood the
// span attributes.
func (o *dbObserver) start(ctx context.Context, op, arg string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "bigcache"),
		attribute.String("db.operation", op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("db.statement", strutil.Truncate(arg, 512)))
	}
	return otel.Tracer(obsTracer).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...))
}

// record ends the span and emits the duration metric and the access log for
// one finished operation. The log level carries the outcome: an error at
// Warn, a success with an argument at Debug (cache access is frequent and
// uninteresting until something fails), an argument-less success at Info;
// the Debug fields are built lazily since they are only needed when debug
// logging is on.
func (o *dbObserver) record(ctx context.Context, op, arg string, start time.Time, span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	dur := float64(time.Since(start).Nanoseconds()) / 1e6
	o.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("db.system", "bigcache"),
		attribute.String("db.operation", op),
		attribute.String("status", statusOf(err)),
	))

	fields := func() []log.Field {
		fs := []log.Field{
			log.String("db.operation", op),
			log.Float("duration_ms", dur),
		}
		if arg != "" {
			fs = append(fs, log.String("db.statement", strutil.Truncate(arg, 512)))
		}
		return fs
	}
	switch {
	case err != nil:
		log.Warn(ctx, accessTag, append(fields(), log.Any("error", err))...)
	case arg != "":
		log.Debug(ctx, accessTag, fields)
	default:
		log.Info(ctx, accessTag, fields()...)
	}
}

// statusOf maps the outcome to the coarse metric/log dimension.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
