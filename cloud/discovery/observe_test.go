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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// installGlobals wires in-memory OTel providers so the assertions below see
// real spans and records. It is called exactly once, from [TestObserveReporting];
// every scenario is driven under that single install.
//
// resetInstruments keeps the install deterministic. This package's instrument
// set is process-wide and resolved once, on first use, so it would otherwise
// keep whichever instruments an earlier use in this test binary had already
// resolved - against the no-op provider every test starts with - and these
// records would go nowhere. Nothing resolves it before this point today; the
// reset is what keeps that from being a load-bearing ordering accident.
func installGlobals(t *testing.T) (*tracetest.InMemoryExporter, sdkmetric.Reader, func()) {
	t.Helper()
	prevTP := otel.GetTracerProvider()
	prevMP := otel.GetMeterProvider()

	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	resetInstruments()
	return spanExp, rdr, func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		_ = tp.Shutdown(context.Background())
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

// findHist collects once and returns the duration datapoint carrying want.
func findHist(t *testing.T, rdr sdkmetric.Reader, name string, want map[string]string) (metricdata.HistogramDataPoint[float64], bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, p := range h.DataPoints {
					if matches(p.Attributes, want) {
						return p, true
					}
				}
			}
		}
	}
	return metricdata.HistogramDataPoint[float64]{}, false
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

// findIntGauge collects once and returns the int64 gauge value carrying want.
func findIntGauge(t *testing.T, rdr sdkmetric.Reader, name string, want map[string]string) (int64, bool) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[int64]); ok {
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

// spanAttr reads one attribute off a span stub.
func spanAttr(s tracetest.SpanStub, key string) (string, bool) {
	for _, a := range s.Attributes {
		if a.Key == attribute.Key(key) {
			return a.Value.AsString(), true
		}
	}
	return "", false
}

// TestObserveReporting drives every scenario under one provider install. The
// OTel global meter binds to the first provider set, so a second install in this
// package would silently record nothing; see installGlobals.
func TestObserveReporting(t *testing.T) {
	spanExp, rdr, cleanup := installGlobals(t)
	defer cleanup()

	testRegistryLifecycle(t, spanExp, rdr)
	testRegistryFailureSemantics(t, rdr)
	testWeightChangeObserved(t, spanExp, rdr)
	testSyncAgeClimbs(t, rdr)
	testTwoCentersStayDistinct(t, rdr)
}

// testTwoCentersStayDistinct pins why center is part of the identity: one process
// publishes the same service into EVERY configured center (the discovery core
// drives all collected registrys), and each block keeps its own state. Without
// center the two blocks would be one series — and now that the state is per
// block, two datapoints sharing a label set, which is a scrape error rather than
// a wrong number.
//
// Closing must also take a block's series away: a re-armed watch or a
// re-registered instance rebuilds its block, and a callback left behind would
// report the same service twice for as long as the process lived.
func testTwoCentersStayDistinct(t *testing.T, rdr sdkmetric.Reader) {
	const sys, svc = "etcd", "orders"
	alpha := newObserver(t, sys, "alpha")
	beta := newObserver(t, sys, "beta")

	reportRegister(alpha, svc, ReasonInitial, nil)
	reportRegister(beta, svc, ReasonInitial, errutil.Explain(nil, "center unreachable"))

	assert.Equal(t, int64(1), mustGauge(t, rdr, map[string]string{"system": sys, "center": "alpha", "service": svc}))
	assert.Equal(t, int64(0), mustGauge(t, rdr, map[string]string{"system": sys, "center": "beta", "service": svc}))

	require.NoError(t, beta.Close())
	if _, ok := findIntGauge(t, rdr, "discovery.instance.registered", map[string]string{
		"system": sys, "center": "beta", "service": svc,
	}); assert.False(t, ok, "a closed observer must drop its series") {
	}
	assert.Equal(t, int64(1), mustGauge(t, rdr, map[string]string{"system": sys, "center": "alpha", "service": svc}),
		"closing one block must leave its siblings reporting")
}

// newObserver builds the observer for one scenario. Its gauge callbacks are
// dropped at the end of the test: an observer that outlived its scenario would
// keep reporting into the next scenario's collection.
func newObserver(t *testing.T, system, center string) *Observer {
	t.Helper()
	o, err := NewObserver(system, center)
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Close() })
	return o
}

// reportRegister / reportDeregister / reportWeight run one reported operation to
// completion with the given outcome: these tests assert on the instruments, and
// the span-context correlation is covered by the backends' own tests.
func reportRegister(o *Observer, svc, reason string, err error) {
	_ = o.RegisterAttempt(context.Background(), svc, reason, func(context.Context) error { return err })
}

func reportDeregister(o *Observer, svc string, err error) {
	_ = o.DeregisterAttempt(context.Background(), svc, func(context.Context) error { return err })
}

