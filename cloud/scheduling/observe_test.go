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

package scheduling_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// withTracer installs a real tracer provider for a test; the run's span is
// started from whichever provider is current at that moment.
func withTracer(t *testing.T) {
	t.Helper()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
}

// withoutTracer installs a no-op tracer provider, so a run's span carries no
// valid identity. A test that asserts the run names itself must say so itself
// rather than trusting the ambient provider.
func withoutTracer(t *testing.T) {
	t.Helper()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(noop.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
}

// TestRunLeavesIdentityToItsSpan proves a run does not mint a trace_id of its
// own when a tracer is installed: the span it opens supplies one, and a minted
// id would put a second value under the same key.
func TestRunLeavesIdentityToItsSpan(t *testing.T) {
	withTracer(t)

	s := scheduling.NewScheduler()
	var mu sync.Mutex
	var runs []context.Context
	_, err := s.Schedule(mustJob(t, "spanned", scheduling.FixedRate(20*time.Millisecond),
		func(ctx context.Context) error {
			mu.Lock()
			runs = append(runs, ctx)
			mu.Unlock()
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(60 * time.Millisecond) // ~3 runs
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(runs) >= 1).True("expected at least 1 run")
	assert.That(t, !strings.Contains(renderFields(log.CarriedFields(runs[0])), "trace_id=")).
		True("the run must not mint a trace_id when its span already provides one")
}

// withMeter installs a manual reader as the global meter provider for a test;
// the built-in instrumentation binds to the provider current at record time, so
// every fire the test triggers reports into the reader.
func withMeter(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	rdr := sdkmetric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr)))
	// The instrument set is process-wide and resolved once, so a test running
	// after one that already resolved it would keep reporting into the earlier
	// provider.
	scheduling.ResetInstruments()
	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return rdr
}

// fireCounts collects scheduling.runs into a "job|status" -> count map: the
// statuses are exclusive, so the sum over the dimension is the number of fires.
func fireCounts(t *testing.T, rdr *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "scheduling.runs" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				job, status := "", ""
				for _, kv := range dp.Attributes.ToSlice() {
					switch kv.Key {
					case "job":
						job = kv.Value.AsString()
					case "status":
						status = kv.Value.AsString()
					}
				}
				out[job+"|"+status] += dp.Value
			}
		}
	}
	return out
}

// TestBuiltinObservabilityCountsFires pins the built-in instrumentation: every
// fire of a failing job reports one scheduling.runs record with job and status.
func TestBuiltinObservabilityCountsFires(t *testing.T) {
	rdr := withMeter(t)
	s := scheduling.NewScheduler()

	n := 0
	_, err := s.Schedule(mustJob(t, "obs", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error {
			n++
			if n == 1 {
				return errors.New("boom")
			}
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(45 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	got := fireCounts(t, rdr)
	assert.That(t, got["obs|error"]).Equal(int64(1))
	assert.That(t, got["obs|ok"] >= 1).True("later fires should report ok")
}
