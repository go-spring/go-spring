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

package resilience

import (
	"context"
	"errors"
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

// resilienceTag is the static log tag for the resilience access log; the
// protected client's system is a log field, not part of the tag.
var resilienceTag = log.RegisterAppTag("resilience", "")

// WrapExecutor returns an [Executor] that wraps inner with an internal span
// (system/resource/outcome attributes), the resilience metrics, and an
// access log per call. Pass the system label (e.g. "redis", "gorm", "grpc")
// so calls from several protected clients are distinguishable. A nil inner
// returns nil — no wrapper, so an unarmed client stays untouched.
//
// Tracer and instruments are resolved from whatever OTel providers are
// current — here, at wiring time, not at package init — so an SDK installed
// after this package's init still receives the spans and records.
func WrapExecutor(inner Executor, system string) Executor {
	if inner == nil {
		return nil
	}
	meter := otel.Meter("go-spring.org/cloud/governance/resilience")
	duration, _ := meter.Float64Histogram("resilience.operation.duration",
		metric.WithDescription("Duration of resilience-protected calls"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	// calls counts protected calls with the outcome classified as one of:
	// success, rate_limited, circuit_open, bulkhead_full, timeout, error.
	calls, _ := meter.Int64Counter("resilience.calls",
		metric.WithDescription("Number of resilience-protected calls by outcome"),
		metric.WithUnit("{call}"))
	// breakerChanges counts circuit-breaker state transitions (from/to attrs).
	breakerChanges, _ := meter.Int64Counter("resilience.breaker.state_change",
		metric.WithDescription("Circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	w := &wrappedExecutor{
		inner:          inner,
		system:         system,
		tracer:         otel.Tracer("go-spring.org/cloud/governance/resilience"),
		duration:       duration,
		calls:          calls,
		breakerChanges: breakerChanges,
	}
	// If the inner executor emits breaker state transitions, subscribe so each
	// trip / half-open / recovery emits a counter + log automatically. Drivers
	// without that capability (no BreakerEventListenerSetter) are silently
	// skipped — the call-level signals still emit.
	if setter, ok := inner.(BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedExecutor struct {
	inner  Executor
	system string

	tracer         trace.Tracer
	duration       metric.Float64Histogram
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
}

// OnBreakerStateChange satisfies [BreakerEventListener]. It is invoked
// synchronously from inside the breaker's transition (so it must not call back
// into the executor); it emits a state-change counter and a log line.
func (w *wrappedExecutor) OnBreakerStateChange(resource string, from, to BreakerState) {
	w.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("resource", resource),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	fields := []log.Field{
		log.String("system", w.system),
		log.String("resource", resource),
		log.String("from", from.String()),
		log.String("to", to.String()),
	}
	// A trip (→open) is a service-level degradation worth flagging at Warn;
	// recovery and half-open trial are Info.
	if to == BreakerOpen {
		log.Warn(context.Background(), resilienceTag, fields...)
	} else {
		log.Info(context.Background(), resilienceTag, fields...)
	}
}

// Execute wraps the inner call in an internal span, records the duration
// histogram and the outcome-classified call counter, and writes the access
// log: a rejection or error at Warn, a success at Debug (protected calls are
// frequent; the success record is there for troubleshooting, not everyday
// reading).
func (w *wrappedExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	start := time.Now()
	ctx, span := w.tracer.Start(ctx, resource,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("resilience.system", w.system),
			attribute.String("resilience.resource", resource),
		))
	err := w.inner.Execute(ctx, resource, fn)
	outcome := classifyOutcome(err)
	span.SetAttributes(attribute.String("outcome", outcome))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	w.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("resource", resource),
		attribute.String("outcome", outcome),
	))
	w.calls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("resource", resource),
		attribute.String("outcome", outcome),
	))

	if err != nil {
		log.Warn(ctx, resilienceTag,
			log.String("system", w.system),
			log.String("resource", resource),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.String("outcome", outcome),
			log.Any("error", err))
		return err
	}
	log.Debug(ctx, resilienceTag, func() []log.Field {
		return []log.Field{
			log.String("system", w.system),
			log.String("resource", resource),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.String("outcome", outcome),
		}
	})
	return nil
}

func (w *wrappedExecutor) Close() error { return w.inner.Close() }

// Refresh forwards p to the inner executor, keeping the wrapper's
// metrics/listener intact. The inner driver always implements Refresh.
func (w *wrappedExecutor) Refresh(p Policy) error {
	return w.inner.Refresh(p)
}

// classifyOutcome maps an Executor's return error to a coarse outcome dimension.
// The resilience sentinels (rate-limited / circuit-open / bulkhead-full) are
// distinguished from caller timeouts and ordinary errors so a dashboard can
// separate "rejected by protection" from "downstream failed".
func classifyOutcome(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "error"
	}
}
