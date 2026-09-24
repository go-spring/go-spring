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
	"fmt"
	"maps"
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName is the OTel scope all registry/discovery instruments are
// built from.
const instrumentationName = "go-spring.org/cloud/discovery"

// Registration reasons a backend passes to [RegisterAttempt]: the first
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

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set, shared with the other domain packages.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// obsNow reads the wall clock. A variable so tests can advance time and assert
// that a stale snapshot's age actually climbs.
var obsNow = time.Now

// obsKey identifies one (backend, service) pair in the state the observable
// gauges read.
type obsKey struct{ system, service string }

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

var (
	// obsOnce resolves the instruments and registers the observable gauges
	// exactly once per process.
	obsOnce sync.Once

	opDuration  metric.Float64Histogram
	regAttempts metric.Int64Counter
	syncTotal   metric.Int64Counter

	// obsMu guards the state maps below. The gauge callbacks snapshot them
	// instead of observing under the lock: OTel advises against locking inside
	// a callback (Go mutexes are not reentrant, and collection is concurrent).
	obsMu     sync.Mutex
	published = map[obsKey]bool{}
	syncs     = map[obsKey]*syncState{}
)

// instruments resolves the synchronous instruments and registers the
// process-wide observable gauges. Resolution happens on first use — after
// starter-otel has installed the global providers — not at package init, so
// records reach whichever SDK is current. Without starter-otel the globals are
// no-ops and the whole file costs a couple of map lookups.
func instruments() {
	obsOnce.Do(func() {
		m := otel.Meter(instrumentationName)
		opDuration, _ = m.Float64Histogram("registry.operation.duration",
			metric.WithDescription("Duration of registry-center operations"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(durationBuckets...))
		regAttempts, _ = m.Int64Counter("registry.registration.attempts_total",
			metric.WithDescription("Registration attempts against a registry center, by reason and outcome"))
		syncTotal, _ = m.Int64Counter("discovery.sync_total",
			metric.WithDescription("Background endpoint-cache syncs, by outcome"))

		_, _ = m.Int64ObservableGauge("registry.instance.registered",
			metric.WithDescription("1 while this instance is published into the registry center, 0 while it is not"),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				obsMu.Lock()
				snapshot := make(map[obsKey]bool, len(published))
				maps.Copy(snapshot, published)
				obsMu.Unlock()
				for k, ok := range snapshot {
					var v int64
					if ok {
						v = 1
					}
					o.Observe(v, metric.WithAttributes(
						attribute.String("system", k.system),
						attribute.String("service", k.service),
					))
				}
				return nil
			}))

		_, _ = m.Float64ObservableGauge("discovery.cache.age_seconds",
			metric.WithDescription("Seconds since the cached endpoint snapshot for this service was last confirmed fresh"),
			metric.WithUnit("s"),
			metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
				now := obsNow()
				obsMu.Lock()
				snapshot := make(map[obsKey]time.Duration, len(syncs))
				for k, s := range syncs {
					snapshot[k] = s.ageAt(now)
				}
				obsMu.Unlock()
				for k, age := range snapshot {
					o.Observe(age.Seconds(), metric.WithAttributes(
						attribute.String("system", k.system),
						attribute.String("service", k.service),
					))
				}
				return nil
			}))
	})
}

// SyncFields returns the identity fields a per-sync log line has to carry to be
// joinable, BY FIELD, to the discovery metrics and spans for the same service:
// the keys are the metric attribute names, so a dashboard can select the failed
// syncs and land on the lines that explain them.
//
// It lives here rather than in each backend for the same reason one status
// value drives both the metric and the log in cloud/lock: keys spelled out at
// every call site drift, and a drifted key is a silent one — the log still
// looks right and simply never joins anything.
//
// The caller adds its own message and its own detail (the key, the action
// taken); this covers only what identifies the operation.
func SyncFields(system, service string) []log.Field {
	return []log.Field{
		log.String("system", system),
		log.String("service", service),
		log.String("operation", "sync"),
	}
}

// SyncFailedFields returns the fields a FAILED per-sync log line carries:
// [SyncFields]' identity, the outcome, and the error itself. The status comes
// from the same [statusOf] the metric uses, so the log can never report an
// outcome the counter disagrees with.
//
// The caller still calls log.Warn/Error itself and supplies its own message —
// that keeps the line's caller attribution, which a wrapper would collapse onto
// the wrapper's own file and line.
func SyncFailedFields(system, service string, err error) []log.Field {
	return append(SyncFields(system, service),
		log.String("status", statusOf(err)),
		log.Err(err),
	)
}

// RegisterFields returns the identity fields a per-registration log line has to
// carry — the keys RegisterAttempt puts on the span and the counter for the same
// operation, so the line joins them (see [SyncFields] for why the keys live
// here). reason separates the initial publish from a self-healing re-publish,
// which is the distinction an operator alerts on.
func RegisterFields(system, service, reason string) []log.Field {
	return []log.Field{
		log.String("system", system),
		log.String("service", service),
		log.String("operation", opRegister),
		log.String("reason", reason),
	}
}

