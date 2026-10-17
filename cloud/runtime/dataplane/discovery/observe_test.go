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

package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// installGlobals wires an in-memory OTel meter so the assertions below see real
// records. It is called exactly once, from [TestObserveReporting].
//
// resetInstruments keeps the install deterministic. This package's instrument
// set is process-wide and resolved once, on first use, so it would otherwise
// keep whichever instruments an earlier use in this test binary had already
// resolved - against the no-op provider every test starts with - and these
// records would go nowhere.
func installGlobals(t *testing.T) (sdkmetric.Reader, func()) {
	t.Helper()
	prevMP := otel.GetMeterProvider()

	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))

	otel.SetMeterProvider(mp)
	resetInstruments()
	return rdr, func() {
		otel.SetMeterProvider(prevMP)
		_ = mp.Shutdown(context.Background())
	}
}

// matches reports whether a datapoint's attributes carry every wanted key/value.
func matches(attrs attribute.Set, want map[string]string) bool {
	for k, v := range want {
		got, ok := attrs.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			return false
		}
	}
	return true
}

// findSum collects once and returns the counter value carrying want.
func findSum(t *testing.T, rdr sdkmetric.Reader, name string, want map[string]string) (int64, bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if s, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, p := range s.DataPoints {
					if matches(p.Attributes, want) {
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
}

// findFloatGauge collects once and returns the float64 gauge value carrying want.
func findFloatGauge(t *testing.T, rdr sdkmetric.Reader, name string, want map[string]string) (float64, bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[float64]); ok {
				for _, p := range g.DataPoints {
					if matches(p.Attributes, want) {
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
}

// newObserver builds the observer for one scenario.
func newObserver(t *testing.T, system, center string) *Observer {
	t.Helper()
	o, err := NewObserver(system, center)
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Close() })
	return o
}

// TestObserveReporting drives the read-side scenario under one provider
// install. The OTel global meter binds to the first provider set, so a second
// install in this package would silently record nothing; see installGlobals.
func TestObserveReporting(t *testing.T) {
	rdr, cleanup := installGlobals(t)
	defer cleanup()

	testSyncAgeClimbs(t, rdr)
}

// testSyncAgeClimbs is the load-bearing assertion of this instrumentation: a
// stale snapshot must show an age that KEEPS GROWING. A failed sync has to
// leave the clock alone (the snapshot was not confirmed fresh), otherwise the
// gauge would freeze at zero on exactly the outage it exists to expose.
func testSyncAgeClimbs(t *testing.T, rdr sdkmetric.Reader) {
	const sys, center, svc = "zookeeper", "main", "orders"
	o := newObserver(t, sys, center)
	key := map[string]string{"system": sys, "center": center, "service": svc}

	// Pin the clock so the assertions are about the rule, not about timing.
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	now := base
	prev := obsNow
	obsNow = func() time.Time { return now }
	t.Cleanup(func() { obsNow = prev })

	o.Synced(svc, nil)
	assert.InDelta(t, 0, mustAge(t, rdr, key), 0.001, "a fresh sync has no age")

	// The watch dies: 90 seconds pass with a failed sync in between.
	now = base.Add(90 * time.Second)
	o.Synced(svc, errutil.Explain(nil, "watch closed"))
	assert.InDelta(t, 90, mustAge(t, rdr, key), 0.001,
		"a failed sync must not reset the freshness clock")

	// Another failure later: the age keeps climbing rather than stalling.
	now = base.Add(150 * time.Second)
	o.Synced(svc, errutil.Explain(nil, "watch closed"))
	assert.InDelta(t, 150, mustAge(t, rdr, key), 0.001, "age must keep growing while stale")

	// The watch comes back: freshness is restored.
	o.Synced(svc, nil)
	assert.InDelta(t, 0, mustAge(t, rdr, key), 0.001, "a successful sync restores freshness")

	// Both outcomes are counted, so a failing stretch is visible as a rate.
	if v, ok := findSum(t, rdr, "discovery.sync_total", map[string]string{
		"system": sys, "service": svc, "status": "failed",
	}); assert.True(t, ok) {
		assert.Equal(t, int64(2), v)
	}
	if v, ok := findSum(t, rdr, "discovery.sync_total", map[string]string{
		"system": sys, "service": svc, "status": "ok",
	}); assert.True(t, ok) {
		assert.Equal(t, int64(2), v)
	}
}

// mustAge reads the freshness gauge for an exact (system, service) pair.
func mustAge(t *testing.T, rdr sdkmetric.Reader, want map[string]string) float64 {
	t.Helper()
	v, ok := findFloatGauge(t, rdr, "discovery.cache.age_seconds", want)
	require.True(t, ok, "age gauge missing for %v", want)
	return v
}

// TestAgeAtNeverSynced pins the fallback: a service whose syncs have all failed
// still reports an age (measured from its first attempt) instead of nothing, so
// "never synced" cannot hide behind an absent series.
func TestAgeAtNeverSynced(t *testing.T) {
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	s := syncState{origin: base}

	assert.Equal(t, 45*time.Second, s.ageAt(base.Add(45*time.Second)))
	assert.Zero(t, s.ageAt(base))

	// Once a sync lands, the baseline moves to it.
	s.last = base.Add(20 * time.Second)
	assert.Equal(t, 25*time.Second, s.ageAt(base.Add(45*time.Second)))
}
