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

// WrapClientExecutor returns an [ClientExecutor] that wraps inner with an internal span
// (system/service/status attributes), the resilience metrics, and an
// access log per call. Pass the system label (e.g. "redis", "gorm", "grpc")
// so calls from several protected clients are distinguishable. A nil inner
// returns nil — no wrapper, so an unarmed client stays untouched.
//
// This is an internal composition step, not a client API: [ClientExecutorFor] applies
// it while the provider-built executor is still private (see [resolve]), so
// clients get an already-observed executor and never wrap one themselves. The
// only other caller is a client that composes its own stack around a raw
// executor it built directly (e.g. httpx's custom ClientExecutor / ResilienceDriver
// paths).
//
// Attaching the breaker listener happens here, while inner is still private:
// the type assertion below is a construction-time handshake, so the executor
// never escapes without its listener and no late binding is needed. Tracer and
// instruments are resolved from whatever OTel providers are current — at
// first-resolve time, not at package init — so an SDK installed after this
// package's init still receives the spans and records.
func WrapClientExecutor(inner ClientExecutor, system, service string) ClientExecutor {
	if inner == nil {
		return nil
	}
	meter := otel.Meter("go-spring.org/cloud/governance/resilience")
	duration, _ := meter.Float64Histogram("resilience.client.duration",
		metric.WithDescription("Duration of resilience-protected calls"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	// calls counts protected calls with the status classified as one of:
	// success, rate_limited, circuit_open, bulkhead_full, timeout, error.
	calls, _ := meter.Int64Counter("resilience.client.calls",
		metric.WithDescription("Number of resilience-protected calls by status"),
		metric.WithUnit("{call}"))
	// breakerChanges counts circuit-breaker state transitions (from/to attrs).
	breakerChanges, _ := meter.Int64Counter("resilience.client.breaker.state_change",
		metric.WithDescription("Circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	w := &wrappedClientExecutor{
		inner:          inner,
		system:         system,
		service:        service,
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

type wrappedClientExecutor struct {
	inner   ClientExecutor
	system  string
	service string

	tracer         trace.Tracer
	duration       metric.Float64Histogram
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
}

// OnBreakerStateChange satisfies [BreakerEventListener]. It is invoked
// synchronously from inside the breaker's transition (so it must not call back
// into the executor); it emits a state-change counter and a log line.
func (w *wrappedClientExecutor) OnBreakerStateChange(service string, from, to BreakerState) {
	w.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	fields := []log.Field{
		log.String("system", w.system),
		log.String("service", w.service),
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
// histogram and the status-classified call counter, and writes the access
// log: a rejection or error at Warn, a success at Debug (protected calls are
// frequent; the success record is there for troubleshooting, not everyday
// reading).
func (w *wrappedClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	start := time.Now()
	ctx, span := w.tracer.Start(ctx, w.service,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("resilience.system", w.system),
			attribute.String("resilience.service", w.service),
		))
	err := w.inner.Execute(ctx, fn)
	status := classifyStatus(err)
	span.SetAttributes(attribute.String("status", status))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	w.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("status", status),
	))
	w.calls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("status", status),
	))

	if err != nil {
		log.Warn(ctx, resilienceTag,
			log.String("system", w.system),
			log.String("service", w.service),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.String("status", status),
			log.Err(err))
		return err
	}
	log.Debug(ctx, resilienceTag, func() []log.Field {
		return []log.Field{
			log.String("system", w.system),
			log.String("service", w.service),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.String("status", status),
		}
	})
	return nil
}

func (w *wrappedClientExecutor) Close() error { return w.inner.Close() }

// Refresh forwards p to the inner executor, keeping the wrapper's
// metrics/listener intact. The inner driver always implements Refresh.
func (w *wrappedClientExecutor) Refresh(p ClientPolicy) error {
	return w.inner.Refresh(p)
}

// WrapServerExecutor returns a [ServerExecutor] that wraps inner with an
// internal span, the inbound admission metrics, and an access log per request —
// the server-side counterpart of [WrapClientExecutor], applied by [Manager.ServerExecutorFor]
// at the same point and for the same reason (the executor is still private when it
// is wrapped, so the breaker-listener handshake below can reach it).
//
// The instruments are a SEPARATE family (resilience.server.*) rather than a
// direction attribute on the outbound ones: the outcome sets genuinely differ —
// inbound has no retry, so no retry_budget_exceeded and no per-attempt timeout —
// and keeping them apart leaves every existing outbound dashboard untouched. The
// access log reuses the resilience tag, with system/service naming the route.
func WrapServerExecutor(inner ServerExecutor, system, service string) ServerExecutor {
	if inner == nil {
		return nil
	}
	meter := otel.Meter("go-spring.org/cloud/governance/resilience")
	duration, _ := meter.Float64Histogram("resilience.server.duration",
		metric.WithDescription("Duration of inbound requests under resilience admission"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	// calls counts admitted requests with the status classified as one of:
	// success, rate_limited, circuit_open, bulkhead_full, timeout, error.
	calls, _ := meter.Int64Counter("resilience.server.calls",
		metric.WithDescription("Number of inbound requests by admission status"),
		metric.WithUnit("{request}"))
	// breakerChanges counts inbound circuit-breaker state transitions (from/to attrs).
	breakerChanges, _ := meter.Int64Counter("resilience.server.breaker.state_change",
		metric.WithDescription("Inbound circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	w := &wrappedServerExecutor{
		inner:          inner,
		system:         system,
		service:        service,
		tracer:         otel.Tracer("go-spring.org/cloud/governance/resilience"),
		duration:       duration,
		calls:          calls,
		breakerChanges: breakerChanges,
	}
	// Construction-time handshake, exactly as in WrapClientExecutor: the breaker does
	// not exist yet, so the listener is installed on the still-private executor
	// and is in place for every transition of its life.
	if setter, ok := inner.(BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedServerExecutor struct {
	inner   ServerExecutor
	system  string
	service string

	tracer         trace.Tracer
	duration       metric.Float64Histogram
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
}

// OnBreakerStateChange satisfies [BreakerEventListener] for the inbound breaker. A
// trip (→open) is a route-level degradation worth flagging at Warn; recovery and
// half-open trial are Info — the same levelling the outbound listener uses.
func (w *wrappedServerExecutor) OnBreakerStateChange(service string, from, to BreakerState) {
	w.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	fields := []log.Field{
		log.String("system", w.system),
		log.String("service", w.service),
		log.String("from", from.String()),
		log.String("to", to.String()),
	}
	if to == BreakerOpen {
		log.Warn(context.Background(), resilienceTag, fields...)
	} else {
		log.Info(context.Background(), resilienceTag, fields...)
	}
}

// Execute wraps the inner call in an internal span, records the duration histogram
// and the status-classified request counter, and writes the access log: a
// rejection or error at Warn, a success at Debug.
func (w *wrappedServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	start := time.Now()
	ctx, span := w.tracer.Start(ctx, w.service,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("resilience.system", w.system),
			attribute.String("resilience.service", w.service),
			attribute.String("resilience.direction", "inbound"),
		))
	err := w.inner.Execute(ctx, fn)
	status := classifyStatus(err)
	span.SetAttributes(attribute.String("status", status))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	w.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("status", status),
	))
	w.calls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("service", w.service),
		attribute.String("status", status),
	))

	if err != nil {
		log.Warn(ctx, resilienceTag,
			log.String("system", w.system),
			log.String("service", w.service),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.String("status", status),
			log.Err(err))
		return err
	}
	log.Debug(ctx, resilienceTag, func() []log.Field {
		return []log.Field{
			log.String("system", w.system),
			log.String("service", w.service),
			log.Float("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
			log.String("status", status),
		}
	})
	return nil
}

func (w *wrappedServerExecutor) Close() error { return w.inner.Close() }

// Refresh forwards a to the inner executor, keeping the wrapper's
// metrics/listener intact.
func (w *wrappedServerExecutor) Refresh(p ServerPolicy) error {
	return w.inner.Refresh(p)
}

// classifyStatus maps an ClientExecutor's return error to the coarse status dimension
// go-spring uses everywhere else for "how did this end".
// The resilience sentinels (rate-limited / circuit-open / bulkhead-full) are
// distinguished from caller timeouts and ordinary errors so a dashboard can
// separate "rejected by protection" from "downstream failed".
func classifyStatus(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, ErrRetryBudgetExceeded):
		return "retry_budget_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "error"
	}
}