func reportWeight(o *Observer, svc string, err error) {
	_ = o.WeightChange(context.Background(), svc, func(context.Context) error { return err })
}

// testRegistryLifecycle asserts each registration outcome's span, duration,
// counter and the registered gauge — including the self-heal path, which never
// passes through the Registry interface.
func testRegistryLifecycle(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	fail := errutil.Explain(nil, "center unreachable")
	const sys, center, svc = "etcd", "main", "orders"
	o := newObserver(t, sys, center)

	// --- initial registration succeeds: span, duration, counter, gauge=1 ---
	reportRegister(o, svc, ReasonInitial, nil)

	spans := spanExp.GetSpans()
	require.NotEmpty(t, spans)
	last := spans[len(spans)-1]
	assert.Equal(t, opRegister, last.Name)
	if got, ok := spanAttr(last, "discovery.reason"); assert.True(t, ok) {
		assert.Equal(t, ReasonInitial, got)
	}
	if got, ok := spanAttr(last, "discovery.system"); assert.True(t, ok) {
		assert.Equal(t, sys, got)
	}

	if _, ok := findHist(t, rdr, "discovery.operation.duration", map[string]string{
		"system": sys, "operation": opRegister, "service": svc, "status": "ok",
	}); !assert.True(t, ok, "register duration datapoint missing") {
		t.FailNow()
	}
	if v, ok := findSum(t, rdr, "discovery.registration.attempts_total", map[string]string{
		"system": sys, "service": svc, "reason": ReasonInitial, "status": "ok",
	}); assert.True(t, ok, "initial attempts counter missing") {
		assert.Equal(t, int64(1), v)
	}
	if v, ok := findIntGauge(t, rdr, "discovery.instance.registered", map[string]string{
		"system": sys, "service": svc,
	}); assert.True(t, ok, "registered gauge missing") {
		assert.Equal(t, int64(1), v)
	}

	// --- a failed self-heal declares the instance unpublishable ---
	reportRegister(o, svc, ReasonSelfHeal, fail)
	if v, ok := findSum(t, rdr, "discovery.registration.attempts_total", map[string]string{
		"system": sys, "service": svc, "reason": ReasonSelfHeal, "status": "failed",
	}); assert.True(t, ok, "self-heal failure counter missing") {
		assert.Equal(t, int64(1), v)
	}
	if v, ok := findIntGauge(t, rdr, "discovery.instance.registered", map[string]string{
		"system": sys, "service": svc,
	}); assert.True(t, ok) {
		assert.Equal(t, int64(0), v, "a failed re-register must report the instance as unpublished")
	}

	// --- the center recovers: the next self-heal succeeds and the gauge clears ---
	reportRegister(o, svc, ReasonSelfHeal, nil)
	if v, _ := findIntGauge(t, rdr, "discovery.instance.registered", map[string]string{
		"system": sys, "service": svc,
	}); assert.Equal(t, int64(1), v) {
	}
}

// testRegistryFailureSemantics pins the two cases where the gauge must NOT
// simply follow the error: a failed deregister leaves the instance published
// (that is exactly the leak worth alerting on), while a successful one clears it.
func testRegistryFailureSemantics(t *testing.T, rdr sdkmetric.Reader) {
	const sys, center, svc = "consul", "main", "orders"
	o := newObserver(t, sys, center)
	want := map[string]string{"system": sys, "center": center, "service": svc}

	reportRegister(o, svc, ReasonInitial, nil)
	require.Equal(t, int64(1), mustGauge(t, rdr, want))

	reportDeregister(o, svc, errutil.Explain(nil, "revoke failed"))
	assert.Equal(t, int64(1), mustGauge(t, rdr, want),
		"a failed deregister must not claim the instance is gone")

	reportDeregister(o, svc, nil)
	assert.Equal(t, int64(0), mustGauge(t, rdr, want))
}

// mustGauge reads the registered gauge for an exact (system, service) pair.
func mustGauge(t *testing.T, rdr sdkmetric.Reader, want map[string]string) int64 {
	t.Helper()
	v, ok := findIntGauge(t, rdr, "discovery.instance.registered", want)
	require.True(t, ok, "registered gauge missing for %v", want)
	return v
}

// testWeightChangeObserved asserts a weight re-advertisement is a first-class
// operation on the duration metric — it is the drain path, and the one the
// discovery core does not log.
func testWeightChangeObserved(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	const sys, center, svc = "nacos", "main", "orders"
	o := newObserver(t, sys, center)

	reportWeight(o, svc, nil)

	spans := spanExp.GetSpans()
	require.NotEmpty(t, spans)
	assert.Equal(t, opUpdateWeight, spans[len(spans)-1].Name)

	_, ok := findHist(t, rdr, "discovery.operation.duration", map[string]string{
		"system": sys, "operation": opUpdateWeight, "service": svc, "status": "ok",
	})
	assert.True(t, ok, "update_weight duration datapoint missing")
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
