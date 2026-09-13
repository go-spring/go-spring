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

package lock

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Observability contract.
//
// A Locker is observed by decorating the interface, so no backend carries
// instrumentation of its own and a service can switch backend — or use several
// at once — without changing what gets reported.
//
// Every signal shares one vocabulary, so a metric, a span and a log line always
// agree:
//
//	operation  acquire | try_acquire | unlock
//	status     ok | missed | error | not_held | lost
//
// A backend's own telemetry (Redis commands, etcd revisions, apiserver
// round-trips) is deliberately not mirrored here. The wrapper builds the parent
// span and passes its context down, so whatever instrumentation the client
// library ships nests underneath it in the same trace.
//
// Two invariants:
//   - lock.key is a caller-chosen resource name. It belongs on spans and in
//     logs, never on a metric, where its cardinality is unbounded.
//   - Every backend closes the lost channel on a voluntary release as well as
//     on a real loss, so the two are told apart by whether Unlock ran first.

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// lockTag is the static log tag of the lock access log (renders as
// "_app_lock_access"); the backend's system is a log field, not part of the
// tag.
var lockTag = log.RegisterAppTag("lock", "access")

// instruments bundles the metrics the wrapper records. They are built from
// whatever meter provider is current — created per Wrap, not at package init,
// so an SDK installed later than this package's init still receives the
// records.
type instruments struct {
	duration metric.Float64Histogram
	lost     metric.Int64Counter
}

func newInstruments() instruments {
	m := otel.Meter("go-spring.org/cloud/lock")
	duration, _ := m.Float64Histogram("lock.operation.duration",
		metric.WithDescription("Duration of lock operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	lost, _ := m.Int64Counter("lock.lost.total",
		metric.WithDescription("Locks lost while still in use, because the lease expired or renewal failed"))
	return instruments{duration: duration, lost: lost}
}

// WrapLocker returns a [Locker] that decorates inner with a client span per
// operation, the lock.operation.duration and lock.lost.total metrics, and an
// access log, labelled with the backend's system value. When starter-otel is
// not imported the global OTel providers are no-ops, so the wrapper adds
// negligible overhead and changes no behaviour.
//
// Tracer and instruments are resolved from whatever OTel providers are current
// — here, at wiring time, not at package init — so an SDK installed after this
// package's init still receives the spans and records.
//
// A lock starter installs it with its backend's system value:
//
//	locker = lock.WrapLocker("redis", inner)
func WrapLocker(system string, inner Locker) Locker {
	return &observedLocker{
		system:      system,
		inner:       inner,
		tracer:      otel.Tracer("go-spring.org/cloud/lock"),
		instruments: newInstruments(),
	}
}

type observedLocker struct {
	system      string
	inner       Locker
	tracer      trace.Tracer
	instruments instruments
}

// start opens the operation's client span.
func (l *observedLocker) start(ctx context.Context, op, key string) (context.Context, trace.Span) {
	return l.tracer.Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lock.system", l.system),
			attribute.String("lock.operation", op),
			attribute.String("lock.key", key),
		))
}

// statusOf maps an outcome to the status dimension every signal shares.
// ErrNotHeld is a status of its own: releasing a lock that was taken over is
// neither a success nor a backend failure, and conflating it with an error
// would hide the one event a distributed lock exists to prevent.
func statusOf(err error, acquired bool) string {
	switch {
	case errors.Is(err, ErrNotHeld):
		return "not_held"
	case err != nil:
		return "error"
	case !acquired:
		return "missed"
	default:
		return "ok"
	}
}

