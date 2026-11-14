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

package observe

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// withMeter installs a manual reader as the global meter provider and drops the
// memoized instrument set, so the signals a test drives resolve against this
// provider rather than one an earlier test installed.
func withMeter(t *testing.T) *metric.ManualReader {
	t.Helper()
	rdr := metric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(metric.NewMeterProvider(metric.WithReader(rdr)))
	ResetInstruments()
	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return rdr
}

// collect reads every metric the provider holds, keyed by name.
func collect(t *testing.T, rdr *metric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = m
		}
	}
	return out
}

// histAttrs returns every histogram datapoint's attributes as a key→value map.
func histAttrs(t *testing.T, m metricdata.Metrics) []map[string]string {
	t.Helper()
	h, ok := m.Data.(metricdata.Histogram[float64])
	assert.That(t, ok).True()
	out := make([]map[string]string, 0, len(h.DataPoints))
	for _, dp := range h.DataPoints {
		kv := map[string]string{}
		for _, a := range dp.Attributes.ToSlice() {
			kv[string(a.Key)] = a.Value.Emit()
		}
		out = append(out, kv)
	}
	return out
}

// TestDeclaredServerOperationEmitsFamilySignals proves an inbound route that
// declares its operation is served like a client is: its family metric under the
// FAMILY's name (http.server.request.duration, not the client side's
// .operation.duration) labelled by the route's bounded attributes, plus the one
// thing only a server has — a response half the handler records after answering.
func TestDeclaredServerOperationEmitsFamilySignals(t *testing.T) {
	rdr := withMeter(t)
	m, err := NewOTelMetrics()
	if err != nil {
		t.Fatal(err)
	}
	exec := NewServerObserver("http-server", "http-server::9090", m)

	declared := ServerOperation{
		Action: "GET /users",
		Observe: ServerObserve{
			Metric: ServerMetric{
				Name: "http.server",
				Attrs: []attribute.KeyValue{
					attribute.String("http.request.method", "GET"),
					attribute.String("http.route", "/users"),
				},
			},
		},
	}
	err = exec.Run(context.Background(), declared, func(ctx context.Context, ob *Observation) error {
		ob.Response().Add(attribute.Int("http.response.status_code", 200))
		return nil
	})
	assert.Error(t, err).Nil()

	got := collect(t, rdr)
	fam, ok := got["http.server.request.duration"]
	assert.That(t, ok).True("the family name must be kept, not renamed to .operation.duration")
	pts := histAttrs(t, fam)
	assert.Number(t, len(pts)).Equal(1)
	assert.String(t, pts[0]["http.request.method"]).Equal("GET")
	assert.String(t, pts[0]["http.route"]).Equal("/users")
	// The response half reaches the metric because the metric is recorded after
	// the handler answered — which is exactly why it is a carrier and not part of
	// the declaration.
	assert.String(t, pts[0]["http.response.status_code"]).Equal("200")
	assert.String(t, pts[0]["status"]).Equal("ok")

	_, ok = got["http.server.active_requests"]
	assert.That(t, ok).True("the in-flight gauge must be emitted for a declared route")
}
