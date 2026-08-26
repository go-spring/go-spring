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

// Package resilobserve is the shared resilience-instrumentation adapter for the
// go-spring observability story. It wraps any [resilience.Executor] so each
// Execute emits the observe kit's three signals (internal span + duration/
// in-flight metric + access log) via [observe.New] with ResilienceSemConv, plus
// a call counter classified by outcome — making circuit-breaker trips,
// rate-limit rejects and bulkhead rejections visible in production instead of
// being a black box, which is the gap the core resilience package leaves by
// design (it deliberately does no metric/trace/log).
//
// It lives in the observe package for the same reason observe-gorm /
// observe-lock / observe-transaction exist: the otel-free spring core defines
// the [resilience.Executor] interface, and the instrumentation belongs beside
// the adapters, not in core. A client starter that already builds an Executor wraps
// it once at construction:
//
//	exec = resilobserve.WrapExecutor(exec, "redis", c.Observability)
//
// Everything rides the OTel globals starter-otel installs; without it the
// global tracer/meter are no-ops, so the wrapper adds negligible overhead and
// changes no behaviour. Only the access log always emits (gated by cfg.Level).
package resilobserve

import (
	"context"
	"errors"

	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// WrapExecutor returns a [resilience.Executor] that wraps inner, emitting the
// observe kit's three signals for each Execute plus a call counter with the
// outcome classified as one of: success, rate_limited, circuit_open,
// bulkhead_full, timeout, error (attached as the resilience.outcome attribute).
// Pass the system label (e.g. "redis", "gorm", "grpc") so metrics from several
// protected clients are distinguishable. cfg controls the access log
// (off/brief/detailed). A nil inner returns nil — no wrapper, so an unarmed
// client stays untouched.
func WrapExecutor(inner resilience.Executor, system string, cfg observe.ObserveConfig) resilience.Executor {
	if inner == nil {
		return nil
	}
	meter := otel.Meter("go-spring.org/cloud/observe/resilience")
	calls, _ := meter.Int64Counter("resilience.calls",
		metric.WithDescription("Number of resilience-protected calls by outcome"),
		metric.WithUnit("{call}"))
	breakerChanges, _ := meter.Int64Counter("resilience.breaker.state_change",
		metric.WithDescription("Circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	w := &wrappedExecutor{
		inner:          inner,
		system:         system,
		obs:            observe.New(system, observe.ResilienceSemConv, trace.SpanKindInternal, cfg),
		calls:          calls,
		breakerChanges: breakerChanges,
		logTag:         log.RegisterAppTag(system, "resilience"),
		cfg:            cfg,
	}
	// If the inner executor emits breaker state transitions, subscribe so each
	// trip / half-open / recovery emits a counter + log automatically. Drivers
	// without that capability (no BreakerEventListenerSetter) are silently
	// skipped — the call-level signals still emit.
	if setter, ok := inner.(resilience.BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedExecutor struct {
	inner          resilience.Executor
	system         string
	obs            *observe.Observer
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
	logTag         *log.Tag
	cfg            observe.ObserveConfig
}

// OnBreakerStateChange satisfies [resilience.BreakerEventListener]. It is
// invoked synchronously from inside the breaker's transition (so it must not
// call back into the executor); it emits a state-change counter and a log line.
func (w *wrappedExecutor) OnBreakerStateChange(resource string, from, to resilience.BreakerState) {
	w.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("resilience.system", w.system),
		attribute.String("resilience.resource", resource),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	if w.cfg.Enabled() {
		fields := []log.Field{
			log.String("resilience.resource", resource),
			log.String("from", from.String()),
			log.String("to", to.String()),
		}
		// A trip (→open) is a service-level degradation worth flagging at Warn;
		// recovery and half-open trial are Info.
		if to == resilience.BreakerOpen {
			log.Warn(context.Background(), w.logTag, fields...)
		} else {
			log.Info(context.Background(), w.logTag, fields...)
		}
	}
}

// Execute runs the inner executor under the observe kit: Start opens an internal
// span (resilience.system/resource attributes) and bumps the in-flight gauge,
// and End records the duration histogram, balances the gauge, ends the span and
// emits the access log. The only resilience-specific additions on this path are
// the outcome-classified call counter and the resilience.outcome attribute
// attached through Span.End.
func (w *wrappedExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	ctx, sp := w.obs.Start(ctx, resource, "")
	err := w.inner.Execute(ctx, resource, fn)
	outcome := classifyOutcome(err)
	w.calls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("resilience.system", w.system),
		attribute.String("resilience.resource", resource),
		attribute.String("resilience.outcome", outcome),
	))
	sp.End(err, attribute.String("resilience.outcome", outcome))
	return err
}

func (w *wrappedExecutor) Close() error { return w.inner.Close() }

// Refresh forwards p to the inner executor, keeping the wrapper's
// metrics/listener intact. The inner driver always implements Refresh.
func (w *wrappedExecutor) Refresh(p resilience.Policy) error {
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
	case errors.Is(err, resilience.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, resilience.ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, resilience.ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "error"
	}
}
