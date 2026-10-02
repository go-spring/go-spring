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

// observe.go is what this starter's observability IS: the per-call signals it
// emits itself — a span and two metrics — plus the cache-statistics gauges, the
// one signal that is not per-call.
//
// bigcache emits its own signals rather than declaring an operation for the
// framework's emitter: this is an in-process cache with no external dependency,
// so neither the protection stages nor the access log an executor chain would
// produce have anything to act on. The span still rides the framework's
// span-attribute carrier — a span this file opens picks up whatever a layer
// above put there, with no cooperation from either side (see
// [observability.SpanAttributesProcessor]).
package StarterBigCache

import (
	"context"
	"errors"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/singleton"
	"go-spring.org/stdlib/strutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// scope is the instrumentation scope name every meter and tracer in this package
// reports under.
const scope = "go-spring.org/starter-bigcache"

// maxKeyAttr bounds the key captured as the span's bigcache.key attribute. A key
// can be long, and a span attribute has no use for all of it.
const maxKeyAttr = 128

// The operation names and the status axis the per-call metric carries.
const (
	opGet    = "get"
	opSet    = "set"
	opDelete = "delete"

	statusOK    = "ok"
	statusError = "error"
)

// statSnapshot is one reading of a cache's statistics. Every gauge of a
// collection reads from the same snapshot, so two gauges can never describe
// different instants — and the five that come from Stats() do not each rescan
// every shard, which at the default 1024 shards is the difference between one
// pass and five.
type statSnapshot struct {
	stats bigcache.Stats
	len   int
	cap   int
}

// snapshot reads each source once. Stats() walks every shard under its read
// lock, so it is the reason the snapshot exists.
func snapshot(c *bigcache.BigCache) statSnapshot {
	return statSnapshot{stats: c.Stats(), len: c.Len(), cap: c.Capacity()}
}

// statInstrument describes one gauge to register.
type statInstrument struct {
	name string
	desc string
	read func(statSnapshot) int64
}

// statInstruments lists the bigcache statistics surfaced as gauges. The five
// counters come from Stats() (cumulative); entries/capacity come from Len()/
// Capacity() (current). All are gauges rather than counters because
// [bigcache.BigCache.ResetStats] can reset the counters, breaking monotonicity.
//
// The five counters are only recorded when the instance sets stats-enabled;
// with it off bigcache keeps them at zero, and the gauges export that zero
// faithfully rather than hiding the series.
var statInstruments = []statInstrument{
	{
		name: "bigcache.hits",
		desc: "Number of successfully found keys",
		read: func(s statSnapshot) int64 { return s.stats.Hits },
	},
	{
		name: "bigcache.misses",
		desc: "Number of not found keys",
		read: func(s statSnapshot) int64 { return s.stats.Misses },
	},
	{
		name: "bigcache.delete_hits",
		desc: "Number of successfully deleted keys",
		read: func(s statSnapshot) int64 { return s.stats.DelHits },
	},
	{
		name: "bigcache.delete_misses",
		desc: "Number of not deleted keys",
		read: func(s statSnapshot) int64 { return s.stats.DelMisses },
	},
	{
		name: "bigcache.collisions",
		desc: "Number of key hash collisions",
		read: func(s statSnapshot) int64 { return s.stats.Collisions },
	},
	{
		name: "bigcache.entries",
		desc: "Current number of stored entries",
		read: func(s statSnapshot) int64 { return int64(s.len) },
	},
	{
		name: "bigcache.capacity",
		desc: "Bytes allocated for the entries queues, summed over shards",
		read: func(s statSnapshot) int64 { return int64(s.cap) },
	},
}

// instrumentSet holds this starter's instruments: the per-call counter and
// duration histogram, plus the cache-statistics gauges. It is one per process and
// carries no per-instance state — the labels travel with each record, and with
// each cache's registration of the gauges.
//
// It rides the OTel globals: when starter-otel is not imported the global
// MeterProvider is a no-op, so this adds negligible overhead.
type instrumentSet struct {
	meter    metric.Meter
	gauges   []metric.Int64ObservableGauge
	total    metric.Int64Counter
	duration metric.Float64Histogram
}

// instruments is the one instrument set this starter uses for the whole process.
// Resolution is deferred to the first use rather than run at package init, so the
// instruments are built against whichever MeterProvider is installed when the
// first cache is constructed - starter-otel installs its own before any bean is
// built. A test that swaps the provider starts from a fresh set instead; see
// resetInstruments in observe_test.go.
//
// The gauges are created WITHOUT a callback on purpose; see observeGauges for
// why a creation-time callback would report only the first cache in a process.
var instruments singleton.Singleton[*instrumentSet]

// buildInstruments resolves the process-wide set, or the error that stopped it.
// Every name, unit and description here is a compile-time constant, so the SDK
// cannot reject one; the error exists for a custom MeterProvider, whose refusal
// belongs to the application's startup error — not a panic out of a library, and
// not a silent nil instrument that would panic at the first recorded call.
//
// The error names the instrument the meter refused, so a startup failure points
// at the stage that broke instead of the meter in general. The singleton caches
// that named error with the set, so a failure is reported to every later caller
// rather than retried behind the application's back.
func buildInstruments() (*instrumentSet, error) {
	return instruments.Init(func() (*instrumentSet, error) {
		m := otel.Meter(scope)
		in := &instrumentSet{meter: m}

		in.gauges = make([]metric.Int64ObservableGauge, len(statInstruments))
		for i, inst := range statInstruments {
			g, err := m.Int64ObservableGauge(inst.name, metric.WithDescription(inst.desc))
			if err != nil {
				return nil, errutil.Explain(err, "bigcache: create gauge %q", inst.name)
			}
			in.gauges[i] = g
		}

		total, err := m.Int64Counter("bigcache.operation.total",
			metric.WithDescription("Cache operations executed, by operation and status"),
			metric.WithUnit("{operation}"))
		if err != nil {
			return nil, errutil.Explain(err, "bigcache: create counter %q", "bigcache.operation.total")
		}
		in.total = total

		duration, err := m.Float64Histogram("bigcache.operation.duration",
			metric.WithDescription("Duration of cache operations, by operation and status"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
		if err != nil {
			return nil, errutil.Explain(err, "bigcache: create histogram %q", "bigcache.operation.duration")
		}
		in.duration = duration
		return in, nil
	})
}

// statObserver is one cache's observability: its instance identity, its
// registration of the shared gauges, and the one place its calls become signals.
// One per [Cache], built with it — which is what keeps the instance's name and
// its registration from being threaded through every call.
type statObserver struct {
	in   *instrumentSet
	name string

	// reg is this cache's own registration of its statistics against the shared
	// gauges. Its lifetime is the cache's — [statObserver.close] takes it away —
	// because the values those gauges report come from this cache. Dropping it
	// would leave the instruments reporting a destroyed cache, and never taking it
	// away would pin the cache.
	reg metric.Registration
}

// newStatObserver builds the observer for the cache over client, named name, and
// registers that cache's statistics against the shared gauges.
//
// The two failures are treated differently, because they mean different things. A
// set the meter will not build is process-wide and permanent - every cache in the
// process is affected and nothing can be reported at all - so it comes back as an
// error for the application's startup to answer. A registration that fails is one
// cache's, and only means the provider is already going away, which is not a
// reason to refuse to serve.
func newStatObserver(client *bigcache.BigCache, name string) (*statObserver, error) {
	in, err := buildInstruments()
	if err != nil {
		return nil, err
	}
	o := &statObserver{in: in, name: name}
	if reg, err := o.observeGauges(client); err == nil {
		o.reg = reg
	}
	return o, nil
}

// observeGauges registers c's statistics as this cache's own observations of the
// shared gauges, labeled with this observer's name so several caches in one
// process stay distinguishable, and returns the registration the observer holds
// until the cache is destroyed.
//
// The instruments are registered here rather than given a creation-time callback
// (metric.WithInt64Callback): the SDK keys an observable instrument by
// name/description/unit/kind and returns the first instrument on every later
// creation, silently dropping the new callbacks. A process with more than one
// cache would then report only the first one, with no error to show for it.
func (o *statObserver) observeGauges(c *bigcache.BigCache) (metric.Registration, error) {
	attrs := metric.WithAttributes(attribute.String("cache.name", o.name))
	insts := make([]metric.Observable, len(o.in.gauges))
	for i, g := range o.in.gauges {
		insts[i] = g
	}
	return o.in.meter.RegisterCallback(func(_ context.Context, ob metric.Observer) error {
		snap := snapshot(c)
		for i, inst := range statInstruments {
			ob.ObserveInt64(o.in.gauges[i], inst.read(snap), attrs)
		}
		return nil
	}, insts...)
}

// close takes away this cache's gauge registration. It comes first on the way
// down: once the cache is closed its statistics are meaningless, and a
// registration left behind would both report a dead cache and pin it.
func (o *statObserver) close() {
	if o.reg != nil {
		_ = o.reg.Unregister()
	}
	o.reg = nil
}

// statusOf maps an outcome to the metric's status axis. A miss is not a failure:
// the cache answered, the key was simply absent, and the hit rate is the gauges'
// to report — folding it in here would hide the distinction.
func statusOf(err error) string {
	if err != nil && !errors.Is(err, bigcache.ErrEntryNotFound) {
		return statusError
	}
	return statusOK
}

// observe runs one cache operation: it opens the operation's span, runs fn under
// it, and records the outcome under the status it maps to. It is the one place a
// bigcache call becomes a signal, and the instance's name rides every one of
// them, so several caches in a process stay distinguishable.
func (o *statObserver) observe(ctx context.Context, op, key string, fn func(context.Context) error) error {
	// Internal, not client: the dependency is in this process, so there is no edge
	// to add to the trace topology. The key is an attribute and never a metric
	// label — cache keys are drawn from an open set, and one as a label would
	// multiply the series without bound.
	ctx, span := otel.Tracer(scope).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("bigcache.operation", op),
			attribute.String("bigcache.key", strutil.Truncate(key, maxKeyAttr)),
		))

	start := time.Now()
	err := fn(ctx)
	status := statusOf(err)

	span.SetAttributes(attribute.String("status", status))
	if status == statusError {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	attrs := metric.WithAttributes(
		attribute.String("operation", op),
		attribute.String("status", status),
		attribute.String("cache.name", o.name),
	)
	o.in.total.Add(ctx, 1, attrs)
	o.in.duration.Record(ctx, time.Since(start).Seconds(), attrs)
	return err
}
