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
const componentName = "go-spring.org/cloud/governance/controlplane/registration"

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

// Observer reports one configured registration block: the publish, deregister
// and weight operations of that block and the "is this instance published"
// gauge they feed. It is a layer the block owns — created by the block's
// constructor, closed by its destructor — so the state it accumulates lives
// exactly as long as the block that produced it, and two blocks in one process
// cannot contaminate each other. Each observer resolves its own instrument set
// at construction; only the state is per block.
//
// The two axes of that identity travel with every reading: system names the
// backend implementation ("etcd", "nacos", ...), center names the configured
// block (${spring.discovery.<backend>.<name>}). Both are needed for more than
// bookkeeping: one process routinely publishes the same service into several
// centers at once (the discovery core drives every configured registry), so
// without center the readings of two clusters would be one indistinguishable
// series.
// Observer reports one configured registry block's WRITE side: publish,
// deregister and weight-change operations, each wrapped in a span and counted
// with its outcome, plus the gauge of currently registered instances. The read
// side is the discovery package's observer; the two planes are independent.
//
// The two axes of the block's identity travel with every reading: system names
// the backend implementation, center names the configured block. A nil Observer
// reports nothing.
type Observer interface {
	// RegisterAttempt records one publish attempt, labelled with reason, and
	// moves the published gauge to whether it succeeded.
	RegisterAttempt(ctx context.Context, service, reason string, a Attempt)

	// DeregisterAttempt records one deregister attempt. A failed deregister
	// leaves the published gauge untouched.
	DeregisterAttempt(ctx context.Context, service string, a Attempt)

	// WeightChange records one weight-update attempt.
	WeightChange(ctx context.Context, service string, a Attempt)

	// System and Center report this block's identity.
	System() string
	Center() string

	// Close releases the registered-instances gauge callback. It must be safe to
	// call more than once.
	Close() error
}

// Attempt is one registry operation's measured outcome.
type Attempt struct {
	// Status is ok | failed.
	Status string
	// Duration is the operation's measured duration.
	Duration time.Duration
}

// ObserverFactory builds an [Observer] for one configured block. It is the
// injection seam: a registry starter asks the factory for the block's observer.
type ObserverFactory interface {
	New(system, center string) (Observer, error)
}

// NewOTelObserverFactory returns the default OTel-backed [ObserverFactory].
func NewOTelObserverFactory() (ObserverFactory, error) {
	return otelObserverFactory{}, nil
}

// otelObserverFactory resolves no instruments itself: each block resolves its
// own set.
type otelObserverFactory struct{}

