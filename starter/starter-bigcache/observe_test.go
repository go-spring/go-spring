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
	"slices"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

// newTestCache builds a Cache the way a standalone caller does - the raw client
// plus the instance name, then Init - which is what gs does for each configured
// instance, minus the container.
func newTestCache(t *testing.T, name string) *Cache {
	t.Helper()
	client, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	if err != nil {
		t.Fatalf("bigcache.New(%q): %v", name, err)
	}
	if err := client.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set(%q): %v", name, err)
	}
	c := &Cache{BigCache: client, name: name}
	if err := c.Init(); err != nil {
		t.Fatalf("Init(%q): %v", name, err)
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
