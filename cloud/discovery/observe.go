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

package discovery

import (
	"context"
	"sync"
	"time"

	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation component name every meter in this
// package reports under. The registration half has its own component name; the
// shared `discovery.*` metric names are what group the two planes on a
// dashboard, not a borrowed package path.
// The string keeps the historical path on purpose: it is the OTel
// instrumentation scope (a backend-facing contract), not this package's import
// path.
const componentName = "go-spring.org/cloud/governance/dataplane/discovery"

// obsNow reads the wall clock. A variable so tests can advance time and assert
// that a stale snapshot's age actually climbs.
var obsNow = time.Now

// Observer reports one configured discovery block's READ side: the background
// endpoint syncs and the freshness gauge they feed. The write side — publish /
// deregister / weight — is observed by the registration package on the control
// plane; the two observers are independent so neither package has to import the
// other's plane.
//
// The two axes of that identity travel with every reading: system names the
// backend implementation ("etcd", "nacos", ...), center names the configured
// block (${spring.discovery.<backend>.<name>}). Both are needed because one
// process routinely talks to several centers at once. A nil Observer reports
// nothing.
type Observer interface {
	// Synced reports one background endpoint-cache sync. A successful sync
	// advances this service's freshness clock; a failed one only counts.
	Synced(service string, err error)

	// System and Center report this block's identity — the values every reading
	// and log line carries.
	System() string
	Center() string

	// Close stops reporting and releases the freshness-gauge callback. It must
	// be safe to call more than once.
	Close() error
}

// ObserverFactory builds an [Observer] for one configured block. It is the
// injection seam: a backend starter asks the factory for the block's observer,
// and an application may replace the factory with its own bean.
type ObserverFactory interface {
	New(system, center string) (Observer, error)
}

// NewOTelObserverFactory returns the default OTel-backed [ObserverFactory].
func NewOTelObserverFactory() (ObserverFactory, error) {
	return otelObserverFactory{}, nil
}

// NewObserver builds the default observer for one configured block. It is the
// convenience entry for a backend that is not wired through the container.
func NewObserver(system, center string) (Observer, error) {
	return otelObserverFactory{}.New(system, center)
}

// otelObserverFactory resolves no instruments itself: each block resolves its
// own set, so a block that is never used registers no metrics.
type otelObserverFactory struct{}

// New implements [ObserverFactory].
func (otelObserverFactory) New(system, center string) (Observer, error) {
	if system == "" {
		return nil, errutil.Explain(nil, "discovery: observer system is required")
	}
	if center == "" {
		return nil, errutil.Explain(nil, "discovery: observer center is required")
	}
	m := otel.Meter(componentName)
	o := &otelObserver{
		system: system,
		center: center,
		meter:  m,
		syncs:  map[string]*syncState{},
	}
	var err error
	o.syncTotal, err = m.Int64Counter("discovery.sync_total",
		metric.WithDescription("Background endpoint-cache syncs, by outcome"))
	if err != nil {
		return nil, errutil.Explain(err, "discovery: register discovery.sync_total")
	}
	// The gauge is created WITHOUT a callback on purpose; see
	// [otelObserver.register] for why a creation-time callback would report only
	// the first block.
	o.age, err = m.Float64ObservableGauge("discovery.cache.age_seconds",
		metric.WithDescription("Seconds since the cached endpoint snapshot for this service was last confirmed fresh"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, errutil.Explain(err, "discovery: register discovery.cache.age_seconds")
	}
	return o, nil
}

// syncState tracks how fresh one service's cached endpoint snapshot is.
// origin is the first reported attempt, last advances only on a successful
// sync. Measuring from last — not from origin — is what makes a dead watch
// show a climbing age instead of a frozen zero, and falling back to origin
// keeps a service that has never synced successfully visible.
type syncState struct {
	origin time.Time
	last   time.Time
}

// ageAt returns how long the snapshot has been unconfirmed as of now.
func (s syncState) ageAt(now time.Time) time.Duration {
	if s.last.IsZero() {
		return now.Sub(s.origin)
	}
	return now.Sub(s.last)
}

// otelObserver is the default [Observer]. Its instruments are resolved when the
// observer is constructed; the value the observable gauge reports lives on the
// observer itself.
type otelObserver struct {
	system string
	center string

	// meter and the instruments resolved against it at construction. The gauge
	// is created without a callback; see [otelObserver.register].
	meter     metric.Meter
	syncTotal metric.Int64Counter
	age       metric.Float64ObservableGauge

	// regOnce registers this block's gauge callback on the set resolved at
	// construction. It runs on first use so a block that never reports never
	// registers a callback.
	regOnce sync.Once

	// mu guards the state below: the map the observable gauge reads, and the
	// callback registrations Close needs. The gauge callback snapshots the map
	// instead of observing under the lock: OTel advises against locking inside a
	// callback.
	mu     sync.Mutex
	closed bool
	syncs  map[string]*syncState
	unregs []metric.Registration
}

// System implements [Observer].
func (o *otelObserver) System() string { return o.system }

// Center implements [Observer].
func (o *otelObserver) Center() string { return o.center }

// attrs is the identity prefix every instrument of this block carries.
func (o *otelObserver) attrs(extra ...attribute.KeyValue) []attribute.KeyValue {
	return append([]attribute.KeyValue{
		attribute.String("system", o.system),
		attribute.String("center", o.center),
	}, extra...)
}

// live reports whether this observer still publishes.
func (o *otelObserver) live() bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.closed
}

// register registers this block's gauge callback against the shared gauge. It
// runs once per observer and pairs with [otelObserver.Close]'s unregistration.
//
// RegisterCallback is used rather than a creation-time callback because one
// instrument descriptor can only be created once per meter: a second observer's
// creation-time callbacks would be silently dropped.
func (o *otelObserver) register() {
	o.regOnce.Do(func() {
		if reg, err := o.meter.RegisterCallback(o.observeAge, o.age); err == nil {
			o.mu.Lock()
			o.unregs = append(o.unregs, reg)
			o.mu.Unlock()
		}
	})
}

// observeAge reports, per service, how long this block's cached endpoint
// snapshot has gone unconfirmed.
func (o *otelObserver) observeAge(_ context.Context, ob metric.Observer) error {
	now := obsNow()
	o.mu.Lock()
	snapshot := make(map[string]time.Duration, len(o.syncs))
	for service, s := range o.syncs {
		snapshot[service] = s.ageAt(now)
	}
	o.mu.Unlock()
	age := o.age
	for service, d := range snapshot {
		ob.ObserveFloat64(age, d.Seconds(), metric.WithAttributes(o.attrs(attribute.String("service", service))...))
	}
	return nil
}

// Close implements [Observer].
func (o *otelObserver) Close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	o.closed = true
	unregs := o.unregs
	o.unregs = nil
	o.mu.Unlock()
	for _, r := range unregs {
		_ = r.Unregister()
	}
	return nil
}

// StatusOf names an outcome the way the metric attributes expect. A backend's
// log line carries this under "status" rather than spelling the vocabulary
// itself: the value is "failed" — not the "error" the RPC and DB families use —
// and a line that gets it wrong silently stops joining the counter and the span
// for the operation it explains.
func StatusOf(err error) string {
	if err != nil {
		return "failed"
	}
	return "ok"
}

// Synced implements [Observer]. There is deliberately no span (a span per sync
// would be noise) and the backend already logs failures itself.
func (o *otelObserver) Synced(service string, err error) {
	if !o.live() {
		return
	}
	o.register()
	ctx := context.Background()
	o.syncTotal.Add(ctx, 1, metric.WithAttributes(
		o.attrs(
			attribute.String("service", service),
			attribute.String("status", StatusOf(err)),
		)...))

	now := obsNow()
	o.mu.Lock()
	defer o.mu.Unlock()
	s := o.syncs[service]
	if s == nil {
		s = &syncState{origin: now}
		o.syncs[service] = s
	}
	if err == nil {
		s.last = now
	}
}