// New implements [ObserverFactory].
func (otelObserverFactory) New(system, center string) (Observer, error) {
	if system == "" {
		return nil, errutil.Explain(nil, "registration: observer system is required")
	}
	if center == "" {
		return nil, errutil.Explain(nil, "registration: observer center is required")
	}
	m := otel.Meter(componentName)
	o := &otelObserver{
		system:    system,
		center:    center,
		meter:     m,
		published: map[string]bool{},
	}
	var err error
	o.opDuration, err = m.Float64Histogram("discovery.operation.duration",
		metric.WithDescription("Duration of discovery-center registration operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "registration: register discovery.operation.duration")
	}
	o.regAttempts, err = m.Int64Counter("discovery.registration.attempts_total",
		metric.WithDescription("Registration attempts against a discovery center, by reason and outcome"))
	if err != nil {
		return nil, errutil.Explain(err, "registration: register discovery.registration.attempts_total")
	}
	// The gauge is created WITHOUT a callback on purpose; see
	// [Observer.register].
	o.registered, err = m.Int64ObservableGauge("discovery.instance.registered",
		metric.WithDescription("1 while this instance is published into the discovery center, 0 while it is not"))
	if err != nil {
		return nil, errutil.Explain(err, "registration: register discovery.instance.registered")
	}
	return o, nil
}

type otelObserver struct {
	system string
	center string

	// meter and the instruments resolved against it at construction.
	meter       metric.Meter
	opDuration  metric.Float64Histogram
	regAttempts metric.Int64Counter
	registered  metric.Int64ObservableGauge

	// regOnce registers this block's gauge callback on the set resolved at
	// construction. It runs on first use so a block that never reports never
	// registers a callback.
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

// NewObserver builds the default observer for one configured block. It is the
// convenience entry for a backend that is not wired through the container.
func NewObserver(system, center string) (Observer, error) {
	return otelObserverFactory{}.New(system, center)
}

// System and Center report this block's identity — the values its instrument
// attributes carry. A backend's log lines take them from here rather than from
// its own copy, so a log line and a metric can never disagree about which block
// they describe.
func (o *otelObserver) System() string { return o.system }

// Center names the configured discovery block (see [NewObserver]).
func (o *otelObserver) Center() string { return o.center }

// attrs is the identity prefix every instrument of this block carries.
func (o *otelObserver) attrs(extra ...attribute.KeyValue) []attribute.KeyValue {
	return append([]attribute.KeyValue{
		attribute.String("system", o.system),
		attribute.String("center", o.center),
	}, extra...)
}

// live reports whether this observer still publishes. A closed observer stops
// reporting and drops the operation's span; the operation itself still runs.
//
// A nil observer — one a caller never wired — reports nothing either, the same
// pass-through as a nil injector in the direction's fault package: instrumentation is
// declared by construction, and its absence must never decide whether an
// operation runs.
func (o *otelObserver) live() bool {
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
func (o *otelObserver) register() {
	o.regOnce.Do(func() {
		if reg, err := o.meter.RegisterCallback(o.observeRegistered, o.registered); err == nil {
			o.mu.Lock()
			o.unregs = append(o.unregs, reg)
			o.mu.Unlock()
		}
	})
}

// ready registers this block's gauge callback. The instruments were resolved at
// construction; calling this on every reporting path is cheap because [register]
// is once-only.
func (o *otelObserver) ready() {
	o.register()
}

// observeRegistered reports this block's per-service publication state.
func (o *otelObserver) observeRegistered(_ context.Context, ob metric.Observer) error {
	o.mu.Lock()
	snapshot := make(map[string]bool, len(o.published))
	maps.Copy(snapshot, o.published)
	o.mu.Unlock()
	registered := o.registered
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

// startOp opens the standard OTel span for one discovery-center operation. Span
// handling is the STANDARD OTel capability, not part of the metric seam. The
// span attributes are namespaced (discovery.*) while the metric attributes are
// bare, matching the other domain packages' instrumentation.
func startOp(ctx context.Context, obs Observer, op, service, reason string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("discovery.system", obs.System()),
		attribute.String("discovery.center", obs.Center()),
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

// endOp closes the standard span.
func endOp(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// RegisterAttempt runs one publish attempt against this observer's discovery
// center inside its standard span: fn executes the attempt, and its error
// becomes the attempt's outcome. It returns fn's error unchanged so the caller
// keeps its normal error flow.
//
// fn receives the context carrying the attempt's span, so a backend's log lines
// join the attempt in the same trace.
//
// A backend reports at the seam BOTH its first publish and its background
// re-registration funnel through, so a self-healing re-publish is covered too.
// Pass [ReasonInitial] for the first publish and [ReasonSelfHeal] for a
// re-publish; the distinction is what exposes an instance that is serving
// while no longer discoverable.
//
// A panic inside fn is reported as a failed attempt before propagating, so an
// aborted attempt never leaves the span and gauge dangling.
func RegisterAttempt(ctx context.Context, obs Observer, service, reason string, fn func(ctx context.Context) error) error {
	if obs == nil {
		return fn(ctx)
	}
	return runAttempt(ctx, obs, opRegister, service, reason, fn,
		func(ctx context.Context, a Attempt) { obs.RegisterAttempt(ctx, service, reason, a) })
}

// DeregisterAttempt runs one deregistration attempt inside its standard span
// and reports it through obs (see [RegisterAttempt] for the fn/context
// contract). A FAILED deregister leaves the published state untouched — the
// instance may well still be discoverable, and reporting it as gone would hide
// a leak.
func DeregisterAttempt(ctx context.Context, obs Observer, service string, fn func(ctx context.Context) error) error {
	if obs == nil {
		return fn(ctx)
	}
	return runAttempt(ctx, obs, opDeregister, service, "", fn,
		func(ctx context.Context, a Attempt) { obs.DeregisterAttempt(ctx, service, a) })
}

// WeightChange runs one weight re-advertisement inside its standard span and
// reports it through obs — the operation that keeps an instance discoverable
// while it drains, and the one path the discovery core does not log (see
// [RegisterAttempt] for the fn/context contract). It carries no reason: a
// weight change is never a re-register.
func WeightChange(ctx context.Context, obs Observer, service string, fn func(ctx context.Context) error) error {
	if obs == nil {
		return fn(ctx)
	}
	return runAttempt(ctx, obs, opUpdateWeight, service, "", fn,
		func(ctx context.Context, a Attempt) { obs.WeightChange(ctx, service, a) })
}

// runAttempt measures fn inside its span and hands the outcome to record.
func runAttempt(ctx context.Context, obs Observer, op, service, reason string, fn func(context.Context) error, record func(context.Context, Attempt)) error {
	start := obsNow()
	ctx, span := startOp(ctx, obs, op, service, reason)
	return finishAttempt(ctx, func(err error) {
		endOp(span, err)
		record(ctx, Attempt{Status: StatusOf(err), Duration: time.Since(start)})
	}, fn)
}

// RegisterAttempt implements [Observer].
func (o *otelObserver) RegisterAttempt(ctx context.Context, service, reason string, a Attempt) {
	if !o.live() {
		return
	}
	o.ready()
	o.recordDuration(ctx, opRegister, service, a)
	o.regAttempts.Add(ctx, 1, metric.WithAttributes(
		o.attrs(
			attribute.String("service", service),
			attribute.String("reason", reason),
			attribute.String("status", a.Status),
		)...))
	o.setPublished(service, a.Status == "ok")
}

// DeregisterAttempt implements [Observer].
func (o *otelObserver) DeregisterAttempt(ctx context.Context, service string, a Attempt) {
	if !o.live() {
		return
	}
	o.ready()
	o.recordDuration(ctx, opDeregister, service, a)
	if a.Status == "ok" {
		o.setPublished(service, false)
	}
}

// WeightChange implements [Observer].
func (o *otelObserver) WeightChange(ctx context.Context, service string, a Attempt) {
	if !o.live() {
		return
	}
	o.ready()
	o.recordDuration(ctx, opUpdateWeight, service, a)
}

// recordDuration records one operation's duration with the operation/status
// axes.
func (o *otelObserver) recordDuration(ctx context.Context, op, service string, a Attempt) {
	o.opDuration.Record(ctx, a.Duration.Seconds(), metric.WithAttributes(
		o.attrs(
			attribute.String("operation", op),
			attribute.String("service", service),
			attribute.String("status", a.Status),
		)...))
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
func (o *otelObserver) setPublished(service string, ok bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.published[service] = ok
}
