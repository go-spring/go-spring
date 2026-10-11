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

// emitter_test.go exercises the OBSERVE layer composed over the resilience
// engines — spans, metrics and the recorder hand-off. It is an EXTERNAL test
// package (resilience_test) because the observe wrapper lives in
// cloud/observability, which imports this package: an internal test importing it
// would be an import cycle. Here the dependency direction is the natural one —
// the observer reads the engine — so the package under test can be wrapped by
// observability exactly as the center wraps it in production.

package resilience_test

import (
	"context"
	"slices"
	"testing"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
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

// newBuiltin builds the bundled client engine for one service under p.
func newBuiltin(t *testing.T, p resilience.ClientPolicy) chain.Executor {
	t.Helper()
	e, err := resilience.NewDefaultDriver(nil).NewClientExecutor("svc", p)
	assert.Error(t, err).Nil()
	return e
}

// observedServer builds the bundled inbound engine for one route, wrapped by the
// observe layer exactly as the center wraps it — the executor the middleware runs
// under.
func observedServer(t *testing.T, system, service string) chain.Executor {
	t.Helper()
	return observability.WrapServerExecutor(resilience.NewManager(nil).ServerExecutorFor(system, service), system, service)
}

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

// histAttrs returns every histogram datapoint's attributes as a key→value map.
func histAttrs(t *testing.T, m metricdata.Metrics) []map[string]string {
	t.Helper()
	h, ok := m.Data.(metricdata.Histogram[float64])
	assert.That(t, ok).True()
	out := make([]map[string]string, 0, len(h.DataPoints))
	for _, dp := range h.DataPoints {
		kv := map[string]string{}
		for _, a := range dp.Attributes.ToSlice() {
			kv[string(a.Key)] = a.Value.Emit()
		}
		out = append(out, kv)
	}
	return out
}

// dbOperation is the identity a database client declares, mirroring what a
// starter passes once it is migrated.
func dbOperation() chain.Operation {
	return chain.Operation{
		Name:   "get",
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", "memcached"),
			attribute.String("db.operation", "get"),
		},
	}
}

// TestNonIdempotentOperationIsNotRetried proves a caller that declares its call
// non-idempotent (see [chain.RetryPolicy.NonIdempotent]) runs ONCE however many
// retries its retry node asks for. The suppression must survive the wrapper in
// between, which is the only executor a client ever holds.
func TestNonIdempotentOperationIsNotRetried(t *testing.T) {
	e := observability.WrapClientExecutor(newBuiltin(t, resilience.ClientPolicy{}), "mail", "mail:svc")
	op := chain.Operation{
		Name:   "send",
		Metric: "email.client",
		Retry:  chain.RetryPolicy{MaxRetries: 3, NonIdempotent: true},
	}
	var attempts int
	err := e.Execute(context.Background(), op, func(context.Context) error {
		attempts++
		return errutil.Explain(nil, "transient")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, attempts).Equal(1)
}

// TestDeclaredSpanKind proves the emitter opens the call's span with the kind the
// client declared — the producer edge a publish adds to a trace, the consumer
// edge a consume adds — and falls back to Internal when nothing is declared.
func TestDeclaredSpanKind(t *testing.T) {
	exp := withTracer(t)
	e := observability.WrapClientExecutor(newBuiltin(t, resilience.ClientPolicy{}), "kafka", "kafka:svc")

	declared := chain.Operation{
		Name:     "publish",
		Metric:   "messaging.client",
		SpanKind: trace.SpanKindProducer,
	}
	assert.Error(t, e.Execute(context.Background(), declared, func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil })).Nil()

	spans := exp.GetSpans()
	assert.Number(t, len(spans)).Equal(2)
	assert.That(t, spans[0].SpanKind).Equal(trace.SpanKindProducer)
	assert.That(t, spans[1].SpanKind).Equal(trace.SpanKindInternal)
}

// TestDeclaredOperationEmitsTwoGrains proves a call that declared its operation
// reports the call level and the attempt level under the operation's own metric
// prefix, and stops reporting the undeclared layer's generic duration.
func TestDeclaredOperationEmitsTwoGrains(t *testing.T) {
	rdr := withMeter(t)
	exec := observability.WrapClientExecutor(newBuiltin(t, resilience.ClientPolicy{Retry: chain.RetryPolicy{MaxRetries: 1}}), "memcached", "svc")
	op := dbOperation()

	boom := errutil.Explain(nil, "transient")
	var calls int
	err := exec.Execute(context.Background(), op, func(context.Context) error {
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
	exec := observability.WrapClientExecutor(newBuiltin(t, resilience.ClientPolicy{}), "redis", "svc")

	assert.Error(t, exec.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil })).Nil()

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
	exec := observability.WrapClientExecutor(newBuiltin(t, resilience.ClientPolicy{RateLimit: 1, Burst: 1}), "memcached", "svc")
	op := dbOperation()

	// First call spends the burst; the second is rejected before running fn.
	assert.Error(t, exec.Execute(context.Background(), op, func(context.Context) error { return nil })).Nil()
	assert.Error(t, exec.Execute(context.Background(), op, func(context.Context) error { return nil })).Is(chain.ErrRateLimited)

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

// TestDeclaredServerOperationEmitsFamilySignals proves an inbound route that
// declares its operation is served like a client is: its family metric under the
// FAMILY's name (http.server.request.duration, not the client side's
// .operation.duration) labelled by the route's bounded attributes, plus the one
// thing only a server has — a response half the handler records after answering.
func TestDeclaredServerOperationEmitsFamilySignals(t *testing.T) {
	rdr := withMeter(t)
	exec := observedServer(t, "http-server", "http-server::9090")

	declared := chain.Operation{
		Name:   "GET /users",
		Metric: "http.server",
		Attrs: []attribute.KeyValue{
			attribute.String("http.request.method", "GET"),
			attribute.String("http.route", "/users"),
		},
	}
	err := exec.Execute(context.Background(), declared, func(ctx context.Context) error {
		observability.ResponseFrom(ctx).Add(attribute.Int("http.response.status_code", 200))
		return nil
	})
	assert.Error(t, err).Nil()

	got := collect(t, rdr)
	fam, ok := got["http.server.request.duration"]
	assert.That(t, ok).True("the family name must be kept, not renamed to .operation.duration")
	pts := histAttrs(t, fam)
	assert.Number(t, len(pts)).Equal(1)
	assert.String(t, pts[0]["http.request.method"]).Equal("GET")
	assert.String(t, pts[0]["http.route"]).Equal("/users")
	// The response half reaches the metric because the metric is recorded after
	// the handler answered — which is exactly why it is a carrier and not part of
	// the declaration.
	assert.String(t, pts[0]["http.response.status_code"]).Equal("200")
	assert.String(t, pts[0]["status"]).Equal("ok")

	_, ok = got["http.server.active_requests"]
	assert.That(t, ok).True("the in-flight gauge must be emitted for a declared route")
}

// TestUndeclaredServerRouteKeepsItsSignals proves a route that declares nothing
// still reports exactly what this layer always reported: the inbound metrics
// under resilience.server.*, and no family metric invented on its behalf.
func TestUndeclaredServerRouteKeepsItsSignals(t *testing.T) {
	rdr := withMeter(t)
	exec := observedServer(t, "http-server", "http-server::9090")

	assert.Error(t, exec.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil })).Nil()

	got := collect(t, rdr)
	_, ok := got["resilience.server.duration"]
	assert.That(t, ok).True()
	_, ok = got["http.server.request.duration"]
	assert.That(t, ok).False("an undeclared route must not invent a family metric")
}
