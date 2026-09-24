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
	"time"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// withReader installs a manual reader as the global meter provider for the
// test; since New builds its instrument from the provider current at
// construction time, call withReader before New.
func withReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	rdr := sdkmetric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr)))
	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prev)
	})
	return rdr
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
				op, status := "", ""
				for _, kv := range dp.Attributes.ToSlice() {
					if kv.Key == "operation" {
						op = kv.Value.AsString()
					}
					if kv.Key == "status" {
						status = kv.Value.AsString()
					}
				}
				out[op+"."+status] = dp.Value
			}
		}
	}
	return out
}

// stubCache is a ByteCache whose outcomes are scripted per test.
type stubCache struct {
	get func(ctx context.Context, key string) ([]byte, error)
	set func(ctx context.Context, key string, val []byte, ttl time.Duration) error
	del func(ctx context.Context, key string) error
}

func (s stubCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return s.get(ctx, key)
}

func (s stubCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return s.set(ctx, key, val, ttl)
}

func (s stubCache) Delete(ctx context.Context, key string) error {
	return s.del(ctx, key)
}

func TestObservabilityStatuses(t *testing.T) {
	rdr := withReader(t)
	ctx := context.Background()

	// A get that hits, one that misses ([ErrMiss]), and one that fails.
	sentinel := errors.New("boom")
	c := New(stubCache{
		get: func(_ context.Context, key string) ([]byte, error) {
			switch key {
			case "hit":
				return []byte("v"), nil
			case "miss":
				return nil, ErrMiss
			default:
				return nil, sentinel
			}
		},
		set: func(context.Context, string, []byte, time.Duration) error { return sentinel },
		del: func(context.Context, string) error { return nil },
	})

	b, err := c.GetBytes(ctx, "hit")
	assert.That(t, err).Nil()
	assert.That(t, string(b)).Equal("v")

	_, err = c.GetBytes(ctx, "miss")
	assert.That(t, errors.Is(err, ErrMiss)).True()
	_, err = c.GetBytes(ctx, "fail")
	assert.That(t, errors.Is(err, sentinel)).True()
	err = c.SetBytes(ctx, "k", []byte("v"), 0) // errors pass through unchanged
	assert.That(t, errors.Is(err, sentinel)).True()
	assert.That(t, c.Delete(ctx, "k")).Nil()

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
