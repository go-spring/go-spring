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

package lock

import (
	"context"
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
// lockTag is the static log tag for the lock access log; the backend's
// system is a log field, not part of the tag.
var lockTag = log.RegisterAppTag("lock", "")

// newDuration builds the lock.operation.duration histogram from whatever
// meter provider is current — created per Wrap, not at package init, so an
// SDK installed later than this package's init still receives the records.
func newDuration() metric.Float64Histogram {
	h, _ := otel.Meter("go-spring.org/cloud/lock").Float64Histogram("lock.operation.duration",
		metric.WithDescription("Duration of lock acquire operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	return h
}

// WrapLocker returns a [Locker] that wraps Acquire and TryAcquire with a
// client span, the lock.operation.duration metric, and an access log,
// labelled with the backend's system value. When starter-otel is not imported
// the global OTel providers are no-ops, so the wrapper adds negligible
// overhead and changes no behaviour.
//
// Tracer and histogram are resolved from whatever OTel providers are current
// — here, at wiring time, not at package init — so an SDK installed after
// this package's init still receives the spans and records.
//
// A lock starter installs it with its backend's system value:
//
//	locker = lock.WrapLocker("redis", inner)
func WrapLocker(system string, inner Locker) Locker {
	return &observedLocker{
		system:   system,
		inner:    inner,
		tracer:   otel.Tracer("go-spring.org/cloud/lock"),
		duration: newDuration(),
	}
}

type observedLocker struct {
	system   string
	inner    Locker
	tracer   trace.Tracer
	duration metric.Float64Histogram
}

// start opens the operation's client span.
func (l *observedLocker) start(ctx context.Context, op, key string) (context.Context, trace.Span) {
	return l.tracer.Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lock.system", l.system),
			attribute.String("lock.operation", op),
			attribute.String("lock.key", key),
		))
}

// record emits the duration metric and the access log for one finished
// operation. The log level carries the outcome: an error at Warn, a
// TryAcquire miss at Info, a plain success at Debug (lock acquire is frequent
// and uninteresting until it fails or misses).
func (l *observedLocker) record(ctx context.Context, op, key string, start time.Time, err error, acquired bool) {
	status := "ok"
	if err == nil && !acquired {
		status = "missed"
	}
	dur := float64(time.Since(start).Nanoseconds()) / 1e6
	l.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("system", l.system),
		attribute.String("operation", op),
		attribute.String("status", status),
	))

	common := func() []log.Field {
		return []log.Field{
			log.String("system", l.system),
			log.String("operation", op),
			log.String("key", key),
			log.Float("duration_ms", dur),
		}
	}
	switch {
	case err != nil:
		fields := append(common(), log.Any("error", err))
		log.Warn(ctx, lockTag, fields...)
	case !acquired:
		fields := append(common(), log.String("status", status))
		log.Info(ctx, lockTag, fields...)
	default:
		log.Debug(ctx, lockTag, common)
	}
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	start := time.Now()
	ctx, span := l.start(ctx, "acquire", key)
	held, err := l.inner.Acquire(ctx, key, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	l.record(ctx, "acquire", key, start, err, err == nil)
	return held, err
}

func (l *observedLocker) TryAcquire(ctx context.Context, key string, opts ...Option) (Lock, bool, error) {
	start := time.Now()
	ctx, span := l.start(ctx, "try_acquire", key)
	held, ok, err := l.inner.TryAcquire(ctx, key, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	l.record(ctx, "try_acquire", key, start, err, err == nil && ok)
	return held, ok, err
}

func (l *observedLocker) Close() error { return l.inner.Close() }
