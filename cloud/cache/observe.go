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

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Observability contract.
//
// The decorator observes the [ByteCache] it wraps, so the unit of observation is
// the byte layer: a get that finds the bytes is a hit even when a codec above
// this layer then fails to decode them. Encoding and decoding are a different
// concern and are not reported here.
//
// Operation and status attribute values. The statuses are exclusive per
// operation, so cache.operation.total summed over status is the number of
// operations executed — there is no separate counter that could double-count.
// A get distinguishes hit from miss ([ErrMiss]); set and delete have no miss
// outcome, so theirs is ok/error. A miss is a normal outcome, not a failure: it
// drives the hit rate, not an error rate.
const (
	opGet    = "get"
	opSet    = "set"
	opDelete = "delete"

	statusHit   = "hit"
	statusMiss  = "miss"
	statusOK    = "ok"
	statusError = "error"
)

// componentName is the instrumentation componentName name every meter and tracer in this
// package reports under.
const componentName = "go-spring.org/cloud/cache"

// Observer receives the cache layer's observability events. It is the seam: the
// decorator reports what happened in the cache's own vocabulary, and an
// implementation decides how to record it — the default is OTel (see
// [NewOTelObserver]), and an application may replace it with its own bean.
//
// A nil Observer disables observation. Implementations must not block: events
// are emitted synchronously from the cache call path.
type Observer interface {
	// Operation records one finished cache operation. operation is get|set|delete
	// and status is hit|miss|ok|error; elapsed is the caller-visible duration.
	Operation(ctx context.Context, operation, status string, elapsed time.Duration)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean, so an
// application that contributes its own Observer takes precedence.
//
// The cache owns the instruments, so a failed registration fails construction
// instead of silently dropping the whole metric family.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.total, err = m.Int64Counter("cache.operation.total",
		metric.WithDescription("Cache operations executed, by operation and status"),
		metric.WithUnit("{operation}"))
	if err != nil {
		return nil, errutil.Explain(err, "cache: register cache.operation.total")
	}
	o.duration, err = m.Float64Histogram("cache.operation.duration",
		metric.WithDescription("Duration of cache operations, by operation and status"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "cache: register cache.operation.duration")
	}
	return o, nil
}

// otelObserver adapts the layer's events to the OTel instruments. It holds no
// per-cache state — the operation and status labels travel with each record.
type otelObserver struct {
	total    metric.Int64Counter
	duration metric.Float64Histogram
}

// Operation implements [Observer].
func (o *otelObserver) Operation(ctx context.Context, operation, status string, elapsed time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
	)
	o.total.Add(ctx, 1, attrs)
	o.duration.Record(ctx, elapsed.Seconds(), attrs)
}

// observedCache is the [ByteCache] decorator [New] wraps every backend in. It
// records the cache semantics — hit vs miss vs error — that no backend's own
// instrumentation can see: a redis GET that returns nil is a successful command
// down there, and only this layer knows it was a miss.
//
// The metric's labels and the attributes this layer contributes to the call's
// span share one vocabulary:
//
//	operation  get | set | delete
//	status     hit | miss | ok | error
//
// It opens no span of its own: the span belongs to the emitter the call ends up
// under (the backend starter's executor), and this layer contributes to it
// through the framework's carrier. See [observedCache.observe].
//
// No logs: cache calls are high-frequency, so a per-call line would be noise.
//
// Two invariants:
//   - The key is a caller-chosen resource name. It belongs on the call's span,
//     never on a metric, where its cardinality is unbounded.
//   - The duration is the caller-visible one — the backend round trip — and it
//     is the reason the histogram is here: the backend's own client metric
//     cannot split hit from miss, and those two have different latency profiles.
//
// The observer is resolved when the cache is constructed (see [New]): cache
// calls are too frequent for a per-call lookup. Under the framework
// construction happens after RefreshPrepare, where starter-otel installs the
// provider. A nil observer still wraps the backend — the span-attribute carrier
// below is not an observability-backend concern — it only skips the metric
// event.
type observedCache struct {
	ByteCache
	obs Observer
}

// newObservedCache wraps bc. obs may be nil, which disables the metric event
// while keeping the span-attribute contribution.
func newObservedCache(bc ByteCache, obs Observer) observedCache {
	return observedCache{ByteCache: bc, obs: obs}
}

func (o observedCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	var b []byte
	err := o.observe(ctx, opGet, key, func(ctx context.Context) error {
		var err error
		b, err = o.ByteCache.GetBytes(ctx, key)
		return err
	})
	return b, err
}

func (o observedCache) SetBytes(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	return o.observe(ctx, opSet, key, func(ctx context.Context) error {
		return o.ByteCache.SetBytes(ctx, key, val, ttlSeconds)
	})
}

func (o observedCache) Delete(ctx context.Context, key string) error {
	return o.observe(ctx, opDelete, key, func(ctx context.Context) error {
		return o.ByteCache.Delete(ctx, key)
	})
}

// observe runs fn and reports the outcome.
//
// The operation and the key are put on the framework's span-attribute carrier
// rather than on a span opened here: this layer has no business opening one, and
// the emitter that owns the span sits BELOW it. The carrier is what closes that
// gap — it rides the context down, so the span the emitter starts on the way
// picks the attributes up as it opens.
//
// The status is not among them. A miss is only known once fn returns, and by
// then the span is over; it stays a metric dimension, which is where it drives
// the hit rate. On the span the outcome is the emitter's own ok/error.
//
// fn's error is the outcome that gets reported, and it is returned unchanged.
func (o observedCache) observe(ctx context.Context, op, key string, fn func(ctx context.Context) error) error {
	ctx = attrs.WithSpanAttributes(ctx,
		attribute.String("cache.operation", op),
		attribute.String("cache.key", key))

	start := time.Now()
	err := fn(ctx)
	o.record(ctx, op, err, time.Since(start))
	return err
}

// statusOf maps an outcome to the status dimension the metric records. ErrMiss
// is a status of its own: a miss is neither a success nor a backend failure, and
// folding it into either would hide the hit rate this decorator exists to
// report.
func statusOf(op string, err error) string {
	switch {
	case err == nil && op == opGet:
		return statusHit
	case err == nil:
		return statusOK
	case op == opGet && errors.Is(err, ErrMiss):
		return statusMiss
	default:
		return statusError
	}
}

// record reports one finished operation with the status its outcome maps to.
// elapsed is what the caller measured: the window belongs to whoever knows
// where the operation began.
func (o observedCache) record(ctx context.Context, op string, err error, elapsed time.Duration) {
	if o.obs == nil {
		return
	}
	o.obs.Operation(ctx, op, statusOf(op, err), elapsed)
}
