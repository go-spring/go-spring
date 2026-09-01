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

package lock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	observe "go-spring.org/cloud/observe"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeLock is a minimal held-lock handle for the fake locker.
type fakeLock struct{ key string }

func (f fakeLock) Key() string                      { return f.key }
func (f fakeLock) Token() string                    { return "token" }
func (f fakeLock) Unlock(ctx context.Context) error { return nil }
func (f fakeLock) Lost() <-chan struct{}            { return nil }

// fakeLocker scripts Acquire/TryAcquire results so a test can drive the
// success and miss paths without a real backend.
type fakeLocker struct {
	tryOK bool
}

func (f fakeLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	return fakeLock{key}, nil
}

func (f fakeLocker) TryAcquire(ctx context.Context, key string, opts ...Option) (Lock, bool, error) {
	if !f.tryOK {
		return nil, false, nil
	}
	return fakeLock{key}, true, nil
}

func (fakeLocker) Close() error { return nil }

// installGlobals wires in-memory OTel providers (same harness the observe kit's
// own tests use) so a test can assert spans and metrics actually fire.
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

// attrValue reads one attribute off a span stub or metric datapoint.
func attrValue(attrs []attribute.KeyValue, key attribute.Key) (attribute.Value, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// histPoints collects the duration histogram datapoints for the named metric.
func histPoints(t *testing.T, rdr sdkmetric.Reader, name string) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
				return h.DataPoints
			}
		}
	}
	return nil
}

// TestWrapLocker_AcquireSuccess asserts the Acquire path: a client span named
// "acquire" carrying system + key, and a operation.duration
// metric datapoint tagged status=ok.
func TestWrapLocker_AcquireSuccess(t *testing.T) {
	spanExp, rdr, cleanup := installGlobals(t)
	defer cleanup()

	l := WrapLocker("redis", observe.ObserveConfig{Level: observe.DefaultBrief}, fakeLocker{})
	held, err := l.Acquire(context.Background(), "jobs:1")
	require.NoError(t, err)
	assert.Equal(t, "jobs:1", held.Key())

	spans := spanExp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "acquire", spans[0].Name)
	assert.Equal(t, codes.Unset, spans[0].Status.Code)

	system, ok := attrValue(spans[0].Attributes, "lock.system")
	require.True(t, ok)
	assert.Equal(t, "redis", system.AsString())
	key, ok := attrValue(spans[0].Attributes, "lock.key")
	require.True(t, ok)
	assert.Equal(t, "jobs:1", key.AsString())

	points := histPoints(t, rdr, "lock.operation.duration")
	require.Len(t, points, 1)
	status, ok := attrValue(points[0].Attributes.ToSlice(), "status")
	require.True(t, ok)
	assert.Equal(t, "ok", status.AsString())
}

// TestWrapLocker_TryAcquireMiss asserts the miss path: a span named
// "try_acquire" whose duration datapoint carries acquired=false, so a
// dashboard can separate misses from wins without parsing errors.
func TestWrapLocker_TryAcquireMiss(t *testing.T) {
	spanExp, rdr, cleanup := installGlobals(t)
	defer cleanup()

	l := WrapLocker("etcd", observe.ObserveConfig{Level: observe.DefaultBrief}, fakeLocker{tryOK: false})
	_, ok, err := l.TryAcquire(context.Background(), "leader")
	require.NoError(t, err)
	assert.False(t, ok)

	spans := spanExp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "try_acquire", spans[0].Name)
	assert.Equal(t, codes.Unset, spans[0].Status.Code)

	points := histPoints(t, rdr, "lock.operation.duration")
	require.Len(t, points, 1)
	acquired, ok := attrValue(points[0].Attributes.ToSlice(), "lock.acquired")
	require.True(t, ok)
	assert.False(t, acquired.AsBool())
}
