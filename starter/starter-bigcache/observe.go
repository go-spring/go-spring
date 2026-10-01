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

// observe.go declares what a bigcache operation IS, and owns the one signal of
// this module that is not per-call: the cache-statistics gauges.
//
// The per-call signals themselves — the span, the duration metrics, the access
// log — are emitted by the resilience layer, the single point on the executor
// chain that sees a whole call (retries included). This file therefore holds no
// per-call emission code: only the vocabulary that this starter alone knows,
// because only it knows these calls reach an in-process cache, plus the
// [metric.Int64ObservableGauge] set each cache feeds from its own statistics.
package StarterBigCache

import (
	"context"
	"sync"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/stdlib/strutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// accessTag is the static log tag of the bigcache access log (renders as
// "_app_bigcache_access"). It is registered here, at package init, because a tag
// must exist before the framework's first property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("bigcache", "access")

// obsScope names the meter this starter's cache-statistics gauges register
// under. It is a meter scope only: the per-call span and metrics are emitted by
// the resilience layer, under its own scope.
const obsScope = "go-spring.org/starter-bigcache"

// bigcacheSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const bigcacheSystem = "bigcache"

// maxStatement bounds the key captured as db.statement. A key can be long and a
// span attribute or a log line has no use for all of it.
const maxStatement = 512

// operation is the semantic identity of one bigcache command.
//
// The key rides in Detail rather than Attrs: cache keys are drawn from an open
// set, so as a metric label one would multiply the series without bound. Detail
// reaches the span and the log — where a key is exactly what makes a line worth
// reading — and never a label. Every bigcache command carries a key, so an
// argument is always present; an empty key would carry no detail at all, which
// is also what would level its success log at Info.
func operation(op, key string) observability.Operation {
	o := observability.Operation{
		Name:   op,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", bigcacheSystem),
			attribute.String("db.operation", op),
		},
		LogTag: accessTag,
	}
	if key != "" {
		o.Detail = []attribute.KeyValue{
			attribute.String("db.statement", strutil.Truncate(key, maxStatement)),
		}
	}
	return o
}

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

// statObserver holds this starter's cache-statistics gauges. bigcache is an
// in-process heap cache with no network, so there is no per-call signal for this
// layer to emit: only these process-level gauges, which the resilience layer
// cannot know about because they are not tied to any one call.
//
// It carries no per-instance state: the values its gauges report are read from
// the *bigcache.BigCache each cache registers itself with (see observeGauges).
// It rides the OTel globals: when starter-otel is not imported the global
// MeterProvider is a no-op, so registering the gauges adds negligible overhead.
type statObserver struct {
	meter  metric.Meter
	gauges []metric.Int64ObservableGauge
}

// instruments is the one gauge set this starter uses for the whole process.
// Resolution is deferred to the first use, not run at package init, so the
// instruments bind to whichever providers are current then - starter-otel
// installs them before any bean is built, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
//
// The gauges are created WITHOUT a callback on purpose; see observeGauges for
// why a creation-time callback would report only the first cache in a process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *statObserver {
	m := otel.Meter(obsScope)
	o := &statObserver{meter: m}
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
func (o *statObserver) observeGauges(c *bigcache.BigCache, name string) (metric.Registration, error) {
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
