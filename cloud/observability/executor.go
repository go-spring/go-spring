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

package observability

import (
	"context"

	"errors"
	"go-spring.org/cloud/chain"
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// resilienceTag is the static log tag for the resilience access log; the
// protected client's system is a log field, not part of the tag.
var resilienceTag = log.RegisterAppTag("resilience", "")

// executorScope names the OTel executorScope this package's spans and instruments
// register under.
const executorScope = "go-spring.org/cloud/resilience"

// executorInstrumentSet is this package's instruments: one per process, resolved lazily
// on first use so it binds to whichever providers are current then, and immutable
// afterwards. It holds no per-client state — the system and service labels travel
// with each wrapper, not here.
//
// The outbound (resilience.client.*) and inbound (resilience.server.*) families
// are separate instruments rather than one family with a direction attribute: the
// outcome sets genuinely differ (inbound has no retry, so no retry_budget_exceeded
// and no per-attempt timeout), and keeping them apart leaves existing outbound
// dashboards untouched.
type executorInstrumentSet struct {
	clientDuration       metric.Float64Histogram
	clientCalls          metric.Int64Counter
	clientBreakerChanges metric.Int64Counter

	serverDuration       metric.Float64Histogram
	serverCalls          metric.Int64Counter
	serverBreakerChanges metric.Int64Counter
}

// instruments is the one instrument set this package uses for the whole process.
var executorInstruments = sync.OnceValue(buildExecutorInstruments)

// resetInstruments makes the next use of executorInstruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func ResetExecutorInstruments() {
	executorInstruments = sync.OnceValue(buildExecutorInstruments)
	resetOperationInstruments()
}

func buildExecutorInstruments() *executorInstrumentSet {
	m := otel.Meter(executorScope)
	in := &executorInstrumentSet{}
	in.clientDuration, _ = m.Float64Histogram("resilience.client.duration",
		metric.WithDescription("Duration of resilience-protected calls"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(DurationBuckets()...))
	// calls counts protected calls on the shared status axis (ok/error) plus,
	// where protection intervened, the resilience.outcome that says what it did
	// (rate_limited, circuit_open, bulkhead_full, retry_budget_exceeded, timeout).
	in.clientCalls, _ = m.Int64Counter("resilience.client.calls",
		metric.WithDescription("Number of resilience-protected calls by status"),
		metric.WithUnit("{call}"))
	// breakerChanges counts circuit-breaker state transitions (from/to attrs).
	in.clientBreakerChanges, _ = m.Int64Counter("resilience.client.breaker.state_change",
		metric.WithDescription("Circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))

	in.serverDuration, _ = m.Float64Histogram("resilience.server.duration",
		metric.WithDescription("Duration of inbound requests under resilience governance"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(DurationBuckets()...))
	in.serverCalls, _ = m.Int64Counter("resilience.server.calls",
		metric.WithDescription("Number of inbound requests by inbound status"),
		metric.WithUnit("{request}"))
	in.serverBreakerChanges, _ = m.Int64Counter("resilience.server.breaker.state_change",
		metric.WithDescription("Inbound circuit-breaker state transitions (from/to attrs)"),
		metric.WithUnit("{event}"))
	return in
}

// resetOperationInstruments drops the memoized sets so the next use resolves a
// fresh one. It exists for tests that install their own MeterProvider, for the
// same reason [resetInstruments] does.
func resetOperationInstruments() {
	opInstruments.mu.Lock()
	defer opInstruments.mu.Unlock()
	opInstruments.m = map[string]*opInstrumentSet{}
	opServerInstruments.mu.Lock()
	defer opServerInstruments.mu.Unlock()
	opServerInstruments.m = map[string]*opServerInstrumentSet{}
}

// successQuiet reports whether a successful call's access line is logged at
// Debug (true) or Info (false).
//
// An UNDECLARED call is this layer's historical quiet case: it has always logged
// success at Debug, and a client that declares nothing (redigo's skipped PING,
// say) declares nothing precisely to keep the noise down — so it stays quiet.
// A declared call is levelled by its detail: one carrying a key or a subject is
// frequent and uninteresting until it fails, one carrying none is worth a line.
func successQuiet(hasOp bool, op Operation) bool {
	if !hasOp {
		return true
	}
	return len(op.Detail) > 0
}

// spanKind is the span kind for a call: the one the client declared, so a
// publish and a consume keep the producer/consumer edge they add to the trace,
// and Internal — the kind an in-process client call has always had — when the
// client declared none or left the kind unspecified.
func spanKind(op Operation, hasOp bool) trace.SpanKind {
	if hasOp && op.SpanKind != trace.SpanKindUnspecified {
		return op.SpanKind
	}
	return trace.SpanKindInternal
}

// attrsToLogFields renders the operation's attributes as log fields, so the log
// keys equal the metric labels' names and a dashboard's selector joins the two.
func attrsToLogFields(attrs []attribute.KeyValue) []log.Field {
	fields := make([]log.Field, 0, len(attrs))
	for _, a := range attrs {
		key := string(a.Key)
		switch a.Value.Type() {
		case attribute.BOOL:
			fields = append(fields, log.Bool(key, a.Value.AsBool()))
		case attribute.INT64:
			fields = append(fields, log.Any(key, a.Value.AsInt64()))
		case attribute.FLOAT64:
			fields = append(fields, log.Float(key, a.Value.AsFloat64()))
		default:
			fields = append(fields, log.String(key, a.Value.AsString()))
		}
	}
	return fields
}

// durMs renders a duration in milliseconds for the access log, the unit the log
// field name promises.
func durMs(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

// classifyStatus maps a chain.Executor's return error onto the status axis
// go-spring uses everywhere else — `ok` / `error`, the two words a cross-family
// query joins on. A protected call is not a special case: it reports the same
// two values as an inbound request or a discovery lookup.
//
// What protection decided is a different question, and it has its own key (see
// [classifyOutcome]). Keeping them apart is the same split as `status` beside
// `rpc.grpc.status_code`: the shared axis carries the outcome, the
// family-specific key carries the detail.
// ClassifyStatus maps an error onto the status axis go-spring uses everywhere
// else — `ok` / `error`, the two words a cross-family query joins on. Engines and
// emitters alike use it, so it is exported from the emission package rather
// than duplicated per engine.
func ClassifyStatus(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

// classifyOutcome names what the resilience stages did to a call, when they did
// anything: they rejected it (rate limit, open circuit, bulkhead) or it ran out
// of time. It is the resilience-side detail of `status`'s shared axis, so the
// protection's own vocabulary stays out of the key every family shares.
//
// It is empty when nothing out of the ordinary happened — a success, or a plain
// downstream error — and an empty outcome is dropped rather than labelled, so
// the label is absent instead of meaningless for the common case.
func classifyOutcome(err error) string {
	switch {
	case errors.Is(err, chain.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, chain.ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, chain.ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, chain.ErrRetryBudgetExceeded):
		return "retry_budget_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return ""
	}
}
