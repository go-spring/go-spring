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

// client_integration_test.go drives THIS package's observer over a scripted
// chain node: the spans, metrics and recorder hand-off a client sees when the
// emitter wraps the rest of the chain. The inner node is a stub on purpose —
// this package owns emission, not retry or admission, so its tests must not
// depend on the governance engine.

package observe

import (
	"context"
	"slices"
	"testing"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/log"
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

// stubChain is the inner chain the emitter wraps in these tests. run scripts
// what the chain does — one call, a retry, a refusal — and records the rounds a
// real engine would have written, so the emitter is exercised against the
// contract it actually consumes.
type stubChain struct {
	run func(context.Context, chain.Operation, *chain.Recorder, func(context.Context) error) error
}

func (s stubChain) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	return s.run(ctx, op, rec, fn)
}

func (stubChain) Close() error { return nil }

// round runs fn once and records the round, the way a chain node does.
func round(ctx context.Context, rec *chain.Recorder, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	rec.AddRound(chain.Round{
		Start:    start,
		Duration: time.Since(start),
		Status:   chain.ClassifyStatus(err),
		Error:    err,
	})
	return err
}

// oneRound is a chain node that runs fn once and records the round.
var oneRound = stubChain{run: func(ctx context.Context, _ chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	return round(ctx, rec, fn)
}}

// twoAttempts retries once, recording each attempt — the shape a retry leaves
// in the recorder.
var twoAttempts = stubChain{run: func(ctx context.Context, _ chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	var err error
	for i := 0; i < 2; i++ {
		err = round(ctx, rec, fn)
		if err == nil {
			break
		}
	}
	return err
}}

