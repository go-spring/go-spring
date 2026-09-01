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

	observe "go-spring.org/cloud/observe"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// WrapExecutor returns a [Executor] that wraps inner, emitting the
// observe kit's three signals for each Execute plus a call counter with the
// outcome classified as one of: success, rate_limited, circuit_open,
// bulkhead_full, timeout, error (attached as the outcome attribute).
// Pass the system label (e.g. "redis", "gorm", "grpc") so metrics from several
// protected clients are distinguishable. cfg controls the access log
// (off/brief/detailed). A nil inner returns nil — no wrapper, so an unarmed
// client stays untouched.
func WrapExecutor(inner Executor, system string, cfg observe.ObserveConfig) Executor {
	if inner == nil {
		return nil
	}
	meter := otel.Meter("go-spring.org/cloud/governance/resilience")
	calls, _ := meter.Int64Counter("calls",
		metric.WithDescription("Number of resilience-protected calls by outcome"),
		metric.WithUnit("{call}"))
	breakerChanges, _ := meter.Int64Counter("breaker.state_change",
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
	if setter, ok := inner.(BreakerEventListenerSetter); ok {
		setter.SetBreakerEventListener(w)
	}
	return w
}

type wrappedExecutor struct {
	inner          Executor
	system         string
	obs            *observe.Observer
	calls          metric.Int64Counter
	breakerChanges metric.Int64Counter
	logTag         *log.Tag
	cfg            observe.ObserveConfig
}

// OnBreakerStateChange satisfies [BreakerEventListener]. It is
// invoked synchronously from inside the breaker's transition (so it must not
// call back into the executor); it emits a state-change counter and a log line.
func (w *wrappedExecutor) OnBreakerStateChange(resource string, from, to BreakerState) {
	w.breakerChanges.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("resource", resource),
		attribute.String("from", from.String()),
		attribute.String("to", to.String()),
	))
	if w.cfg.Enabled() {
		fields := []log.Field{
			log.String("resource", resource),
			log.String("from", from.String()),
			log.String("to", to.String()),
		}
		// A trip (→open) is a service-level degradation worth flagging at Warn;
		// recovery and half-open trial are Info.
		if to == BreakerOpen {
			log.Warn(context.Background(), w.logTag, fields...)
		} else {
			log.Info(context.Background(), w.logTag, fields...)
		}
	}
}

// Execute runs the inner executor under the observe kit: Start opens an internal
// span (system/resource attributes) and bumps the in-flight gauge,
// and End records the duration histogram, balances the gauge, ends the span and
// emits the access log. The only resilience-specific additions on this path are
// the outcome-classified call counter and the outcome attribute
// attached through Span.End.
func (w *wrappedExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	ctx, sp := w.obs.Start(ctx, resource, "")
	err := w.inner.Execute(ctx, resource, fn)
	outcome := classifyOutcome(err)
	w.calls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("system", w.system),
		attribute.String("resource", resource),
		attribute.String("outcome", outcome),
	))
	sp.End(err, attribute.String("outcome", outcome))
	return err
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