// RegisterFailedFields adds the outcome and the error to [RegisterFields].
func RegisterFailedFields(system, service, reason string, err error) []log.Field {
	return append(RegisterFields(system, service, reason),
		log.String("status", statusOf(err)),
		log.Err(err),
	)
}

// statusOf names an outcome the way the metric attributes expect.
func statusOf(err error) string {
	if err != nil {
		return "failed"
	}
	return "ok"
}

// startOp opens the client span for one registry-center operation. The span
// attributes are namespaced (registry.*) while the metric attributes are bare,
// matching the other domain packages' instrumentation.
func startOp(ctx context.Context, system, op, service, reason string) (context.Context, trace.Span) {
	instruments()
	attrs := []attribute.KeyValue{
		attribute.String("registry.system", system),
		attribute.String("registry.operation", op),
		attribute.String("registry.service", service),
	}
	if reason != "" {
		attrs = append(attrs, attribute.String("registry.reason", reason))
	}
	return otel.Tracer(instrumentationName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...))
}

// endOp closes the span and records the operation's duration.
func endOp(ctx context.Context, span trace.Span, system, op, service string, start time.Time, err error) {
	instruments()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	opDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String("system", system),
		attribute.String("operation", op),
		attribute.String("service", service),
		attribute.String("status", statusOf(err)),
	))
}

// RegisterAttempt runs and reports one registration attempt against a registry
// center: fn executes the attempt, and its error becomes the attempt's outcome.
// It returns fn's error unchanged so the caller keeps its normal error flow.
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
func RegisterAttempt(ctx context.Context, system, service, reason string, fn func(ctx context.Context) error) error {
	start := obsNow()
	ctx, span := startOp(ctx, system, opRegister, service, reason)
	return finishAttempt(ctx, func(err error) {
		endOp(ctx, span, system, opRegister, service, start, err)
		regAttempts.Add(ctx, 1, metric.WithAttributes(
			attribute.String("system", system),
			attribute.String("service", service),
			attribute.String("reason", reason),
			attribute.String("status", statusOf(err)),
		))
		setPublished(system, service, err == nil)
	}, fn)
}

// DeregisterAttempt runs and reports one deregistration attempt (see
// [RegisterAttempt] for the fn/context contract). Unlike registration, a FAILED
// deregister leaves the published state untouched — the instance may well still
// be discoverable, and reporting it as gone would hide a leak.
func DeregisterAttempt(ctx context.Context, system, service string, fn func(ctx context.Context) error) error {
	start := obsNow()
	ctx, span := startOp(ctx, system, opDeregister, service, "")
	return finishAttempt(ctx, func(err error) {
		endOp(ctx, span, system, opDeregister, service, start, err)
		if err == nil {
			setPublished(system, service, false)
		}
	}, fn)
}

// WeightChange runs and reports one weight re-advertisement — the operation
// that keeps an instance discoverable while it drains, and the one path the
// registry core does not log (see [RegisterAttempt] for the fn/context
// contract). It carries no reason: a weight change is never a re-register.
func WeightChange(ctx context.Context, system, service string, fn func(ctx context.Context) error) error {
	start := obsNow()
	ctx, span := startOp(ctx, system, opUpdateWeight, service, "")
	return finishAttempt(ctx, func(err error) {
		endOp(ctx, span, system, opUpdateWeight, service, start, err)
	}, fn)
}

// finishAttempt runs fn, reports the outcome through end, and returns fn's
// error unchanged. A panic inside fn is reported as a failed attempt before it
// propagates, so an aborted attempt never leaves the span and metrics dangling.
func finishAttempt(ctx context.Context, end func(err error), fn func(ctx context.Context) error) error {
	var err error
	defer func() {
		if p := recover(); p != nil {
			end(fmt.Errorf("panic: %v", p))
			panic(p)
		}
		end(err)
	}()
	err = fn(ctx)
	return err
}

// Synced reports the outcome of one background endpoint-cache sync for service.
//
// It is metric-only by design: the sync loop is a high-frequency background
// path (a span per sync would be noise), and the backend already logs failures
// itself. Only a successful sync advances freshness, so a watch that has died
// shows up as a steadily climbing discovery.cache.age_seconds rather than a
// value frozen at zero — the failure mode where discovery keeps handing out
// stale addresses that look perfectly healthy.
func Synced(system, service string, err error) {
	instruments()
	ctx := context.Background()
	syncTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("system", system),
		attribute.String("service", service),
		attribute.String("status", statusOf(err)),
	))

	now := obsNow()
	key := obsKey{system, service}
	obsMu.Lock()
	defer obsMu.Unlock()
	s := syncs[key]
	if s == nil {
		s = &syncState{origin: now}
		syncs[key] = s
	}
	if err == nil {
		s.last = now
	}
}

// setPublished records whether the instance is currently published.
//
// The maps here are keyed by (system, service) and live for the process: a
// registry core publishes exactly one service identity per process, so the
// cardinality is the number of configured centers, plus one entry per resolved
// service name on the discovery side.
func setPublished(system, service string, ok bool) {
	obsMu.Lock()
	defer obsMu.Unlock()
	published[obsKey{system, service}] = ok
}
