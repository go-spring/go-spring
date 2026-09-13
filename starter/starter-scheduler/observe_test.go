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
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// testReader collects the instruments this starter emits. It is installed once
// in TestMain.
var testReader sdkmetric.Reader

// TestMain installs the in-memory meter provider before any test runs. The OTel
// global meter binds to the FIRST provider set, so installing here keeps these
// assertions independent of which test happens to touch the instrumentation
// first; installing per-test would record nowhere.
func TestMain(m *testing.M) {
	prev := otel.GetMeterProvider()
	testReader = sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(testReader))
	otel.SetMeterProvider(mp)
	code := m.Run()
	otel.SetMeterProvider(prev)
	_ = mp.Shutdown(context.Background())
	os.Exit(code)
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
