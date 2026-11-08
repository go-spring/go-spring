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

// instrumentSet is the read-side instrument set: one per observer, resolved
// when the observer is constructed. It holds no per-instance state — the value
// the observable gauge reports lives in each Observer, not here.
//
// The gauge is created WITHOUT a callback on purpose; see [Observer.register]
// for why a creation-time callback would report only the first block.
type instrumentSet struct {
	meter     metric.Meter
	syncTotal metric.Int64Counter
	age       metric.Float64ObservableGauge
}

// buildInstruments resolves this package's instruments against the meter
// provider the otel starter installs before any bean is constructed. The
// observer owns the set, so a failed registration fails construction instead of
// silently dropping the whole metric family.
func buildInstruments() (*instrumentSet, error) {
	m := otel.Meter(componentName)
	in := &instrumentSet{meter: m}
	var err error
	in.syncTotal, err = m.Int64Counter("discovery.sync_total",
		metric.WithDescription("Background endpoint-cache syncs, by outcome"))
	if err != nil {
		return nil, errutil.Explain(err, "discovery: register discovery.sync_total")
	}
	in.age, err = m.Float64ObservableGauge("discovery.cache.age_seconds",
		metric.WithDescription("Seconds since the cached endpoint snapshot for this service was last confirmed fresh"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, errutil.Explain(err, "discovery: register discovery.cache.age_seconds")
	}
	return in, nil
}

// Observer reports one configured discovery block's READ side: the background
// endpoint syncs and the freshness gauge they feed. The write side — publish /
// deregister / weight — is observed by the registration package on the control
// plane; the two observers are independent so neither package has to import the
// other's plane.
//
// The two axes of that identity travel with every reading: system names the
// backend implementation ("etcd", "nacos", ...), center names the configured
// block (${spring.discovery.<backend>.<name>}). Both are needed because one
// process routinely talks to several centers at once.
type Observer struct {
	system string
	center string

	// ins is this block's instrument set, resolved at construction.
	ins *instrumentSet

	// regOnce registers this block's gauge callback on the set resolved at
	// construction. It runs on first use so a block that never reports never
	// registers a callback.
	regOnce sync.Once

	// mu guards the state below: the map the observable gauge reads, and the
	// callback registrations Close needs. The gauge callback snapshots the map
	// instead of observing under the lock: OTel advises against locking inside a
	// callback (Go mutexes are not reentrant, and collection is concurrent).
	mu     sync.Mutex
	closed bool
	syncs  map[string]*syncState
	unregs []metric.Registration
}

// NewObserver builds the observer for one configured block. system and center
// are the values every instrument attribute and log field of that block carries,
// so neither may be empty — an empty one would label the readings of this block
// indistinguishably from another's, which is a defect that shows up only in a
// dashboard, long after the config that caused it.
func NewObserver(system, center string) (*Observer, error) {
	if system == "" {
		return nil, errutil.Explain(nil, "discovery: observer system is required")
	}
	if center == "" {
		return nil, errutil.Explain(nil, "discovery: observer center is required")
	}
	ins, err := buildInstruments()
	if err != nil {
		return nil, err
	}
	return &Observer{
		system: system,
		center: center,
		ins:    ins,
		syncs:  map[string]*syncState{},
	}, nil
}

// System and Center report this block's identity — the values its instrument
// attributes carry. A backend's log lines take them from here rather than from
// its own copy, so a log line and a metric can never disagree about which block
// they describe.
func (o *Observer) System() string { return o.system }

// Center names the configured discovery block (see [NewObserver]).
func (o *Observer) Center() string { return o.center }

// attrs is the identity prefix every instrument of this block carries.
func (o *Observer) attrs(extra ...attribute.KeyValue) []attribute.KeyValue {
	return append([]attribute.KeyValue{
		attribute.String("system", o.system),
		attribute.String("center", o.center),
	}, extra...)
}

// live reports whether this observer still publishes. A closed observer stops
// reporting; the sync itself still runs.
//
// A nil observer — one a caller never wired — reports nothing either, the same
// pass-through as a nil injector in the direction's fault package: instrumentation is
// declared by construction, and its absence must never decide whether an
// operation runs.
func (o *Observer) live() bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.closed
}

// register registers this block's gauge callback against the shared gauge. It
// runs once per observer and pairs with [Observer.Close]'s unregistration: a
// block that is torn down (and rebuilt later, as a re-armed watch is) must not
// leave its callback behind, or the same service would be reported twice.
//
// RegisterCallback is used rather than a creation-time callback because one
// instrument descriptor can only be created once per meter: a second observer's
// creation-time callbacks would be silently dropped, so every block but the
// first would report nothing. RegisterCallback is additive, and unlike creation
// it is reversible.
func (o *Observer) register() {
	o.regOnce.Do(func() {
		in := o.ins
		if reg, err := in.meter.RegisterCallback(o.observeAge, in.age); err == nil {
			o.mu.Lock()
			o.unregs = append(o.unregs, reg)
			o.mu.Unlock()
		}
	})
}

// ready registers this block's gauge callback. The instruments were resolved at
// construction; calling this on every reporting path is cheap because [register]
// is once-only.
func (o *Observer) ready() {
	o.register()
}

// observeAge reports, per service, how long this block's cached endpoint
// snapshot has gone unconfirmed. A service whose watch died reports a climbing
// value instead of nothing at all, which is the failure this gauge exists for.
func (o *Observer) observeAge(_ context.Context, ob metric.Observer) error {
	now := obsNow()
	o.mu.Lock()
	snapshot := make(map[string]time.Duration, len(o.syncs))
	for service, s := range o.syncs {
		snapshot[service] = s.ageAt(now)
	}
	o.mu.Unlock()
	age := o.ins.age
	for service, d := range snapshot {
		ob.ObserveFloat64(age, d.Seconds(), metric.WithAttributes(o.attrs(attribute.String("service", service))...))
	}
	return nil
}

// Close stops this observer from reporting and unregisters its gauge callback.
// It is the block destructor's obligation: a block that is torn down (and
// rebuilt later, as a re-armed watch is) must not leave its callback behind, or
// the same service would be reported twice.
func (o *Observer) Close() error {
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

// Synced reports one background endpoint-cache sync: a successful one advances
// this service's freshness clock, a failed one only counts.
//
// There is deliberately no span (a span per sync would be noise) and the
// backend already logs failures itself. Only a successful sync advances
// freshness, so a watch that has died shows up as a steadily climbing
// discovery.cache.age_seconds rather than a value frozen at zero — the failure
// mode where discovery keeps handing out stale addresses that look perfectly
// healthy.
func (o *Observer) Synced(service string, err error) {
	if !o.live() {
		return
	}
	o.ready()
	ctx := context.Background()
	o.ins.syncTotal.Add(ctx, 1, metric.WithAttributes(
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
