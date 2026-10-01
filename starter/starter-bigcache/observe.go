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

// observe.go is the per-operation observability of this starter: a client
// span, the db.client.operation.duration metric, and an access log for each
// Get/Set/Delete, following the OTel db semantic conventions.
package StarterBigCache

import (
	"context"
	"sync"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/stdlib/strutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// accessTag is the static log tag of the bigcache access log (renders as
// "_app_bigcache_access").
var accessTag = log.RegisterAppTag("bigcache", "access")

// obsTracer names the tracer/meter this starter's instruments register under.
const obsTracer = "go-spring.org/starter-bigcache"

// bigcacheSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const bigcacheSystem = "bigcache"

// statReader reads one snapshot value from a BigCache instance.
type statReader func(*bigcache.BigCache) int64

// statInstrument describes one gauge to register.
type statInstrument struct {
	name string
	desc string
	read statReader
}

// statInstruments lists the bigcache statistics surfaced as gauges. The five
// counters come from Stats() (cumulative); entries/capacity come from Len()/
// Capacity() (current). All are gauges rather than counters because
// [bigcache.BigCache.ResetStats] can reset the counters, breaking monotonicity.
var statInstruments = []statInstrument{
	{name: "bigcache.hits", desc: "Number of successfully found keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().Hits }},
	{name: "bigcache.misses", desc: "Number of not found keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().Misses }},
	{name: "bigcache.delete_hits", desc: "Number of successfully deleted keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().DelHits }},
	{name: "bigcache.delete_misses", desc: "Number of not deleted keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().DelMisses }},
	{name: "bigcache.collisions", desc: "Number of key hash collisions", read: func(c *bigcache.BigCache) int64 { return c.Stats().Collisions }},
	{name: "bigcache.entries", desc: "Current number of stored entries", read: func(c *bigcache.BigCache) int64 { return int64(c.Len()) }},
	{name: "bigcache.capacity", desc: "Maximum number of entries the cache can hold", read: func(c *bigcache.BigCache) int64 { return int64(c.Capacity()) }},
}

// dbObserver emits the trace span, the duration metric, and the access log for
// per-operation cache traffic, plus the cache-statistics gauges. bigcache is an
// in-process heap cache with no network, so the spans are root spans (no caller
// context to link) and the durations are sub-microsecond - the value is per-key
// access visibility and a uniform signal vocabulary with the other client
// starters.
//
// It carries no per-instance state: the values its gauges report are read from
// the *bigcache.BigCache each cache registers itself with (see observeGauges).
//
// It rides the OTel globals: when starter-otel is not imported the global
// TracerProvider and MeterProvider are no-ops, so trace+metric add negligible
// overhead and the span's SpanContext stays invalid. The access log always
// emits through the project log package.
type dbObserver struct {
	meter    metric.Meter
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
	gauges   []metric.Int64ObservableGauge
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before any bean is built, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
//
// The gauges are created WITHOUT a callback on purpose; see observeGauges for
// why a creation-time callback would report only the first cache in a process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *dbObserver {
	m := otel.Meter(obsTracer)
	o := &dbObserver{meter: m}
	o.duration, _ = m.Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of cache operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	o.active, _ = m.Int64UpDownCounter("db.client.active_requests",
		metric.WithDescription("Number of in-flight cache operations"),
		metric.WithUnit("{request}"))
	o.gauges = make([]metric.Int64ObservableGauge, len(statInstruments))
	for i, inst := range statInstruments {
		o.gauges[i], _ = m.Int64ObservableGauge(inst.name, metric.WithDescription(inst.desc))
	}
	return o
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// observeGauges registers c's statistics as this cache's own observations of the
// shared gauges, labeled with name so several caches in one process stay
// distinguishable, and returns the registration the caller holds until the cache
// is destroyed.
//
// The instruments are registered here rather than given a creation-time callback
// (metric.WithInt64Callback): the SDK keys an observable instrument by
// name/description/unit/kind and returns the first instrument on every later
// creation, silently dropping the new callbacks. A process with more than one
// cache would then report only the first one, with no error to show for it.
func (o *dbObserver) observeGauges(c *bigcache.BigCache, name string) (metric.Registration, error) {
	attrs := metric.WithAttributes(attribute.String("cache.name", name))
	insts := make([]metric.Observable, len(o.gauges))
	for i, g := range o.gauges {
		insts[i] = g
	}
	return o.meter.RegisterCallback(func(_ context.Context, ob metric.Observer) error {
		for i, inst := range statInstruments {
			ob.ObserveInt64(o.gauges[i], inst.read(c), attrs)
		}
		return nil
	}, insts...)
}

// inflightOf names the in-flight gauge's dimensions. The +1 at start and the
// -1 at end must carry identical attributes or the gauge never balances, so
// both go through here.
func inflightOf(op string) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("db.system", bigcacheSystem),
		attribute.String("db.operation", op),
	)
}

// start opens the operation's client span. arg is the cache key, carried as
// db.statement truncated to 512 bytes so a pathological key can't flood the
// span attributes.
func (o *dbObserver) start(ctx context.Context, op, arg string) (context.Context, trace.Span) {
	o.active.Add(ctx, 1, inflightOf(op))
	attrs := []attribute.KeyValue{
		attribute.String("db.system", bigcacheSystem),
		attribute.String("db.operation", op),
	}
	if arg != "" {
		attrs = append(attrs, attribute.String("db.statement", strutil.Truncate(arg, 512)))
	}
	return otel.Tracer(obsTracer).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...))
}

// record ends the span and emits the duration metric and the access log for
// one finished operation. The log level carries the outcome: an error at
// Warn, a success with an argument at Debug (cache access is frequent and
// uninteresting until something fails), an argument-less success at Info;
// the Debug fields are built lazily since they are only needed when debug
// logging is on.
func (o *dbObserver) record(ctx context.Context, op, arg string, start time.Time, span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()

	dur := float64(time.Since(start).Nanoseconds()) / 1e6
	status := statusOf(err)
	o.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("db.system", bigcacheSystem),
		attribute.String("db.operation", op),
		attribute.String("status", status),
	))
	o.active.Add(ctx, -1, inflightOf(op))

	fields := func() []log.Field {
		fs := []log.Field{
			log.String("db.operation", op),
			log.String("status", status),
			log.Float("duration_ms", dur),
		}
		if arg != "" {
			fs = append(fs, log.String("db.statement", strutil.Truncate(arg, 512)))
		}
		return fs
	}
	switch {
	case err != nil:
		log.Warn(ctx, accessTag, append(fields(), log.Err(err))...)
	case arg != "":
		log.Debug(ctx, accessTag, fields)
	default:
		log.Info(ctx, accessTag, fields()...)
	}
}

// statusOf maps the outcome to the coarse metric/log dimension.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
