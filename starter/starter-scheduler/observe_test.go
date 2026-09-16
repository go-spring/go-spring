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

package StarterScheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// testReader collects the instruments this starter emits; testSpans holds its
// spans. Both are installed once in TestMain.
var (
	testReader sdkmetric.Reader
	testSpans  *tracetest.InMemoryExporter
)

// TestMain installs the in-memory OTel providers before any test runs. The OTel
// global meter/tracer binds to the FIRST provider set, so installing here keeps
// these assertions independent of which test happens to touch the
// instrumentation first; installing per-test would record nowhere.
func TestMain(m *testing.M) {
	prevMP := otel.GetMeterProvider()
	prevTP := otel.GetTracerProvider()
	testSpans = tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(testSpans))
	testReader = sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(testReader))
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	code := m.Run()
	otel.SetMeterProvider(prevMP)
	otel.SetTracerProvider(prevTP)
	_ = tp.Shutdown(context.Background())
	_ = mp.Shutdown(context.Background())
	os.Exit(code)
}

// TestInstrumentEmitsSpan pins the trace half of the instrumentation: an
// instrumented run opens a consumer span named after the job, carrying the job
// name attribute, and a failing run's span ends in error with the outcome
// attribute set. Without an SDK installed the wrap is a no-op pass-through, so
// this is the only proof the span link actually exists.
func TestInstrumentEmitsSpan(t *testing.T) {
	testSpans.Reset()
	s := newTestServer()

	_ = s.instrument("t_span", func(context.Context) error { return nil })(context.Background())
	err := s.instrument("t_span_fail", func(context.Context) error { return errors.New("boom") })(context.Background())
	assert.Error(t, err).NotNil()

	found, failed := false, false
	for _, sp := range testSpans.GetSpans() {
		attrs := map[string]string{}
		for _, a := range sp.Attributes {
			attrs[string(a.Key)] = a.Value.AsString()
		}
		switch {
		case attrs["scheduler.job.name"] == "t_span":
			found = true
			if sp.Name != "scheduler.job t_span" {
				t.Fatalf("span name must be %q, got %q", "scheduler.job t_span", sp.Name)
			}
			if attrs["scheduling.outcome"] != "ok" {
				t.Fatalf("a successful run's span must carry outcome=ok, got %q", attrs["scheduling.outcome"])
			}
			if sp.Status.Code == codes.Error {
				t.Fatal("a successful run's span must not be an error")
			}
		case attrs["scheduler.job.name"] == "t_span_fail":
			failed = true
			if attrs["scheduling.outcome"] != "error" {
				t.Fatalf("a failed run's span must carry outcome=error, got %q", attrs["scheduling.outcome"])
			}
			if sp.Status.Code != codes.Error {
				t.Fatal("a failed run's span must end in error status")
			}
		}
	}
	assert.That(t, found).True("no span for the successful run was emitted")
	assert.That(t, failed).True("no span for the failed run was emitted")
}

// attrsMatch reports whether a datapoint carries every wanted key/value.
func attrsMatch(attrs attribute.Set, want map[string]string) bool {
	for k, v := range want {
		got, ok := attrs.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			return false
		}
	}
	return true
}