// firstServedThenRejected serves the first call and refuses the second before
// it runs, recording the refusal as its own zero-duration round — what an
// admission stage leaves behind.
func firstServedThenRejected() stubChain {
	calls := 0
	return stubChain{run: func(ctx context.Context, _ chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
		calls++
		if calls == 1 {
			return round(ctx, rec, fn)
		}
		err := chain.ErrRateLimited
		rec.AddRound(chain.Round{
			Start:     time.Now(),
			Status:    chain.ClassifyStatus(err),
			ErrorType: chain.ClassifyErrorType(err),
			Error:     err,
		})
		return err
	}}
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

// withMeter installs a manual reader as the global meter provider and drops the
// memoized instrument set, so the signals a test drives resolve against this
// provider rather than one an earlier test installed.
func withMeter(t *testing.T) *metric.ManualReader {
	t.Helper()
	rdr := metric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(metric.NewMeterProvider(metric.WithReader(rdr)))
	ResetInstruments()
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
		var status, errorType string
		for _, kv := range dp.Attributes.ToSlice() {
			switch kv.Key {
			case "status":
				status = kv.Value.AsString()
			case "resilience.error_type":
				errorType = kv.Value.AsString()
			}
		}
		out = append(out, status+"|"+errorType)
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

// TestRoundsAreRecordedPerAttempt proves a retry that lands on a different
// instance keeps the two rounds apart in the write-back: each entry of
// rec.Rounds carries what happened on its own attempt, so "which node is
// failing" stays answerable when the balancer moved the call between rounds.
func TestRoundsAreRecordedPerAttempt(t *testing.T) {
	e := WrapClientExecutor(stubChain{run: func(ctx context.Context, _ chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
		var err error
		for i := 0; i < 2; i++ {
			err = round(ctx, rec, fn)
			if err == nil {
				break
			}
		}
		return err
	}}, "redis", "redis:cache")
	op := chain.Operation{Request: chain.Request{Action: "get"}}
	rec := &chain.Recorder{}
	var i int
	err := e.Execute(context.Background(), op, rec, func(ctx context.Context) error {
		i++
		if i == 1 {
			return errutil.Explain(nil, "transient")
		}
		return nil
	})
	assert.Error(t, err).Nil()
	got := rec.Rounds
	assert.Number(t, len(got)).Equal(2)
	assert.String(t, got[0].Status).Equal("error")
	assert.String(t, got[1].Status).Equal("ok")
}

// TestProducedObservabilityReachesEverySignal proves the write-back side of the
// declared/produced split: a client's own executor appends to the record's
// Observed block (see [chain.Observed]) and each group reaches the signal it
// names — the span attribute, the access-log field, and the CALL-level
// duration's labels — while the declared attributes stay where they were and the
// attempt-level sample stays free of them.
func TestProducedObservabilityReachesEverySignal(t *testing.T) {
	exp := withTracer(t)
	rdr := withMeter(t)
	exec := WrapClientExecutor(oneRound, "memcached", "memcached:svc")
	op := dbOperation()
	rec := &chain.Recorder{}

	err := exec.Execute(context.Background(), op, rec, func(context.Context) error {
		rec.AddLogFields(log.String("cache", "hit"))
		rec.AddSpanAttrs(attribute.String("cache.result", "hit"))
		rec.AddMetricAttrs(attribute.String("cache.result", "hit"))
		return nil
	})
	assert.Error(t, err).Nil()

	// The span carries the produced attribute beside the declared ones.
	spans := exp.GetSpans()
	assert.Number(t, len(spans)).Equal(1)
	spanAttrs := map[string]string{}
	for _, a := range spans[0].Attributes {
		spanAttrs[string(a.Key)] = a.Value.AsString()
	}
	assert.String(t, spanAttrs["cache.result"]).Equal("hit")
	assert.String(t, spanAttrs["peer.service"]).Equal("memcached:svc")

	got := collect(t, rdr)
	call, ok := got["db.client.operation.duration"]
	assert.That(t, ok).True()
	callAttrs := histAttrs(t, call)
	assert.Number(t, len(callAttrs)).Equal(1)
	assert.String(t, callAttrs[0]["cache.result"]).Equal("hit")
	assert.String(t, callAttrs[0]["db.system"]).Equal("memcached")

	// The attempt sample is one round: a supplement to the CALL has no business
	// on it.
	attempts, ok := got["db.client.attempt.duration"]
	assert.That(t, ok).True()
	attemptAttrs := histAttrs(t, attempts)
	assert.Number(t, len(attemptAttrs)).Equal(1)
	_, has := attemptAttrs[0]["cache.result"]
	assert.That(t, has).False("a call-level label must not reach the attempt grain")

	// And the record kept what the client reported, so a caller can read it back.
	assert.Number(t, len(rec.Observed.LogFields)).Equal(1)
}

// TestDeclaredSpanKind proves the emitter opens the call's span with the kind the
// client declared — the producer edge a publish adds to a trace, the consumer
// edge a consume adds — and that an ordinary call of a client falls back to the
// CLIENT edge, which is what makes the outbound call pairable with the server
// edge on the other side.
func TestDeclaredSpanKind(t *testing.T) {
	exp := withTracer(t)
	e := WrapClientExecutor(oneRound, "kafka", "kafka:svc")

	declared := chain.Operation{
		Request: chain.Request{Action: "publish"},
		Observe: chain.Observe{Metric: chain.Metric{Name: "messaging.client"},
			Span: chain.Span{Kind: trace.SpanKindProducer}},
	}
	assert.Error(t, e.Execute(context.Background(), declared, new(chain.Recorder), func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(context.Background(), chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil })).Nil()

	spans := exp.GetSpans()
	assert.Number(t, len(spans)).Equal(2)
	assert.That(t, spans[0].SpanKind).Equal(trace.SpanKindProducer)
	// An undeclared kind is the CLIENT edge: these calls reach another
	// service, a database or a broker, never the current process.
	assert.That(t, spans[1].SpanKind).Equal(trace.SpanKindClient)
}

// TestDeclaredOperationEmitsTwoGrains proves a call that declared its operation
// reports the call level and the attempt level under the operation's own metric
// prefix, and stops reporting the undeclared layer's generic duration.
func TestDeclaredOperationEmitsTwoGrains(t *testing.T) {
	rdr := withMeter(t)
	exec := WrapClientExecutor(twoAttempts, "memcached", "svc")
	op := dbOperation()

	boom := errutil.Explain(nil, "transient")
	var calls int
	err := exec.Execute(context.Background(), op, new(chain.Recorder), func(context.Context) error {
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
	exec := WrapClientExecutor(oneRound, "redis", "svc")

	assert.Error(t, exec.Execute(context.Background(), chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil })).Nil()

	got := collect(t, rdr)
	_, ok := got["resilience.client.duration"]
	assert.That(t, ok).True()
	_, hasOp := got["db.client.operation.duration"]
	assert.That(t, hasOp).False()
	// The status counter is the one signal both paths carry.
	_, hasCalls := got["resilience.client.calls"]
	assert.That(t, hasCalls).True()
}

// TestRejectedCallStillEmits proves a call a protection stage rejects before
// touching the downstream still reports at BOTH grains: the call level and the
// status counter fire, and the refusal lands in the attempt level as its own
// round — zero duration, labelled by the gate that refused it — so refused
// volume is visible next to served volume instead of being silently absent.
func TestRejectedCallStillEmits(t *testing.T) {
	rdr := withMeter(t)
	exec := WrapClientExecutor(firstServedThenRejected(), "memcached", "svc")
	op := dbOperation()

	// First call spends the burst; the second is rejected before running fn.
	assert.Error(t, exec.Execute(context.Background(), op, new(chain.Recorder), func(context.Context) error { return nil })).Nil()
	assert.Error(t, exec.Execute(context.Background(), op, new(chain.Recorder), func(context.Context) error { return nil })).Is(chain.ErrRateLimited)

	got := collect(t, rdr)
	call, ok := got["db.client.operation.duration"]
	assert.That(t, ok).True()
	// `status` is the shared axis, so both calls say `ok`/`error`; what protection
	// did to the rejected one rides `resilience.error_type` instead.
	assert.Slice(t, histPairs(t, call)).Equal([]string{"error|rate_limited", "ok|"})

	attempts, ok := got["db.client.attempt.duration"]
	assert.That(t, ok).True()
	// The served call contributed one real round; the refusal contributed a round
	// of its own, tagged with the gate's verdict.
	assert.Slice(t, histPairs(t, attempts)).Equal([]string{"error|rate_limited", "ok|"})
	assert.Slice(t, histStatuses(t, attempts)).Equal([]string{"error", "ok"})
}
