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

package StarterGovernanceSentinel

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
)

// TestSentinelBreakerMetrics verifies the driver publishes the mirror it keeps
// as the client-side breaker family: a fleet sees the same series whichever
// driver is configured.
func TestSentinelBreakerMetrics(t *testing.T) {
	prev := otel.GetMeterProvider()
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))
	otel.SetMeterProvider(mp)
	defer func() {
		otel.SetMeterProvider(prev)
		_ = mp.Shutdown(context.Background())
	}()

	e := newExec(t, "svc-metrics", resilience.Policy{ErrorThreshold: 1, OpenDuration: time.Hour})
	defer func() { _ = e.Close() }()

	_ = e.Execute(context.Background(), chain.Operation{}, nil,
		func(context.Context) error { return errors.New("boom") })

	var rm metricdata.ResourceMetrics
	if err := rdr.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	if v, ok := sentinelGauge(rm, "resilience.client.breaker.state", map[string]string{
		"service": "svc-metrics", "state": "open",
	}); !ok || v != 1 {
		t.Fatalf("breaker state open = %v, ok = %v", v, ok)
	}
	if trips, ok := sentinelSum(rm, "resilience.client.breaker.transitions_total", map[string]string{
		"service": "svc-metrics", "to": "open",
	}); !ok || trips != 1 {
		t.Fatalf("transitions to open = %v, ok = %v", trips, ok)
	}
}

func sentinelAttrsMatch(set attribute.Set, want map[string]string) bool {
	for k, v := range want {
		got, ok := set.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			return false
		}
	}
	return true
}

func sentinelGauge(rm metricdata.ResourceMetrics, name string, want map[string]string) (float64, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[float64]); ok {
				for _, p := range g.DataPoints {
					if sentinelAttrsMatch(p.Attributes, want) {
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
}

func sentinelSum(rm metricdata.ResourceMetrics, name string, want map[string]string) (int64, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if s, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, p := range s.DataPoints {
					if sentinelAttrsMatch(p.Attributes, want) {
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
}
