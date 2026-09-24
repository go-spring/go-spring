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
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Operation and status attribute values. The statuses are exclusive per
// operation, so cache.operation.total summed over status is the number of
// operations executed — there is no separate counter that could double-count.
// A get distinguishes hit from miss ([ErrMiss]); set and delete have no miss
// outcome, so theirs is ok/error. The key never appears in a metric: it is
// unbounded and would explode the label space.
const (
	opGet    = "get"
	opSet    = "set"
	opDelete = "delete"

	statusHit   = "hit"
	statusMiss  = "miss"
	statusOK    = "ok"
	statusError = "error"
)

// observability is the [ByteCache] decorator [New] wraps every backend in.
// It records the cache semantics — hit vs miss vs error — that no backend's
// own instrumentation can see: a redis GET that returns nil is a successful
// command down there, and only this layer knows it was a miss.
//
// Only that counter is recorded. No duration: an operation's latency is
// the backend client's, not the cache abstraction's. No logs: cache calls
// are high-frequency, so a per-call line would be noise.
//
// The instrument is built once in [newObservability] and reused: cache
// calls are too frequent for a per-call meter lookup. This relies on [New]
// running after the OTel global provider is installed — under the framework
// it does, since bean construction happens after RefreshPrepare, where
// starter-otel installs the provider. A cache built before any provider is
// set records to the no-op meter.
type observability struct {
	ByteCache
	total metric.Int64Counter
}

// newObservability wraps bc and builds its instrument from the meter provider
// current at construction time.
func newObservability(bc ByteCache) observability {
	m := otel.Meter("go-spring.org/cloud/cache")
	total, _ := m.Int64Counter("cache.operation.total",
		metric.WithDescription("Cache operations executed, by operation and status"),
		metric.WithUnit("{operation}"))
	return observability{ByteCache: bc, total: total}
}

func (o observability) GetBytes(ctx context.Context, key string) ([]byte, error) {
	b, err := o.ByteCache.GetBytes(ctx, key)
	o.record(ctx, opGet, err)
	return b, err
}

func (o observability) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	err := o.ByteCache.SetBytes(ctx, key, val, ttl)
	o.record(ctx, opSet, err)
	return err
}

func (o observability) Delete(ctx context.Context, key string) error {
	err := o.ByteCache.Delete(ctx, key)
	o.record(ctx, opDelete, err)
	return err
}

// record reports one operation on cache.operation.total, deriving the status
// from the outcome: nil is hit (get) or ok (set/delete), any other error is
// error, and a get miss — [ErrMiss] — is miss.
func (o observability) record(ctx context.Context, op string, err error) {
	status := statusOK
	if op == opGet {
		status = statusHit
	}
	if err != nil {
		status = statusError
		if op == opGet && errors.Is(err, ErrMiss) {
			status = statusMiss
		}
	}
	o.total.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", op),
		attribute.String("status", status),
	))
}
