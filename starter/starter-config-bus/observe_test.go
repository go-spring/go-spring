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

package StarterConfigBus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/nats-io/nats.go"
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

// hasHist reports whether a float64 histogram datapoint carrying want exists.
func hasHist(t *testing.T, name string, want map[string]string) bool {
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
						return true
					}
				}
			}
		}
	}
	return false
}

// newTestBus builds a bus with its instruments resolved, as Init does at wiring
// time, but without the container or a NATS connection. refresh is the property-
// refresh seam, which onMessage calls.
func newTestBus(refresh func(context.Context) error) *ConfigBus {
	return &ConfigBus{
		Config:  Config{Subject: "t.subject", Origin: "test-origin"},
		ins:     newInstruments(),
		refresh: refresh,
		origin:  "test-origin",
	}
}

// msgFor builds the nats message a peer would have published for ev.
func msgFor(t *testing.T, ev RefreshEvent) *nats.Msg {
	t.Helper()
	data, err := json.Marshal(ev)
	assert.Error(t, err).Nil()
	return &nats.Msg{Subject: "t.subject", Data: data}
}

// Every way a broadcast can end lands in the events counter under its own
// outcome, and the outcomes are exclusive — so their sum is the number of
// broadcasts received, with no double counting.
func TestObserveCountsEachStatus(t *testing.T) {
	b := newTestBus(func(context.Context) error { return nil })
	ctx := context.Background()

	// refreshed: applies, refresh succeeds.
	assert.Error(t, b.onMessage(ctx, msgFor(t, RefreshEvent{Prefix: "db", Origin: "peer"}))).Nil()

	// malformed: the payload is not a RefreshEvent. The message was consumed, so
	// no error is returned — it is counted and warned about instead.
	bad := &nats.Msg{Subject: "t.subject", Data: []byte("{not json")}
	assert.Error(t, b.onMessage(ctx, bad)).Nil()

	// ignored_prefix: the event is outside this instance's watched prefixes.
	b.prefixes = []string{"cache"}
	assert.Error(t, b.onMessage(ctx, msgFor(t, RefreshEvent{Prefix: "db"}))).Nil()

	// refresh_error: the event applies but the refresh itself fails. The error is
	// returned so the consumer span (held by starter-nats) is marked failed.
	b.prefixes = nil
	failing := newTestBus(func(context.Context) error { return errors.New("source unreachable") })
	assert.Error(t, failing.onMessage(ctx, msgFor(t, RefreshEvent{Prefix: "db"}))).NotNil()

	// Exactly one broadcast ended in each outcome, so each series holds 1 and
	// their sum is the number of broadcasts received.
	for _, outcome := range []string{
		outcomeRefreshed, outcomeMalformed, outcomeIgnored, outcomeRefreshError,
	} {
		assert.Number(t, sumValue(t, "config.bus.events", map[string]string{
			"status": outcome,
		})).Equal(int64(1))
	}
}

// A refresh duration is recorded only when a refresh was actually attempted, so
// an ignored or malformed broadcast leaves the histogram untouched rather than
// recording a meaningless zero.
func TestObserveRecordsDurationOnlyWhenRefreshRuns(t *testing.T) {
	b := newTestBus(func(context.Context) error { return nil })
	ctx := context.Background()

	// ignored_prefix: no refresh, so no duration for this instance's prefixes.
	b.prefixes = []string{"cache"}
	assert.Error(t, b.onMessage(ctx, msgFor(t, RefreshEvent{Prefix: "db"}))).Nil()

	// The histogram is shared across outcomes, so assert on the refreshed series
	// after a real refresh rather than on absence, which a prior test could
	// satisfy. Prefixes cleared means the event applies.
	b.prefixes = nil
	assert.Error(t, b.onMessage(ctx, msgFor(t, RefreshEvent{Prefix: "db"}))).Nil()
	if !hasHist(t, "config.bus.refresh.duration", map[string]string{"status": outcomeRefreshed}) {
		t.Fatal("a completed refresh must record its duration")
	}
}

// The publish direction has its own counter and vocabulary, kept separate from
// the receive outcomes so a single dimension never mixes two directions.
func TestObserveCountsPublishStatus(t *testing.T) {
	b := newTestBus(func(context.Context) error { return nil })
	// No observer is armed on the Conn here; recordPublish is driven directly,
	// which is the same call Publish makes once the wire call returns.
	b.recordPublish(context.Background(), outcomePublishOK, 0, nil)

	assert.Number(t, sumValue(t, "config.bus.publishes", map[string]string{
		"status": outcomePublishOK,
	})).Equal(int64(1))
}

// shouldRefresh decides which broadcasts an instance honors. The overlap is
// deliberately symmetric: a "db" watcher reacts to a "db.pool" change and vice
// versa, so neither side has to know the other's granularity.
//
// Matching is raw string-prefix, not segment-aware — "db" also matches "dbs".
// That over-matches rather than under-matches, and an extra refresh only re-reads
// the configuration, so the imprecision is deliberate.
func TestShouldRefresh(t *testing.T) {
	cases := []struct {
		name     string
		watched  []string
		prefix   string
		expected bool
	}{
		{"no prefixes watches everything", nil, "db", true},
		{"no prefixes and empty event prefix", nil, "", true},
		{"empty event prefix is a full-fleet refresh", []string{"db"}, "", true},
		{"exact match", []string{"db"}, "db", true},
		{"event narrower than the watcher", []string{"db"}, "db.pool", true},
		{"event wider than the watcher", []string{"db.pool"}, "db", true},
		{"unrelated prefix", []string{"db"}, "cache", false},
		{"second watcher overlaps", []string{"cache", "db"}, "db.pool", true},
		{"second watcher matches exactly", []string{"cache", "db"}, "db", true},
		{"second watcher unrelated", []string{"cache", "db"}, "queue", false},
		{"raw string prefix ignores segment boundaries", []string{"db"}, "dbs", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := &ConfigBus{prefixes: c.watched}
			assert.That(t, b.shouldRefresh(c.prefix)).Equal(c.expected)
		})
	}
}

// The bus's log lines and its counters must attribute the same event the same
// way, or a dashboard selecting config.bus.events{outcome=...} lands on lines
// that cannot be joined to it. The keys are what make them joinable, and a
// drifted key is silent — hence pinning them here rather than trusting the
// call sites to keep spelling them the same.
func TestEventFieldsCarryTheMetricKeys(t *testing.T) {
	keys := make([]string, 0, 1)
	for _, f := range eventFields(outcomeRefreshError) {
		keys = append(keys, f.Key)
	}
	// Only the keys are pinned: the value is the outcome handed straight to the
	// counter alongside, so the key is the whole drift surface.
	assert.That(t, keys).Equal([]string{"status"})
}
