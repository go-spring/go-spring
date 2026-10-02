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
	"sync"
	"time"

	"go-spring.org/cloud/observability"
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

// scope is the instrumentation scope name every meter and tracer in this
// package reports under.
const scope = "go-spring.org/cloud/cache"

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
// The instruments come from the package's process-wide set (see [instruments]),
// resolved on first use: cache calls are too frequent for a per-call meter
// lookup. This relies on [New] running after the OTel global provider is
// installed — under the framework it does, since bean construction happens after
// RefreshPrepare, where starter-otel installs the provider. A cache built before
// any provider is set records to the no-op meter.
type observedCache struct {
	ByteCache
}

// instrumentSet is this package's metric set: one per process, resolved lazily
// on first use so it binds to whichever meter provider is current then, and
// immutable afterwards. It holds no per-cache state — the operation and status
// labels travel with each record, not here.
type instrumentSet struct {
	total    metric.Int64Counter
	duration metric.Float64Histogram
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(scope)
	in := &instrumentSet{}
	in.total, _ = m.Int64Counter("cache.operation.total",
		metric.WithDescription("Cache operations executed, by operation and status"),
		metric.WithUnit("{operation}"))
	in.duration, _ = m.Float64Histogram("cache.operation.duration",
		metric.WithDescription("Duration of cache operations, by operation and status"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// newObservedCache wraps bc.
func newObservedCache(bc ByteCache) observedCache {
	return observedCache{ByteCache: bc}
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
	ctx = observability.WithSpanAttributes(ctx,
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

// record emits the counter and the duration for one finished operation, under
// the status its outcome maps to. elapsed is what the caller measured: the
// window belongs to whoever knows where the operation began.
func (o observedCache) record(ctx context.Context, op string, err error, elapsed time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("operation", op),
		attribute.String("status", statusOf(op, err)),
	)
	in := instruments()
	in.total.Add(ctx, 1, attrs)
	in.duration.Record(ctx, elapsed.Seconds(), attrs)
}
