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

package StarterBigCache

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/stdlib/singleton"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// TestGaugesReportEveryInstance pins why the cache-statistics gauges are
// registered per cache rather than given a creation-time callback.
//
// The SDK keys an observable instrument by name/description/unit/kind and returns
// the first instrument on every later creation, dropping the new callbacks. It
// reports that collision only through a logr logger at V(1) - which nothing
// installs by default, so the failure is silent: a process holding two caches
// simply exports the first one's statistics and none of the second's. This test
// holds two caches and asserts both are exported, then that destroying one takes
// away only its own series.
func TestGaugesReportEveryInstance(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	resetInstruments()

	hot := newTestCache(t, "hot")
	cold := newTestCache(t, "cold")

	if got, want := gaugeCacheNames(t, reader, "bigcache.entries"), []string{"cold", "hot"}; !slices.Equal(got, want) {
		t.Fatalf("bigcache.entries reported cache.name = %v, want %v", got, want)
	}

	if err := cold.Destroy(); err != nil {
		t.Fatalf("Destroy(cold): %v", err)
	}
	if got, want := gaugeCacheNames(t, reader, "bigcache.entries"), []string{"hot"}; !slices.Equal(got, want) {
		t.Fatalf("after destroying cold, bigcache.entries reported cache.name = %v, want %v", got, want)
	}
	_ = hot
}

// TestCreationTimeCallbackServesOneInstanceOnly pins the SDK behaviour that
// makes the per-cache registration necessary, so the reason survives even if the
// production code is ever rewritten.
//
// Two callers do what a per-instance gauge would do - each creates the same
// observable gauge with its own creation-time callback. Only the first callback
// is ever invoked: a second creation of an already-known descriptor hands back
// the first instrument and drops the new callbacks. A process with two caches
// would export one series, with no error anywhere.
func TestCreationTimeCallbackServesOneInstanceOnly(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

	m := otel.Meter("go-spring.org/starter-bigcache/callback-trap")
	for _, name := range []string{"hot", "cold"} {
		if _, err := m.Int64ObservableGauge("trap.gauge",
			metric.WithDescription("the trap"),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				o.Observe(1, metric.WithAttributes(attribute.String("cache.name", name)))
				return nil
			})); err != nil {
			t.Fatalf("Int64ObservableGauge(%q): %v", name, err)
		}
	}

	if got, want := gaugeCacheNames(t, reader, "trap.gauge"), []string{"hot"}; !slices.Equal(got, want) {
		t.Fatalf("creation-time callbacks reported cache.name = %v, want %v - if this "+
			"now reports both, the SDK stopped dropping repeated callbacks and the "+
			"per-instance registration is no longer required", got, want)
	}
}

// TestStatusOf pins the one judgement this starter makes about an outcome: a
// miss is not a failure. bigcache reports it as an error the caller handles, but
// a cache that answered "not here" is working, so the status axis says ok — the
// hit rate is the gauges' to report, and folding a miss into error would make
// every low-hit-rate cache look broken.
func TestStatusOf(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, statusOK},
		{"miss", bigcache.ErrEntryNotFound, statusOK},
		{"wrapped miss", fmt.Errorf("Get: %w", bigcache.ErrEntryNotFound), statusOK},
		{"backend failure", errors.New("boom"), statusError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusOf(tc.err); got != tc.want {
				t.Fatalf("statusOf(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestObserveEmitsSpanAndMetrics pins the per-call signals: one span per
// operation, named and attributed the same way the metrics are labelled, so a
// trace and a dashboard describe a call identically.
func TestObserveEmitsSpanAndMetrics(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	t.Cleanup(func() { otel.SetTracerProvider(prevTracer) })

	reader := sdkmetric.NewManualReader()
	prevMeter := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	resetInstruments()
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	// The helper seeds "k" on the raw client, so it emits nothing itself.
	c := newTestCache(t, "hot")
	defer func() { _ = c.Destroy() }()

	if _, err := c.Get("k"); err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if _, err := c.Get("absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Get miss: %v", err)
	}
	if err := c.Set("k2", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Delete("k2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	ended := spans.Ended()
	if len(ended) != 4 {
		t.Fatalf("recorded %d spans, want 4", len(ended))
	}
	for i, want := range []struct{ name, op, key string }{
		{"get", opGet, "k"},
		{"get", opGet, "absent"}, // a miss is a normal outcome, and reads as one
		{"set", opSet, "k2"},
		{"delete", opDelete, "k2"},
	} {
		s := ended[i]
		if s.Name() != want.name {
			t.Fatalf("span %d name = %q, want %q", i, s.Name(), want.name)
		}
		// Internal, not client: the dependency is in this process.
		if s.SpanKind() != trace.SpanKindInternal {
			t.Fatalf("span %d kind = %v, want Internal", i, s.SpanKind())
		}
		attrs := spanAttrsOf(s)
		if attrs["bigcache.operation"] != want.op || attrs["bigcache.key"] != want.key || attrs["status"] != statusOK {
			t.Fatalf("span %d attributes = %v, want operation=%s key=%s status=%s",
				i, attrs, want.op, want.key, statusOK)
		}
	}

	got := operationTotals(t, reader)
	want := map[string]int64{"get.ok": 2, "set.ok": 1, "delete.ok": 1}
	if len(got) != len(want) {
		t.Fatalf("bigcache.operation.total reported %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("bigcache.operation.total reported %v, want %v", got, want)
		}
	}
}

// TestObserveMarksFailures pins the other half of the contract: a call that
// fails is a failure on both signals, under the status they share, and the
// caller gets its own error back untouched.
//
// The failure is injected rather than provoked through bigcache, which offers no
// reachable error path for an operation on a live cache — the branch still has to
// work, so it is exercised directly.
func TestObserveMarksFailures(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	t.Cleanup(func() { otel.SetTracerProvider(prevTracer) })

	reader := sdkmetric.NewManualReader()
	prevMeter := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	resetInstruments()
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	c := newTestCache(t, "hot")
	defer func() { _ = c.Destroy() }()

	boom := errors.New("boom")
	err := c.obs.observe(context.Background(), opSet, "k", func(context.Context) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("observe returned %v, want the call's own error", err)
	}

	ended := spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(ended))
	}
	if got := ended[0].Status().Code; got != codes.Error {
		t.Fatalf("span status = %v, want Error", got)
	}
	if got := spanAttrsOf(ended[0])["status"]; got != statusError {
		t.Fatalf("span status attribute = %q, want %q", got, statusError)
	}

	if got := operationTotals(t, reader); got["set.error"] != 1 {
		t.Fatalf("bigcache.operation.total reported %v, want set.error=1", got)
	}
}

// spanAttrsOf flattens a span's attributes.
func spanAttrsOf(s sdktrace.ReadOnlySpan) map[string]string {
	out := make(map[string]string, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}

// operationTotals collects bigcache.operation.total into a "operation.status" ->
// count map, so a caller asserts which outcomes were counted and how often.
func operationTotals(t *testing.T, r *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bigcache.operation.total" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				out[labelsOf(dp.Attributes)] += dp.Value
			}
		}
	}
	return out
}

