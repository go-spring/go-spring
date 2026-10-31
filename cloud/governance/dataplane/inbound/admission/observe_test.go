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

package admission

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"go-spring.org/cloud/governance/dataplane/inbound/chain"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

func intGaugeValue(t *testing.T, rm metricdata.ResourceMetrics, name string, want map[string]string) (int64, bool) {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[int64]); ok {
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

// TestGuardPublishesBulkheadAndRate verifies the guard's organ state: a held
// slot, the capacity, and the local budget's spent fraction.
func TestGuardPublishesBulkheadAndRate(t *testing.T) {
	rdr, restore := installStateMeter(t)
	defer restore()

	id := Identity{System: "sys", Service: "svc-all-metrics"}
	g, err := New(id, Policy{MaxConcurrent: 1, RateLimit: 1, Burst: 10})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = g.Close() }()

	r, done := g.Admit(ctx(), chain.Request{})
	if !r.Admitted() {
		t.Fatalf("request refused: %v", r.Err())
	}

	rm := collect(t, rdr)
	base := map[string]string{"system": "sys", "service": id.Service}
	if v, ok := gaugeValue(t, rm, "resilience.server.bulkhead.usage_ratio", base); !ok || v != 1 {
		t.Fatalf("bulkhead usage_ratio = %v, ok = %v", v, ok)
	}
	if v, ok := intGaugeValue(t, rm, "resilience.server.bulkhead.queued", base); !ok || v != 0 {
		t.Fatalf("bulkhead queued = %v, ok = %v", v, ok)
	}
	if usage, ok := gaugeValue(t, rm, "resilience.server.ratelimit.usage_ratio", base); !ok || usage <= 0 {
		t.Fatalf("rate usage = %v, ok = %v", usage, ok)
	}

	done()
}

// TestGuardCloseStopsPublishing verifies the guard's Close unregisters its
// state callback.
func TestGuardCloseStopsPublishing(t *testing.T) {
	rdr, restore := installStateMeter(t)
	defer restore()

	id := Identity{System: "sys", Service: "svc-close-metrics"}
	g, err := New(id, Policy{MaxConcurrent: 1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	r, _ := g.Admit(ctx(), chain.Request{}) // keep the slot held so occupancy is published
	if !r.Admitted() {
		t.Fatalf("request refused: %v", r.Err())
	}

	base := map[string]string{"system": "sys", "service": id.Service}
	if _, ok := gaugeValue(t, collect(t, rdr), "resilience.server.bulkhead.usage_ratio", base); !ok {
		t.Fatal("state was not published before Close")
	}

	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, ok := gaugeValue(t, collect(t, rdr), "resilience.server.bulkhead.usage_ratio", base); ok {
		t.Fatal("state was still published after Close")
	}
}
