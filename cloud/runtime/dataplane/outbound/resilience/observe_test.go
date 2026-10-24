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

package resilience

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// installStateMeter wires an in-memory meter and resets the package-level
// instrument set so this test binds to it rather than to whatever an earlier
// test resolved against the no-op provider.
func installStateMeter(t *testing.T) (sdkmetric.Reader, func()) {
	t.Helper()
	prev := otel.GetMeterProvider()
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))
	otel.SetMeterProvider(mp)
	resetInstruments()
	return rdr, func() {
		otel.SetMeterProvider(prev)
		_ = mp.Shutdown(context.Background())
	}
}

func collect(t *testing.T, rdr sdkmetric.Reader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := rdr.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	return rm
}

func attrsMatch(set attribute.Set, want map[string]string) bool {
	for k, v := range want {
		got, ok := set.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			return false
		}
	}
	return true
}

func gaugeValue(t *testing.T, rm metricdata.ResourceMetrics, name string, want map[string]string) (float64, bool) {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[float64]); ok {
				for _, p := range g.DataPoints {
					if attrsMatch(p.Attributes, want) {
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
}

func sumValue(t *testing.T, rm metricdata.ResourceMetrics, name string, want map[string]string) (int64, bool) {
	t.Helper()
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

// allowAllLimiter stands in for a cluster store; the local budget is never
// charged when one is configured.
type allowAllLimiter struct{}

func (allowAllLimiter) Allow(context.Context, string, ratelimit.Spec, int) (bool, error) {
	return true, nil
}

// TestExecutorPublishesState verifies the executor reports its OWN breaker and
// rate-budget state, by pull, from the resilience package.
func TestExecutorPublishesState(t *testing.T) {
	rdr, restore := installStateMeter(t)
	defer restore()

	d := NewDefaultDriver(nil)
	exec, err := d.NewExecutor("svc-state-metrics", Policy{
		ErrorThreshold: 1,
		OpenDuration:   time.Hour,
		RateLimit:      1,
		Burst:          10,
	})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	defer func() { _ = exec.Close() }()

	boom := errutil.Explain(nil, "boom")
	_ = exec.Execute(context.Background(), chain.Operation{}, nil,
		func(context.Context) error { return boom })

	rm := collect(t, rdr)
	base := map[string]string{"service": "svc-state-metrics", "unit": "svc-state-metrics"}

	open := map[string]string{"service": "svc-state-metrics", "unit": "svc-state-metrics", "state": "open"}
	if v, ok := gaugeValue(t, rm, "resilience.client.breaker.state", open); !ok || v != 1 {
		t.Fatalf("breaker state open = %v, ok = %v", v, ok)
	}
	closed := map[string]string{"service": "svc-state-metrics", "unit": "svc-state-metrics", "state": "closed"}
	if v, ok := gaugeValue(t, rm, "resilience.client.breaker.state", closed); !ok || v != 0 {
		t.Fatalf("breaker state closed = %v, ok = %v", v, ok)
	}
	if trips, ok := sumValue(t, rm, "resilience.client.breaker.transitions_total",
		map[string]string{"service": "svc-state-metrics", "unit": "svc-state-metrics", "to": "open"}); !ok || trips != 1 {
		t.Fatalf("transitions to open = %v, ok = %v", trips, ok)
	}
	if usage, ok := gaugeValue(t, rm, "resilience.client.ratelimit.usage_ratio", base); !ok || usage <= 0 {
		t.Fatalf("rate usage = %v, ok = %v", usage, ok)
	}
}

// TestClusterLimiterSkipsUsage pins the documented omission: when a store is
// charged, the untouched local budget must not be published as a permanent zero.
func TestClusterLimiterSkipsUsage(t *testing.T) {
	rdr, restore := installStateMeter(t)
	defer restore()

	d := NewDefaultDriver(allowAllLimiter{})
	exec, err := d.NewExecutor("svc-cluster", Policy{RateLimit: 1, Burst: 10})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	defer func() { _ = exec.Close() }()

	_ = exec.Execute(context.Background(), chain.Operation{}, nil,
		func(context.Context) error { return nil })

	rm := collect(t, rdr)
	if _, ok := gaugeValue(t, rm, "resilience.client.ratelimit.usage_ratio",
		map[string]string{"service": "svc-cluster", "unit": "svc-cluster"}); ok {
		t.Fatal("published local rate usage while a cluster limiter is configured")
	}
}

// TestCloseStopsPublishing verifies Close unregisters the callback: the state
// plane belongs to the executor's lifetime.
func TestCloseStopsPublishing(t *testing.T) {
	rdr, restore := installStateMeter(t)
	defer restore()

	d := NewDefaultDriver(nil)
	exec, err := d.NewExecutor("svc-close", Policy{ErrorThreshold: 1, OpenDuration: time.Hour})
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}

	boom := errutil.Explain(nil, "boom")
	_ = exec.Execute(context.Background(), chain.Operation{}, nil,
		func(context.Context) error { return boom })
	if _, ok := gaugeValue(t, collect(t, rdr), "resilience.client.breaker.state",
		map[string]string{"service": "svc-close", "unit": "svc-close", "state": "open"}); !ok {
		t.Fatal("state was not published before Close")
	}

	if err := exec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, ok := gaugeValue(t, collect(t, rdr), "resilience.client.breaker.state",
		map[string]string{"service": "svc-close", "unit": "svc-close", "state": "open"}); ok {
		t.Fatal("state was still published after Close")
	}
}
