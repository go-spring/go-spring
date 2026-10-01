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
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/observability"
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

// lockTag is the static log tag of the lock access log (renders as
// "_app_lock_access"); the backend's system is a log field, not part of the
// tag.
var lockTag = log.RegisterAppTag("lock", "access")

// instrumentSet is this package's metric set: one per process, resolved lazily
// on first use so it binds to whichever meter provider is current then, and
// immutable afterwards. It holds no per-locker state — the system label travels
// with each wrapper, not here.
type instrumentSet struct {
	total    metric.Int64Counter
	duration metric.Float64Histogram
	lost     metric.Int64Counter
	held     metric.Int64UpDownCounter
}

// instruments is the one instrument set this package uses for the whole process.
var instruments = sync.OnceValue(buildInstruments)

func buildInstruments() *instrumentSet {
	m := otel.Meter("go-spring.org/cloud/lock")
	in := &instrumentSet{}
	in.total, _ = m.Int64Counter("lock.operation.total",
		metric.WithDescription("Lock operations executed, by operation and status"),
		metric.WithUnit("{operation}"))
	in.duration, _ = m.Float64Histogram("lock.operation.duration",
		metric.WithDescription("Duration of lock operations; for acquire this includes the blocking wait and retry time"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(observability.DurationBuckets()...))
	in.lost, _ = m.Int64Counter("lock.lost.total",
		metric.WithDescription("Locks lost while still in use, because the lease expired or renewal failed"))
	in.held, _ = m.Int64UpDownCounter("lock.held",
		metric.WithDescription("Locks currently held through this locker, by backend"),
		metric.WithUnit("{lock}"))
	return in
}

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

// Observe returns a [Locker] that decorates inner with a client span per
// operation, the lock.operation.total, lock.operation.duration,
// lock.lost.total and lock.held metrics, and an access log, labelled with the
// backend's system value. When starter-otel is not imported the global OTel
// providers are no-ops, so the wrapper adds negligible overhead and changes no
// behaviour.
//
// Instruments come from the package's process-wide set, resolved on first use —
// here, at wiring time, not at package init — so an SDK installed after this
// package's init still receives the records. The tracer is looked up per use for
// the same reason.
//
// A lock starter installs it with its backend's system value:
//
//	locker = lock.Observe(inner, "redis")
func Observe(inner Locker, system string) Locker {
	return &observedLocker{
		system: system,
		inner:  inner,
		ins:    instruments(),
	}
}

// tracerName names the tracer the operation spans open on. The tracer is
// looked up per use (otel.Tracer at call time), never cached in a field or
// package variable: one captured before any provider is set stops forwarding
// once the global provider is set, unset and set again.
const tracerName = "go-spring.org/cloud/lock"

type observedLocker struct {
	system string
	inner  Locker

	// ins is the shared instrument set; the tracer is deliberately NOT held
	// alongside it (see [tracerName]).
	ins *instrumentSet
}

// startSpan opens the operation's client span.
func (l *observedLocker) startSpan(ctx context.Context, op, key string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, op,
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
	attrs := metric.WithAttributes(
		attribute.String("system", l.system),
		attribute.String("operation", op),
		attribute.String("status", status),
	)
	l.ins.total.Add(ctx, 1, attrs)
	l.ins.duration.Record(ctx, elapsed.Seconds(), attrs)

	fields := func() []log.Field {
		return []log.Field{
			log.String("system", l.system),
			log.String("operation", op),
			log.String("lock.key", key),
			log.String("status", status),
			log.Float("duration_ms", float64(elapsed.Nanoseconds())/1e6),
		}
	}
	switch status {
	case "error", "not_held":
		log.Warn(ctx, lockTag, append(fields(), log.Err(err), log.Msg("lock operation failed"))...)
	case "missed":
		log.Info(ctx, lockTag, fields()...)
	default:
		log.Debug(ctx, lockTag, fields)
	}
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	start := time.Now()
	ctx, span := l.startSpan(ctx, "acquire", key)
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
	ctx, span := l.startSpan(ctx, "try_acquire", key)
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
	l.ins.held.Add(ctx, 1, metric.WithAttributes(
		attribute.String("system", l.system),
	))
	// The loss is reported from a goroutine that outlives the call that
	// acquired the lock, so the caller's context is long done by then. Carry
	// the acquisition's span context forward so the loss lands in the same
	// trace, but nothing else: rebuilding it on Background avoids pinning the
	// caller's request-scoped values (or passing a cancelled ctx around) for
	// as long as the lock is held.
	// spanCtx is just the trace identity; lossCtx is the detached context the
	// loss goroutine reports under — same trace, no request-scoped baggage.
	spanCtx := trace.SpanContextFromContext(ctx)
	lossCtx := trace.ContextWithSpanContext(context.Background(), spanCtx)
	h := &observedLock{
		locker:   l,
		inner:    inner,
		key:      key,
		lost:     inner.Lost(),
		acquired: time.Now(),
		ctx:      lossCtx,
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
	// a silent loss. The Swap return halves the held gauge exactly once per
	// handle — repeated Unlock calls must not under-count it.
	firstUnlock := !h.released.Swap(true)
	if firstUnlock {
		h.locker.ins.held.Add(ctx, -1, metric.WithAttributes(
			attribute.String("system", h.locker.system),
		))
	}

	start := time.Now()
	ctx, span := h.locker.startSpan(ctx, "unlock", h.key)
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
			log.String("lock.key", h.key),
			log.Float("held_ms", float64(time.Since(h.acquired).Nanoseconds())/1e6),
		}
	}
	if h.released.Load() {
		log.Debug(h.ctx, lockTag, func() []log.Field {
			return append(fields(), log.String("status", "resigned"))
		})
		return
	}

	h.locker.ins.held.Add(h.ctx, -1, metric.WithAttributes(
		attribute.String("system", h.locker.system),
	))
	h.locker.ins.lost.Add(h.ctx, 1, metric.WithAttributes(
		attribute.String("system", h.locker.system),
	))
	log.Warn(h.ctx, lockTag, append(fields(),
		log.String("status", "lost"),
		log.Msg("lock lost while still held"))...)
}
