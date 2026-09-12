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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// installGlobals wires in-memory OTel providers so the assertions below see
// real spans and records. It is called exactly once, from [TestObserveReporting];
// every scenario is driven under that single install because the OTel global
// meter caches by name and only ever delegates to the FIRST provider set — a
// second install would resolve instruments that record nowhere.
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
}

// testRegistryLifecycle asserts each registration outcome's span, duration,
// counter and the registered gauge — including the self-heal path, which never
// passes through the Registrar interface.
func testRegistryLifecycle(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	ctx := context.Background()
	fail := errors.New("center unreachable")
	const sys, svc = "etcd", "orders"

	// --- initial registration succeeds: span, duration, counter, gauge=1 ---
	RegisterAttempt(ctx, sys, svc, ReasonInitial)(nil)

	spans := spanExp.GetSpans()
	require.NotEmpty(t, spans)
	last := spans[len(spans)-1]
	assert.Equal(t, opRegister, last.Name)
	if got, ok := spanAttr(last, "registry.reason"); assert.True(t, ok) {
		assert.Equal(t, ReasonInitial, got)
	}
	if got, ok := spanAttr(last, "registry.system"); assert.True(t, ok) {
		assert.Equal(t, sys, got)
	}

	if _, ok := findHist(t, rdr, "registry.operation.duration", map[string]string{
		"system": sys, "operation": opRegister, "service": svc, "status": "ok",
	}); !assert.True(t, ok, "register duration datapoint missing") {
		t.FailNow()
	}
	if v, ok := findSum(t, rdr, "registry.registration.attempts_total", map[string]string{
		"system": sys, "service": svc, "reason": ReasonInitial, "status": "ok",
	}); assert.True(t, ok, "initial attempts counter missing") {
		assert.Equal(t, int64(1), v)
	}
	if v, ok := findIntGauge(t, rdr, "registry.instance.registered", map[string]string{
		"system": sys, "service": svc,
	}); assert.True(t, ok, "registered gauge missing") {
		assert.Equal(t, int64(1), v)
	}

	// --- a failed self-heal declares the instance unpublishable ---
	RegisterAttempt(ctx, sys, svc, ReasonSelfHeal)(fail)
	if v, ok := findSum(t, rdr, "registry.registration.attempts_total", map[string]string{
		"system": sys, "service": svc, "reason": ReasonSelfHeal, "status": "failed",
	}); assert.True(t, ok, "self-heal failure counter missing") {
		assert.Equal(t, int64(1), v)
	}
	if v, ok := findIntGauge(t, rdr, "registry.instance.registered", map[string]string{
		"system": sys, "service": svc,
	}); assert.True(t, ok) {
		assert.Equal(t, int64(0), v, "a failed re-register must report the instance as unpublished")
	}

	// --- the center recovers: the next self-heal succeeds and the gauge clears ---
	RegisterAttempt(ctx, sys, svc, ReasonSelfHeal)(nil)
	if v, _ := findIntGauge(t, rdr, "registry.instance.registered", map[string]string{
		"system": sys, "service": svc,
	}); assert.Equal(t, int64(1), v) {
	}
}

// testRegistryFailureSemantics pins the two cases where the gauge must NOT
// simply follow the error: a failed deregister leaves the instance published
// (that is exactly the leak worth alerting on), while a successful one clears it.
func testRegistryFailureSemantics(t *testing.T, rdr sdkmetric.Reader) {
	ctx := context.Background()
	const sys, svc = "consul", "orders"
	want := map[string]string{"system": sys, "service": svc}

	RegisterAttempt(ctx, sys, svc, ReasonInitial)(nil)
	require.Equal(t, int64(1), mustGauge(t, rdr, want))

	DeregisterAttempt(ctx, sys, svc)(errors.New("revoke failed"))
	assert.Equal(t, int64(1), mustGauge(t, rdr, want),
		"a failed deregister must not claim the instance is gone")

	DeregisterAttempt(ctx, sys, svc)(nil)
	assert.Equal(t, int64(0), mustGauge(t, rdr, want))
}

// mustGauge reads the registered gauge for an exact (system, service) pair.
func mustGauge(t *testing.T, rdr sdkmetric.Reader, want map[string]string) int64 {
	t.Helper()
	v, ok := findIntGauge(t, rdr, "registry.instance.registered", want)
	require.True(t, ok, "registered gauge missing for %v", want)
	return v
}

// testWeightChangeObserved asserts a weight re-advertisement is a first-class
// operation on the duration metric — it is the drain path, and the one the
// registry core does not log.
func testWeightChangeObserved(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	ctx := context.Background()
	const sys, svc = "nacos", "orders"

	WeightChange(ctx, sys, svc)(nil)

	spans := spanExp.GetSpans()
	require.NotEmpty(t, spans)
	assert.Equal(t, opUpdateWeight, spans[len(spans)-1].Name)

	_, ok := findHist(t, rdr, "registry.operation.duration", map[string]string{
		"system": sys, "operation": opUpdateWeight, "service": svc, "status": "ok",
	})
	assert.True(t, ok, "update_weight duration datapoint missing")
}

// testSyncAgeClimbs is the load-bearing assertion of this instrumentation: a
// stale snapshot must show an age that KEEPS GROWING. A failed sync has to
// leave the clock alone (the snapshot was not confirmed fresh), otherwise the
// gauge would freeze at zero on exactly the outage it exists to expose.
func testSyncAgeClimbs(t *testing.T, rdr sdkmetric.Reader) {
	const sys, svc = "zookeeper", "orders"
	key := map[string]string{"system": sys, "service": svc}

	// Pin the clock so the assertions are about the rule, not about timing.
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	now := base
	prev := obsNow
	obsNow = func() time.Time { return now }
	t.Cleanup(func() { obsNow = prev })

	Synced(sys, svc, nil)
	assert.InDelta(t, 0, mustAge(t, rdr, key), 0.001, "a fresh sync has no age")

	// The watch dies: 90 seconds pass with a failed sync in between.
	now = base.Add(90 * time.Second)
	Synced(sys, svc, errors.New("watch closed"))
	assert.InDelta(t, 90, mustAge(t, rdr, key), 0.001,
		"a failed sync must not reset the freshness clock")

	// Another failure later: the age keeps climbing rather than stalling.
	now = base.Add(150 * time.Second)
	Synced(sys, svc, errors.New("watch closed"))
	assert.InDelta(t, 150, mustAge(t, rdr, key), 0.001, "age must keep growing while stale")

	// The watch comes back: freshness is restored.
	Synced(sys, svc, nil)
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
