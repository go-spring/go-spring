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

package cache

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// sentinelErr is the backend failure the scripted cache reports.
var sentinelErr = errors.New("boom")

// withReader installs a manual reader as the global meter provider for the
// test; since the package's shared instrument set is resolved on first use,
// call withReader before the first cache operation.
func withReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	rdr := sdkmetric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr)))
	// The instrument set is process-wide and resolved once, so without this it
	// would keep reporting into whichever provider an earlier use (or test)
	// resolved it against.
	resetInstruments()
	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return rdr
}

// withSpanRecorder installs a recording tracer provider as the global one for
// the test; the decorator resolves the tracer per call, so no reset is needed.
func withSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

// attrMap flattens a span's attributes for assertion.
func attrMap(sp sdktrace.ReadOnlySpan) map[string]string {
	out := map[string]string{}
	for _, kv := range sp.Attributes() {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}

// totals collects cache.operation.total into a "operation.status" -> count map.
func totals(t *testing.T, rdr *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "cache.operation.total" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				out[labels(dp.Attributes)] = dp.Value
			}
		}
	}
	return out
}

// durations collects cache.operation.duration into a "operation.status" -> count
// map, so a caller asserts which outcomes were timed, not their values.
func durations(t *testing.T, rdr *sdkmetric.ManualReader) map[string]uint64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]uint64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "cache.operation.duration" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Histogram[float64]).DataPoints {
				out[labels(dp.Attributes)] = dp.Count
			}
		}
	}
	return out
}

// labels renders a metric data point's operation and status attributes.
func labels(attrs attribute.Set) string {
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

// stubCache is a ByteCache whose outcomes are scripted per test.
type stubCache struct {
	get func(ctx context.Context, key string) ([]byte, error)
	set func(ctx context.Context, key string, val []byte, ttlSeconds int) error
	del func(ctx context.Context, key string) error
}

func (s stubCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return s.get(ctx, key)
}

func (s stubCache) SetBytes(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	return s.set(ctx, key, val, ttlSeconds)
}

func (s stubCache) Delete(ctx context.Context, key string) error {
	return s.del(ctx, key)
}

// scriptedCache is a ByteCache covering every status of the vocabulary: a get
// that hits, one that misses, one that fails, a failing set and a successful
// delete.
func scriptedCache() stubCache {
	return stubCache{
		get: func(_ context.Context, key string) ([]byte, error) {
			switch key {
			case "hit":
				return []byte("v"), nil
			case "miss":
				return nil, ErrMiss
			default:
				return nil, sentinelErr
			}
		},
		set: func(context.Context, string, []byte, int) error { return sentinelErr },
		del: func(context.Context, string) error { return nil },
	}
}

// exerciseStatuses runs scriptedCache's five outcomes. Every signal is asserted
// over the same set, so the tests below cannot drift apart.
func exerciseStatuses(t *testing.T, c *Cache) {
	t.Helper()
	ctx := context.Background()

	b, err := c.GetBytes(ctx, "hit")
	assert.That(t, err).Nil()
	assert.That(t, string(b)).Equal("v")

	_, err = c.GetBytes(ctx, "miss")
	assert.That(t, errors.Is(err, ErrMiss)).True()
	_, err = c.GetBytes(ctx, "fail")
	assert.That(t, errors.Is(err, sentinelErr)).True()
	err = c.SetBytes(ctx, "k", []byte("v"), 0) // errors pass through unchanged
	assert.That(t, errors.Is(err, sentinelErr)).True()
	assert.That(t, c.Delete(ctx, "k")).Nil()
}

func TestObservabilityStatuses(t *testing.T) {
	rdr := withReader(t)
	exerciseStatuses(t, New(scriptedCache()))

	// Statuses are exclusive: hit/miss/error on get, ok/error on set, ok on
	// delete; summed over status they equal the operations executed.
	got := totals(t, rdr)
	assert.That(t, got["get.hit"]).Equal(int64(1))
	assert.That(t, got["get.miss"]).Equal(int64(1))
	assert.That(t, got["get.error"]).Equal(int64(1))
	assert.That(t, got["set.error"]).Equal(int64(1))
	assert.That(t, got["delete.ok"]).Equal(int64(1))
	assert.That(t, got["set.ok"]).Equal(int64(0))
}

func TestObservabilityDuration(t *testing.T) {
	rdr := withReader(t)
	exerciseStatuses(t, New(scriptedCache()))

	// Every operation is timed, broken down by the same status the counter uses
	// — the split the backend's own client metric cannot produce.
	got := durations(t, rdr)
	assert.That(t, got["get.hit"]).Equal(uint64(1))
	assert.That(t, got["get.miss"]).Equal(uint64(1))
	assert.That(t, got["get.error"]).Equal(uint64(1))
	assert.That(t, got["set.error"]).Equal(uint64(1))
	assert.That(t, got["delete.ok"]).Equal(uint64(1))
}

func TestObservabilitySpan(t *testing.T) {
	sr := withSpanRecorder(t)
	exerciseStatuses(t, New(scriptedCache()))

	spans := sr.Ended()
	assert.That(t, len(spans)).Equal(5)

	// The first operation was the get that hit: one client span carrying the
	// operation, the key, and the status — the same status the metric recorded.
	sp := spans[0]
	assert.String(t, sp.Name()).Equal("get")
	assert.That(t, sp.SpanKind()).Equal(trace.SpanKindClient)
	attrs := attrMap(sp)
	assert.String(t, attrs["cache.operation"]).Equal("get")
	assert.String(t, attrs["cache.key"]).Equal("hit")
	assert.String(t, attrs["cache.status"]).Equal("hit")

	// A miss is a normal outcome, a backend failure is not: only the latter
	// marks the span.
	for _, s := range spans {
		status := attrMap(s)["cache.status"]
		wantErr := status == "error"
		assert.That(t, s.Status().Code == codes.Error).Equal(wantErr)
	}
}
