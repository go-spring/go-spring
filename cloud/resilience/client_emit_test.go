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
	"go-spring.org/cloud/chain"
	"slices"
	"testing"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// withTracer installs a recording tracer provider as the global one, so a test
// can read the spans the emitter opens.
func withTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

// TestDeclaredSpanKind proves the emitter opens the call's span with the kind the
// client declared — the producer edge a publish adds to a trace, the consumer
// edge a consume adds — and falls back to Internal when nothing is declared,
// which is the shape an in-process client call has always had.
func TestDeclaredSpanKind(t *testing.T) {
	exp := withTracer(t)
	e := observability.WrapClientExecutor(newBuiltin(t, ClientPolicy{}), "kafka", "kafka:svc")

	ctx := observability.WithOperation(context.Background(), observability.Operation{
		Name:     "publish",
		Metric:   "messaging.client",
		SpanKind: trace.SpanKindProducer,
	})
	assert.Error(t, e.Execute(ctx, func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), func(context.Context) error { return nil })).Nil()

	spans := exp.GetSpans()
	assert.Number(t, len(spans)).Equal(2)
	assert.That(t, spans[0].SpanKind).Equal(trace.SpanKindProducer)
	assert.That(t, spans[1].SpanKind).Equal(trace.SpanKindInternal)
}

// withMeter installs a manual reader as the global meter provider and drops both
// memoized instrument sets, so the signals a test drives resolve against this
// provider rather than one an earlier test installed.
func withMeter(t *testing.T) *metric.ManualReader {
	t.Helper()
	rdr := metric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(metric.NewMeterProvider(metric.WithReader(rdr)))
	observability.ResetExecutorInstruments()
	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return rdr
}

// collect reads every metric the provider holds, keyed by name.
func collect(t *testing.T, rdr *metric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = m
		}
	}
	return out
}

// histStatuses returns the status label of every histogram datapoint, sorted:
// the SDK does not promise datapoint order, so callers compare as a set.
func histStatuses(t *testing.T, m metricdata.Metrics) []string {
	t.Helper()
	h, ok := m.Data.(metricdata.Histogram[float64])
	assert.That(t, ok).True()
	var out []string
	for _, dp := range h.DataPoints {
		for _, kv := range dp.Attributes.ToSlice() {
			if kv.Key == "status" {
				out = append(out, kv.Value.AsString())
			}
		}
	}
	slices.Sort(out)
	return out
}

// histPairs returns "status|outcome" for every histogram datapoint, sorted — the
// pair is asserted together so the two keys cannot be checked against each
// other's datapoint. An absent outcome is the empty string, which is the common
// case: nothing protection did.
func histPairs(t *testing.T, m metricdata.Metrics) []string {
	t.Helper()
	h, ok := m.Data.(metricdata.Histogram[float64])
	assert.That(t, ok).True()
	out := make([]string, 0, len(h.DataPoints))
	for _, dp := range h.DataPoints {
		var status, outcome string
		for _, kv := range dp.Attributes.ToSlice() {
			switch kv.Key {
			case "status":
				status = kv.Value.AsString()
			case "resilience.outcome":
				outcome = kv.Value.AsString()
			}
		}
		out = append(out, status+"|"+outcome)
	}
	slices.Sort(out)
	return out
}

// dbOperation is the identity a database client declares, mirroring what a
// starter will pass once it is migrated.
func dbOperation() observability.Operation {
	return observability.Operation{
		Name:   "get",
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", "memcached"),
			attribute.String("db.operation", "get"),
		},
	}
}

// TestDeclaredOperationEmitsTwoGrains proves a call that declared its operation
// reports the call level and the attempt level under the operation's own metric
// prefix, and stops reporting the undeclared layer's generic duration — the
// whole point of the declared path. One retry means two attempts, the first
// failing and the second succeeding.
func TestDeclaredOperationEmitsTwoGrains(t *testing.T) {
	rdr := withMeter(t)
	exec := observability.WrapClientExecutor(newBuiltin(t, ClientPolicy{MaxRetries: 1}), "memcached", "svc")
	ctx := observability.WithOperation(context.Background(), dbOperation())

	boom := errutil.Explain(nil, "transient")
	var calls int
	err := exec.Execute(ctx, func(context.Context) error {
		calls++
		if calls < 2 {
			return boom
		}
		return nil
	})
	assert.Error(t, err).Nil()
	assert.Number(t, calls).Equal(2)

	got := collect(t, rdr)
	_, hasGeneric := got["resilience.client.duration"]
	assert.That(t, hasGeneric).False()

	call, ok := got["db.client.operation.duration"]
	assert.That(t, ok).True()
	assert.Slice(t, histPairs(t, call)).Equal([]string{"ok|"})

	attempts, ok := got["db.client.attempt.duration"]
	assert.That(t, ok).True()
	assert.Slice(t, histPairs(t, attempts)).Equal([]string{"error|", "ok|"})
}

// TestUndeclaredOperationKeepsHistoricalSignals proves a call that declared no
// operation still reports exactly what this layer has always reported, so a
// deployment on the pre-migration path sees no change.
func TestUndeclaredOperationKeepsHistoricalSignals(t *testing.T) {
	rdr := withMeter(t)
	exec := observability.WrapClientExecutor(newBuiltin(t, ClientPolicy{}), "redis", "svc")

	assert.Error(t, exec.Execute(context.Background(), func(context.Context) error { return nil })).Nil()

	got := collect(t, rdr)
	_, ok := got["resilience.client.duration"]
	assert.That(t, ok).True()
	_, hasOp := got["db.client.operation.duration"]
	assert.That(t, hasOp).False()
	// The status counter is the one signal both paths carry.
	_, hasCalls := got["resilience.client.calls"]
	assert.That(t, hasCalls).True()
}

// TestRejectedCallStillEmits proves a call a protection stage rejects before any
// attempt — nothing downstream was touched — still reports: the call level and
// the status counter fire, and the attempt level stays empty rather than
// inventing a try.
func TestRejectedCallStillEmits(t *testing.T) {
	rdr := withMeter(t)
	exec := observability.WrapClientExecutor(newBuiltin(t, ClientPolicy{RateLimit: 1, Burst: 1}), "memcached", "svc")
	ctx := observability.WithOperation(context.Background(), dbOperation())

	// First call spends the burst; the second is rejected before running fn.
	assert.Error(t, exec.Execute(ctx, func(context.Context) error { return nil })).Nil()
	assert.Error(t, exec.Execute(ctx, func(context.Context) error { return nil })).Is(chain.ErrRateLimited)

	got := collect(t, rdr)
	call, ok := got["db.client.operation.duration"]
	assert.That(t, ok).True()
	// `status` is the shared axis, so both calls say `ok`/`error`; what protection
	// did to the rejected one rides `resilience.outcome` instead.
	assert.Slice(t, histPairs(t, call)).Equal([]string{"error|rate_limited", "ok|"})

	attempts, ok := got["db.client.attempt.duration"]
	assert.That(t, ok).True()
	// Only the first call ran an attempt; the rejected one recorded none.
	assert.Slice(t, histStatuses(t, attempts)).Equal([]string{"ok"})
}