// record emits the duration metric and the access log for one finished
// operation. One status value drives both, so the metric can never disagree
// with the log. The log level carries the same outcome: an error or a lost
// lease at Warn, a contended TryAcquire at Info, a plain success at Debug (lock
// acquisition is frequent and uninteresting until it fails or misses).
func (l *observedLocker) record(ctx context.Context, op, key, status string, start time.Time, err error) {
	elapsed := time.Since(start)
	l.instruments.duration.Record(ctx, elapsed.Seconds(), metric.WithAttributes(
		attribute.String("system", l.system),
		attribute.String("operation", op),
		attribute.String("status", status),
	))

	fields := func() []log.Field {
		return []log.Field{
			log.String("system", l.system),
			log.String("operation", op),
			log.String("key", key),
			log.String("status", status),
			log.Float("duration_ms", float64(elapsed.Nanoseconds())/1e6),
		}
	}
	switch status {
	case "error", "not_held":
		log.Warn(ctx, lockTag, append(fields(), log.Any("error", err))...)
	case "missed":
		log.Info(ctx, lockTag, fields()...)
	default:
		log.Debug(ctx, lockTag, fields)
	}
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	start := time.Now()
	ctx, span := l.start(ctx, "acquire", key)
	held, err := l.inner.Acquire(ctx, key, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	l.record(ctx, "acquire", key, statusOf(err, err == nil), start, err)
	if err != nil || held == nil {
		return held, err
	}
	// Acquiring is only half the lock's life: the caller holds the returned
	// handle for as long as the work runs, and the interesting failure — the
	// lease going away underneath it — happens after this returns.
	return l.observe(ctx, held, key), nil
}

func (l *observedLocker) TryAcquire(ctx context.Context, key string, opts ...Option) (Lock, bool, error) {
	start := time.Now()
	ctx, span := l.start(ctx, "try_acquire", key)
	held, ok, err := l.inner.TryAcquire(ctx, key, opts...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	l.record(ctx, "try_acquire", key, statusOf(err, err == nil && ok), start, err)
	if err != nil || !ok || held == nil {
		return held, ok, err
	}
	return l.observe(ctx, held, key), ok, nil
}

func (l *observedLocker) Close() error { return l.inner.Close() }

// observe wraps a held lock so its release and its loss are reported through
// the same system the acquisition was.
func (l *observedLocker) observe(ctx context.Context, inner Lock, key string) Lock {
	h := &observedLock{
		locker:   l,
		inner:    inner,
		key:      key,
		lost:     inner.Lost(),
		acquired: time.Now(),
		// The loss is reported from a goroutine that outlives the call that
		// acquired the lock, so the caller's context is long done by then.
		// Carry the acquisition's span context forward so the loss lands in the
		// same trace, but nothing else: rebuilding it from Background avoids
		// pinning the caller's request-scoped values for as long as the lock is
		// held.
		ctx: trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx)),
	}
	// Watching a channel costs one goroutine per held lock, which is the price
	// of reporting a loss at the moment it happens. A backend whose lease can
	// never be lost returns nil, and there is nothing to watch.
	if h.lost != nil {
		go h.reportLost()
	}
	return h
}

// observedLock is the handle [observedLocker] hands back: identical to the
// backend's own, with Unlock and the loss of the lease reported as they happen.
type observedLock struct {
	locker   *observedLocker
	inner    Lock
	key      string
	lost     <-chan struct{}
	ctx      context.Context
	acquired time.Time

	// released is set by Unlock before it calls the backend. It is what
	// separates the normal end of a lock's life from a real loss, since every
	// backend closes the lost channel for both.
	released atomic.Bool
}

func (h *observedLock) Key() string           { return h.inner.Key() }
func (h *observedLock) Token() string         { return h.inner.Token() }
func (h *observedLock) Lost() <-chan struct{} { return h.lost }

func (h *observedLock) Unlock(ctx context.Context) error {
	// Set before the call: a release that fails is still a release attempt, not
	// a silent loss.
	h.released.Store(true)

	start := time.Now()
	ctx, span := h.locker.start(ctx, "unlock", h.key)
	err := h.inner.Unlock(ctx)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
	h.locker.record(ctx, "unlock", h.key, statusOf(err, err == nil), start, err)
	return err
}

// reportLost runs once, when the backend closes the lost channel. A close that
// follows Unlock is the ordinary end of a lock's life, so it stays a Debug log
// line and is not counted: reporting it would bury the real losses, one per
// term of every election.
func (h *observedLock) reportLost() {
	<-h.lost

	fields := func() []log.Field {
		return []log.Field{
			log.String("system", h.locker.system),
			log.String("key", h.key),
			log.Float("held_ms", float64(time.Since(h.acquired).Nanoseconds())/1e6),
		}
	}
	if h.released.Load() {
		log.Debug(h.ctx, lockTag, func() []log.Field {
			return append(fields(), log.String("status", "resigned"))
		})
		return
	}

	h.locker.instruments.lost.Add(h.ctx, 1, metric.WithAttributes(
		attribute.String("system", h.locker.system),
	))
	log.Warn(h.ctx, lockTag, append(fields(), log.String("status", "lost"))...)
}