// trySumValue returns the counter value carrying want, reporting false when no
// such series exists.
func trySumValue(t *testing.T, name string, want map[string]string) (int64, bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.Error(t, testReader.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if s, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, p := range s.DataPoints {
					if attrsMatch(p.Attributes, want) {
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
}

// sumValue returns the counter value carrying want. A missing datapoint fails
// the test: an absent series is exactly the wiring bug this file exists to catch.
func sumValue(t *testing.T, name string, want map[string]string) int64 {
	t.Helper()
	v, ok := trySumValue(t, name, want)
	if !ok {
		t.Fatalf("no %s datapoint for %v", name, want)
	}
	return v
}

// tryHist finds the float64 histogram datapoint carrying want.
func tryHist(t *testing.T, name string, want map[string]string) (metricdata.HistogramDataPoint[float64], bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.Error(t, testReader.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, p := range h.DataPoints {
					if attrsMatch(p.Attributes, want) {
						return p, true
					}
				}
			}
		}
	}
	return metricdata.HistogramDataPoint[float64]{}, false
}

// newTestServer builds a Server with its instruments resolved, as Run does at
// wiring time, but without the container.
func newTestServer() *Server {
	s := &Server{}
	s.instruments = newInstruments()
	return s
}

// Every outcome of a fire lands in the runs counter under its own outcome
// value — including the two ways a fire can be swallowed, which are told apart
// because a lock skip is routine under multi-replica de-duplication while a
// policy skip means the job is not keeping up.
func TestObserveCountsEachOutcome(t *testing.T) {
	s := newTestServer()
	scheduled := time.Now()

	s.observe(scheduling.Event{Name: "t_ok", Scheduled: scheduled, Start: scheduled, Duration: time.Millisecond})
	s.observe(scheduling.Event{
		Name: "t_err", Scheduled: scheduled, Start: scheduled,
		Duration: time.Millisecond, Err: errors.New("boom"),
	})
	s.observe(scheduling.Event{
		Name: "t_panic", Scheduled: scheduled, Start: scheduled,
		Duration: time.Millisecond, Err: errutil.Explain(scheduling.ErrJobPanicked, "job run: %v", "boom"),
	})
	s.observe(scheduling.Event{Name: "t_policy", Skipped: true, Reason: "policy"})
	s.observe(scheduling.Event{Name: "t_lock", Skipped: true, Reason: "lock"})

	for job, outcome := range map[string]string{
		"t_ok":     "ok",
		"t_err":    "error",
		"t_panic":  "panic",
		"t_policy": "skipped_policy",
		"t_lock":   "skipped_lock",
	} {
		assert.Number(t, sumValue(t, "scheduling.runs", map[string]string{
			"job": job, "outcome": outcome,
		})).Equal(int64(1))
	}
}

// A skipped fire has no run, so it must leave the duration and lag histograms
// untouched rather than record a meaningless zero against them.
func TestObserveSkipsHistogramsForSwallowedFires(t *testing.T) {
	s := newTestServer()
	s.observe(scheduling.Event{Name: "t_skip_dur", Skipped: true, Reason: "policy"})

	if _, ok := tryHist(t, "scheduling.run.duration", map[string]string{"job": "t_skip_dur"}); ok {
		t.Fatal("a skipped fire must not record a run duration")
	}
	if _, ok := tryHist(t, "scheduling.lag", map[string]string{"job": "t_skip_dur"}); ok {
		t.Fatal("a skipped fire must not record a wake-up lag")
	}
}

// A run records its duration, and its wake-up lag — the distance between the
// instant it was scheduled for and the instant it actually started. That lag is
// the scheduler's own health signal: a rising one means runs are starting later
// and later, which no other instrument in the framework would show.
func TestObserveRecordsDurationAndLag(t *testing.T) {
	s := newTestServer()
	scheduled := time.Now()
	const lag = 3 * time.Millisecond
	s.observe(scheduling.Event{
		Name:      "t_timing",
		Scheduled: scheduled,
		Start:     scheduled.Add(lag),
		Duration:  40 * time.Millisecond,
	})

	dur, ok := tryHist(t, "scheduling.run.duration", map[string]string{"job": "t_timing", "outcome": "ok"})
	if !ok {
		t.Fatal("a run must record its duration")
	}
	assert.Number(t, dur.Count).Equal(uint64(1))
	assert.That(t, math.Abs(dur.Sum-0.04) < 1e-9).
		True(fmt.Sprintf("duration sum should be 40ms, got %v", dur.Sum))

	l, ok := tryHist(t, "scheduling.lag", map[string]string{"job": "t_timing"})
	if !ok {
		t.Fatal("a run must record its wake-up lag")
	}
	assert.Number(t, l.Count).Equal(uint64(1))
	assert.That(t, math.Abs(l.Sum-lag.Seconds()) < 1e-9).
		True(fmt.Sprintf("lag sum should be 3ms, got %v", l.Sum))
}
