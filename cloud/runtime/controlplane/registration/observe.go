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

package registration

import (
	"context"
	"maps"
	"sync"
	"time"

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// componentName is the instrumentation component name every meter and tracer in
// this package reports under. The read half has its own component name; the
// shared `discovery.*` metric names are what group the two planes on a
// dashboard, not a borrowed package path.
const componentName = "go-spring.org/cloud/runtime/controlplane/registration"

// Registration reasons a backend passes to [Observer.RegisterAttempt]: the first
// publish of an instance, and a background re-publish after the center lost it.
// A rising self_heal count (or a registered gauge stuck at 0) is the signal
// that an instance is serving while no longer discoverable.
const (
	ReasonInitial  = "initial"
	ReasonSelfHeal = "self_heal"
)

// operation names shared by the span, the duration histogram and the counters.
const (
	opRegister     = "register"
	opDeregister   = "deregister"
	opUpdateWeight = "update_weight"
)

// obsNow reads the wall clock. A variable so tests can advance time.
var obsNow = time.Now

// instrumentSet is the registration instrument set: one per process, resolved
// lazily on first use so it binds to whichever providers are current then, and
// immutable afterwards. It holds no per-instance state — the value the
// observable gauge reports lives in each Observer, not here.
//
// The gauge is created WITHOUT a callback on purpose; see [Observer.register]
// for why a creation-time callback would report only the first block.
type instrumentSet struct {
	meter       metric.Meter
	opDuration  metric.Float64Histogram
	regAttempts metric.Int64Counter
	registered  metric.Int64ObservableGauge
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter(componentName)
	in := &instrumentSet{meter: m}
	in.opDuration, _ = m.Float64Histogram("discovery.operation.duration",
		metric.WithDescription("Duration of discovery-center registration operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	in.regAttempts, _ = m.Int64Counter("discovery.registration.attempts_total",
		metric.WithDescription("Registration attempts against a discovery center, by reason and outcome"))
	in.registered, _ = m.Int64ObservableGauge("discovery.instance.registered",
		metric.WithDescription("1 while this instance is published into the discovery center, 0 while it is not"))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// Observer reports one configured registration block: the publish, deregister
// and weight operations of that block and the "is this instance published"
// gauge they feed. It is a layer the block owns — created by the block's
// constructor, closed by its destructor — so the state it accumulates lives
// exactly as long as the block that produced it, and two blocks in one process
// cannot contaminate each other. The instrument set those readings go through is
// shared by every block (see [instruments]); only the state is per block.
//
// The two axes of that identity travel with every reading: system names the
// backend implementation ("etcd", "nacos", ...), center names the configured
// block (${spring.discovery.<backend>.<name>}). Both are needed for more than
// bookkeeping: one process routinely publishes the same service into several
// centers at once (the discovery core drives every configured registry), so
// without center the readings of two clusters would be one indistinguishable
// series.
type Observer struct {
	system string
	center string

	// regOnce registers this block's gauge callback against the shared
	// instruments. It runs on first use, not at construction: the callback must
	// be in place before the first collection, and the instruments must be
	// resolved after starter-otel has installed the global providers — which a
	// test may also do later than this constructor runs.
	regOnce sync.Once

	// mu guards the state below: the map the observable gauge reads, and the
	// callback registrations Close needs. The gauge callback snapshots the map
	// instead of observing under the lock: OTel advises against locking inside a
	// callback (Go mutexes are not reentrant, and collection is concurrent).
	mu        sync.Mutex
	closed    bool
	published map[string]bool
	unregs    []metric.Registration
}

// NewObserver builds the observer for one configured block. system and center
// are the values every instrument attribute and log field of that block carries,
// so neither may be empty — an empty one would label the readings of this block
// indistinguishably from another's, which is a defect that shows up only in a
// dashboard, long after the config that caused it.
func NewObserver(system, center string) (*Observer, error) {
	if system == "" {
		return nil, errutil.Explain(nil, "registration: observer system is required")
	}
	if center == "" {
		return nil, errutil.Explain(nil, "registration: observer center is required")
	}
	return &Observer{
		system:    system,
		center:    center,
		published: map[string]bool{},
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
// reporting and drops the operation's span; the operation itself still runs.
//
// A nil observer — one a caller never wired — reports nothing either, the same
// pass-through as a nil injector in cloud/runtime/dataplane/fault: instrumentation is
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
// block that is torn down (and rebuilt later, as a re-registered instance is)
// must not leave its callback behind, or the same service would be reported
// twice.
//
// RegisterCallback is used rather than a creation-time callback because one
// instrument descriptor can only be created once per meter: a second observer's
// creation-time callbacks would be silently dropped, so every block but the
// first would report nothing. RegisterCallback is additive, and unlike creation
// it is reversible.
func (o *Observer) register() {
	o.regOnce.Do(func() {
		in := instruments()
		if reg, err := in.meter.RegisterCallback(o.observeRegistered, in.registered); err == nil {
			o.mu.Lock()
			o.unregs = append(o.unregs, reg)
			o.mu.Unlock()
		}
	})
}

// ready resolves the shared instruments and registers this block's gauge
// callback. Both happen once; calling it on every reporting path is what keeps
// the resolution lazy — after starter-otel has installed the global providers,
// and not at construction.
func (o *Observer) ready() {
	instruments()
	o.register()
}

// observeRegistered reports this block's per-service publication state.
func (o *Observer) observeRegistered(_ context.Context, ob metric.Observer) error {
	o.mu.Lock()
	snapshot := make(map[string]bool, len(o.published))
	maps.Copy(snapshot, o.published)
	o.mu.Unlock()
	registered := instruments().registered
	for service, ok := range snapshot {
		var v int64
		if ok {
			v = 1
		}
		ob.ObserveInt64(registered, v, metric.WithAttributes(o.attrs(attribute.String("service", service))...))
	}
	return nil
}

// Close stops this observer from reporting and unregisters its gauge callback.
// It is the block destructor's obligation: a block that is torn down (and
// rebuilt later, as a re-registered instance is) must not leave its callback
// behind, or the same service would be reported twice.
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

// StatusOf names an outcome the way the metric attributes and the span status
// expect. A backend's log line carries this under "status" rather than spelling
// the vocabulary itself: the value is "failed" — not the "error" the RPC and
// DB families use — and a line that gets it wrong silently stops joining the
// counter and the span for the operation it explains.
func StatusOf(err error) string {
	if err != nil {
		return "failed"
	}
	return "ok"
}

// startOp opens the client span for one discovery-center operation. The span
// attributes are namespaced (discovery.*) while the metric attributes are bare,
// matching the other domain packages' instrumentation.
func (o *Observer) startOp(ctx context.Context, op, service, reason string) (context.Context, trace.Span) {
	o.ready()
	attrs := []attribute.KeyValue{
		attribute.String("discovery.system", o.system),
		attribute.String("discovery.center", o.center),
		attribute.String("discovery.operation", op),
		attribute.String("discovery.service", service),
	}
	if reason != "" {
		attrs = append(attrs, attribute.String("discovery.reason", reason))
	}
	return otel.Tracer(componentName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...))
}

// endOp closes the span and records the operation's duration.
func (o *Observer) endOp(ctx context.Context, span trace.Span, op, service string, start time.Time, err error) {
	o.ready()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	instruments().opDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		o.attrs(
			attribute.String("operation", op),
			attribute.String("service", service),
			attribute.String("status", StatusOf(err)),
		)...))
}

// RegisterAttempt runs and reports one registration attempt against this
// observer's discovery center: fn executes the attempt, and its error becomes the
// attempt's outcome. It returns fn's error unchanged so the caller keeps its
// normal error flow.
//
// fn receives the context carrying the attempt's span: the span is created
// here, so a backend that logs with its own context emits lines that join
// nothing — a span and a log for the same event, unconnected. Taking fn's
// context as a parameter (rather than returning it alongside a finisher) makes
// threading it into the attempt the path of least resistance.
//
// A backend reports at the seam BOTH its first publish and its background
// re-registration funnel through, so a self-healing re-publish is covered too.
// Pass [ReasonInitial] for the first publish and [ReasonSelfHeal] for a
// re-publish; the distinction is what exposes an instance that is serving
// while no longer discoverable.
//
// The gauge follows the outcome either way: a failed attempt means the instance
// is not published (any more), which is exactly the state worth alerting on.
// A panic inside fn is reported as a failed attempt before propagating, so an
// aborted attempt never leaves the span and gauge dangling.
func (o *Observer) RegisterAttempt(ctx context.Context, service, reason string, fn func(ctx context.Context) error) error {
	if !o.live() {
		return fn(ctx)
	}
	start := obsNow()
	ctx, span := o.startOp(ctx, opRegister, service, reason)
	return finishAttempt(ctx, func(err error) {
		o.endOp(ctx, span, opRegister, service, start, err)
		instruments().regAttempts.Add(ctx, 1, metric.WithAttributes(
			o.attrs(
				attribute.String("service", service),
				attribute.String("reason", reason),
				attribute.String("status", StatusOf(err)),
			)...))
		o.setPublished(service, err == nil)
	}, fn)
}

// DeregisterAttempt runs and reports one deregistration attempt (see
// [Observer.RegisterAttempt] for the fn/context contract). Unlike registration,
// a FAILED deregister leaves the published state untouched — the instance may
// well still be discoverable, and reporting it as gone would hide a leak.
func (o *Observer) DeregisterAttempt(ctx context.Context, service string, fn func(ctx context.Context) error) error {
	if !o.live() {
		return fn(ctx)
	}
	start := obsNow()
	ctx, span := o.startOp(ctx, opDeregister, service, "")
	return finishAttempt(ctx, func(err error) {
		o.endOp(ctx, span, opDeregister, service, start, err)
		if err == nil {
			o.setPublished(service, false)
		}
	}, fn)
}

// WeightChange runs and reports one weight re-advertisement — the operation
// that keeps an instance discoverable while it drains, and the one path the
// discovery core does not log (see [Observer.RegisterAttempt] for the fn/context
// contract). It carries no reason: a weight change is never a re-register.
func (o *Observer) WeightChange(ctx context.Context, service string, fn func(ctx context.Context) error) error {
	if !o.live() {
		return fn(ctx)
	}
	start := obsNow()
	ctx, span := o.startOp(ctx, opUpdateWeight, service, "")
	return finishAttempt(ctx, func(err error) {
		o.endOp(ctx, span, opUpdateWeight, service, start, err)
	}, fn)
}

// finishAttempt runs fn, reports the outcome through end, and returns fn's
// error unchanged. A panic inside fn is reported as a failed attempt before it
// propagates, so an aborted attempt never leaves the span and metrics dangling.
func finishAttempt(ctx context.Context, end func(err error), fn func(ctx context.Context) error) error {
	var err error
	defer func() {
		if p := recover(); p != nil {
			end(errutil.Explain(nil, "panic: %v", p))
			panic(p)
		}
		end(err)
	}()
	err = fn(ctx)
	return err
}

// setPublished records whether the instance is currently published.
//
// The state is per service and lives as long as this observer: a discovery block
// publishes exactly one service identity per process.
func (o *Observer) setPublished(service string, ok bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.published[service] = ok
}
