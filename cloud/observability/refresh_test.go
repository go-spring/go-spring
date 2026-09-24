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

package observability

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// withReader installs a manual reader as the global meter provider for the
// test; since RefreshConf builds its instruments per call, they always bind
// to the provider that is current at call time.
func withReader(t *testing.T) *metric.ManualReader {
	t.Helper()
	rdr := metric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(metric.NewMeterProvider(metric.WithReader(rdr)))
	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return rdr
}

// refreshTotals collects config.refresh.total into a status -> count map.
func refreshTotals(t *testing.T, rdr *metric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "config.refresh.total" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				for _, kv := range dp.Attributes.ToSlice() {
					if kv.Key == "status" {
						out[kv.Value.AsString()] = dp.Value
					}
				}
			}
		}
	}
	return out
}

func TestRefreshConf(t *testing.T) {
	rdr := withReader(t)

	// Success path: nil passes through, fn runs exactly once, counted ok.
	calls := 0
	assert.That(t, RefreshConf(context.Background(), func(context.Context) error { calls++; return nil })).Nil()
	assert.That(t, calls).Equal(1)

	// Failure path: the error passes through unchanged and counts as error.
	sentinel := errors.New("boom")
	assert.That(t, errors.Is(RefreshConf(context.Background(), func(context.Context) error { return sentinel }), sentinel)).True()

	// One ok and one error: statuses are exclusive, sum equals refreshes run.
	got := refreshTotals(t, rdr)
	assert.That(t, got["ok"]).Equal(int64(1))
	assert.That(t, got["error"]).Equal(int64(1))
}
