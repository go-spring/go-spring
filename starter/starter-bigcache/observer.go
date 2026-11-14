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

// observer.go is bigcache's observability seam: the per-call and per-instance
// events this starter reports, and the default OTel-backed adapter.

package StarterBigCache

import (
	"context"

	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter in this package
// reports under.
const componentName = "go-spring.org/starter-bigcache"

// Stats is one cache instance's current statistics, read once per collection.
type Stats struct {
	// Hits, Misses, DeleteHits, DeleteMisses, Collisions are the cumulative
	// counters bigcache keeps when stats are enabled.
	Hits, Misses, DeleteHits, DeleteMisses, Collisions int64
	// Entries and Capacity are the current entry count and allocated bytes.
	Entries, Capacity int64
}

// Observer receives bigcache's observability events. A nil Observer disables
// observation; implementations must not block.
type Observer interface {
	// Operation records one cache operation. instance names the cache, operation
	// is get|set|delete and status is ok|error.
	Operation(ctx context.Context, instance, operation, status string)

	// ObserveStats registers read as the source of one cache's statistics,
	// labelled with instance. The returned off removes it; after off the reader
	// is never called again.
	ObserveStats(instance string, read func() Stats) (off func(), err error)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer].
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{meter: m}

	o.gauges = make([]metric.Int64ObservableGauge, len(statInstruments))
	for i, inst := range statInstruments {
		g, err := m.Int64ObservableGauge(inst.name, metric.WithDescription(inst.desc))
		if err != nil {
			return nil, errutil.Explain(err, "bigcache: create gauge %q", inst.name)
		}
		o.gauges[i] = g
	}

	total, err := m.Int64Counter("bigcache.operation.total",
		metric.WithDescription("Cache operations executed, by operation and status"),
		metric.WithUnit("{operation}"))
	if err != nil {
		return nil, errutil.Explain(err, "bigcache: create counter %q", "bigcache.operation.total")
	}
	o.total = total
	return o, nil
}

// statInstrument describes one gauge to register.
type statInstrument struct {
	name string
	desc string
	read func(Stats) int64
}

// statInstruments lists the bigcache statistics surfaced as gauges. The five
// counters come from Stats() (cumulative); entries/capacity come from Len()/
// Capacity() (current). All are gauges rather than counters because
// [bigcache.BigCache.ResetStats] can reset the counters, breaking monotonicity.
var statInstruments = []statInstrument{
	{name: "bigcache.hits", desc: "Number of successfully found keys", read: func(s Stats) int64 { return s.Hits }},
	{name: "bigcache.misses", desc: "Number of not found keys", read: func(s Stats) int64 { return s.Misses }},
	{name: "bigcache.delete_hits", desc: "Number of successfully deleted keys", read: func(s Stats) int64 { return s.DeleteHits }},
	{name: "bigcache.delete_misses", desc: "Number of not deleted keys", read: func(s Stats) int64 { return s.DeleteMisses }},
	{name: "bigcache.collisions", desc: "Number of key hash collisions", read: func(s Stats) int64 { return s.Collisions }},
	{name: "bigcache.entries", desc: "Current number of stored entries", read: func(s Stats) int64 { return s.Entries }},
	{name: "bigcache.capacity", desc: "Bytes allocated for the entries queues, summed over shards", read: func(s Stats) int64 { return s.Capacity }},
}

// otelObserver is the default [Observer].
type otelObserver struct {
	meter  metric.Meter
	gauges []metric.Int64ObservableGauge
	total  metric.Int64Counter
}

// Operation implements [Observer].
func (o *otelObserver) Operation(ctx context.Context, instance, operation, status string) {
	o.total.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
		attribute.String("instance", instance),
	))
}

// ObserveStats implements [Observer]. The callback is registered per cache
// rather than as a creation-time callback: the SDK keys an observable instrument
// by name/description/unit/kind and returns the first instrument on every later
// creation, silently dropping the new callbacks — a process with more than one
// cache would then report only the first.
func (o *otelObserver) ObserveStats(instance string, read func() Stats) (func(), error) {
	attrs := metric.WithAttributes(attribute.String("instance", instance))
	insts := make([]metric.Observable, len(o.gauges))
	for i, g := range o.gauges {
		insts[i] = g
	}
	reg, err := o.meter.RegisterCallback(func(_ context.Context, ob metric.Observer) error {
		snap := read()
		for i, inst := range statInstruments {
			ob.ObserveInt64(o.gauges[i], inst.read(snap), attrs)
		}
		return nil
	}, insts...)
	if err != nil {
		return nil, err
	}
	return func() { _ = reg.Unregister() }, nil
}

// The package's default [Observer] bean: an application that contributes its
// own observer takes precedence.
func init() {
	gs.Provide(NewOTelObserver).Condition(gs.OnMissingBean[Observer]()).Caller(1)
}