// labelsOf renders a datapoint's operation and status labels as "operation.status".
func labelsOf(attrs attribute.Set) string {
	op, status := "", ""
	for _, kv := range attrs.ToSlice() {
		switch kv.Key {
		case "operation":
			op = kv.Value.AsString()
		case "status":
			status = kv.Value.AsString()
		}
	}
	return op + "." + status
}

// TestInstrumentFailureIsAnError pins where a meter that will not build its
// instruments lands: as an error out of construction, not a panic out of a
// library and not a silent nil instrument that would panic at the first recorded
// call. Every name, unit and description is a compile-time constant, so the SDK
// never refuses one — the path exists for a custom MeterProvider, and it is
// process-wide, so it is the application's startup error to answer.
func TestInstrumentFailureIsAnError(t *testing.T) {
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(errMeterProvider{})
	resetInstruments()
	t.Cleanup(func() {
		otel.SetMeterProvider(prev)
		resetInstruments()
	})

	if _, err := buildInstruments(); !errors.Is(err, errInstrument) {
		t.Fatalf("buildInstruments() error = %v, want %v", err, errInstrument)
	}

	raw, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	if err != nil {
		t.Fatalf("bigcache.New: %v", err)
	}
	if _, err := NewCache(raw, "hot"); !errors.Is(err, errInstrument) {
		t.Fatalf("NewCache() error = %v, want %v", err, errInstrument)
	}
	// The failed construction owns the raw cache it was handed: the caller has no
	// Cache to close, so leaving it open would strand the eviction goroutine.
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close: %v", err)
	}
}

// resetInstruments makes the next use of instruments resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
//
// The zero value IS the reset — [singleton.Singleton] is ready to use as
// declared. Tests run sequentially in one package, so the plain assignment needs
// no lock. It lives here, with the tests it serves, so the two cannot drift.
func resetInstruments() { instruments = singleton.Singleton[*instrumentSet]{} }

// errInstrument is what the refusing meter below reports.
var errInstrument = errors.New("instrument refused")

// errMeter refuses every instrument. The embedded nil interface supplies the rest
// of metric.Meter - no test reaches those methods.
type errMeter struct{ metric.Meter }

func (errMeter) Int64ObservableGauge(string, ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	return nil, errInstrument
}

// errMeterProvider hands out errMeter for every scope.
type errMeterProvider struct{ metric.MeterProvider }

func (errMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter { return errMeter{} }

// newTestCache builds a Cache the way a standalone caller does - the raw client
// and the instance name, wrapped by NewCache - which is what gs does for each
// configured instance, minus the container.
func newTestCache(t *testing.T, name string) *Cache {
	t.Helper()
	client, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	if err != nil {
		t.Fatalf("bigcache.New(%q): %v", name, err)
	}
	if err := client.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set(%q): %v", name, err)
	}
	c, err := NewCache(client, name)
	if err != nil {
		t.Fatalf("NewCache(%q): %v", name, err)
	}
	return c
}

// gaugeCacheNames collects one gauge and returns the sorted cache.name of every
// datapoint it carries.
func gaugeCacheNames(t *testing.T, r *sdkmetric.ManualReader, instrument string) []string {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	var names []string
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != instrument {
				continue
			}
			gauge, ok := m.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("%s: data is %T, want metricdata.Gauge[int64]", instrument, m.Data)
			}
			for _, dp := range gauge.DataPoints {
				if v, ok := dp.Attributes.Value(attribute.Key("cache.name")); ok {
					names = append(names, v.AsString())
				}
			}
		}
	}
	slices.Sort(names)
	return names
}
