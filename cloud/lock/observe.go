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

	"go-spring.org/cloud/observability/attrs"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
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
// The observation itself goes through [Observer], whose default is the
// OTel-backed [NewOTelObserver]; the access log stays on the log package.

// lockTag is the static log tag of the lock access log (renders as
// "_app_lock_access"); the backend's system is a log field, not part of the
// tag.
var lockTag = log.RegisterAppTag("lock", "access")

// componentName is the instrumentation componentName name every meter and tracer in this
// package reports under.
const componentName = "go-spring.org/cloud/lock"

// OperationResult is one finished lock operation, as the observer reports it.
type OperationResult struct {
	// Status is ok | missed | error | not_held | lost.
	Status string
	// Duration is the operation's measured duration; for acquire it includes the
	// blocking wait and retry time.
	Duration time.Duration
	// Err is the backend failure, when there was one.
	Err error
}

// Observer receives the lock layer's observability events. It is the seam: the
// decorator reports what happened in the lock's own vocabulary, and an
// implementation decides how to record it — the default is OTel (see
// [NewOTelObserver]). A nil Observer disables observation; implementations must
// not block.
type Observer interface {
	// Operation records one finished lock operation.
	Operation(ctx context.Context, system, operation string, r OperationResult)

	// Held moves the number of locks currently held through this locker: +1 on a
	// successful acquisition, -1 on release or loss.
	Held(ctx context.Context, system string, delta int64)

	// Lost records a lease lost while still in use.
	Lost(ctx context.Context, system string)
}

// NewOTelObserver resolves the layer's instruments and returns the default
// OTel-backed [Observer]. It is registered as the process's default bean.
func NewOTelObserver() (Observer, error) {
	m := otel.Meter(componentName)
	o := &otelObserver{}
	var err error
	o.total, err = m.Int64Counter("lock.operation.total",
		metric.WithDescription("Lock operations executed, by operation and status"),
		metric.WithUnit("{operation}"))
	if err != nil {
		return nil, errutil.Explain(err, "lock: register lock.operation.total")
	}
	o.duration, err = m.Float64Histogram("lock.operation.duration",
		metric.WithDescription("Duration of lock operations; for acquire this includes the blocking wait and retry time"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(attrs.DurationBuckets()...))
	if err != nil {
		return nil, errutil.Explain(err, "lock: register lock.operation.duration")
	}
	o.lost, err = m.Int64Counter("lock.lost.total",
		metric.WithDescription("Locks lost while still in use, because the lease expired or renewal failed"))
	if err != nil {
		return nil, errutil.Explain(err, "lock: register lock.lost.total")
	}
	o.held, err = m.Int64UpDownCounter("lock.held",
		metric.WithDescription("Locks currently held through this locker, by backend"),
		metric.WithUnit("{lock}"))
	if err != nil {
		return nil, errutil.Explain(err, "lock: register lock.held")
	}
	return o, nil
}

// otelObserver adapts the layer's events to OTel metrics. The instruments are
// resolved when the observer is constructed and hold no per-locker state.
type otelObserver struct {
	total    metric.Int64Counter
	duration metric.Float64Histogram
	lost     metric.Int64Counter
	held     metric.Int64UpDownCounter
}

// Operation implements [Observer].
func (o *otelObserver) Operation(ctx context.Context, system, operation string, r OperationResult) {
	attrsOpt := metric.WithAttributes(
		attribute.String("system", system),
		attribute.String("operation", operation),
		attribute.String("status", r.Status),
	)
	o.total.Add(ctx, 1, attrsOpt)
	o.duration.Record(ctx, r.Duration.Seconds(), attrsOpt)
}

// Held implements [Observer].
func (o *otelObserver) Held(ctx context.Context, system string, delta int64) {
	o.held.Add(ctx, delta, metric.WithAttributes(attribute.String("system", system)))
}

// Lost implements [Observer].
func (o *otelObserver) Lost(ctx context.Context, system string) {
	o.lost.Add(ctx, 1, metric.WithAttributes(attribute.String("system", system)))
}

// ObserveWith decorates inner with observability, labelled with the backend's
// system value. A nil observer disables observation.
func ObserveWith(inner Locker, system string, obs Observer) Locker {
	return &observedLocker{system: system, inner: inner, obs: obs}
}

type observedLocker struct {
	system string
	inner  Locker
	obs    Observer
}

// begin opens the span for one lock operation. Span handling is the STANDARD
// OTel capability, not part of the metric seam.
func (l *observedLocker) begin(ctx context.Context, op, key string) (context.Context, trace.Span) {
	return otel.Tracer(componentName).Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("lock.system", l.system),
			attribute.String("lock.operation", op),
			attribute.String("lock.key", key),
		))
}

// end finishes the operation's span and reports its metric event. A nil
// observer disables the metric.
func (l *observedLocker) end(ctx context.Context, span trace.Span, op string, r OperationResult) {
	span.SetAttributes(attribute.String("status", r.Status))
	if r.Err != nil {
		span.SetStatus(codes.Error, r.Err.Error())
	}
	span.End()
	if l.obs != nil {
		l.obs.Operation(ctx, l.system, op, r)
	}
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

// record writes the access log for one finished operation. The log level
// carries the same outcome: an error or a lost lease at Warn, a contended
// TryAcquire at Info, a plain success at Debug (lock acquisition is frequent and
// uninteresting until it fails or misses).
func (l *observedLocker) record(ctx context.Context, op, key, status string, start time.Time, err error) {
	elapsed := time.Since(start)
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
		log.Warn(ctx, lockTag, "lock operation failed", append(fields(), log.Err(err))...)
	case "missed":
		log.Info(ctx, lockTag, "lock acquisition missed", fields()...)
	default:
		log.Debug(ctx, lockTag, "lock operation finished", fields)
	}
}

func (l *observedLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	ctx, span := l.begin(ctx, "acquire", key)
	start := time.Now()
	held, err := l.inner.Acquire(ctx, key, opts...)
	status := statusOf(err, err == nil)
	l.end(ctx, span, "acquire", OperationResult{Status: status, Duration: time.Since(start), Err: err})
	l.record(ctx, "acquire", key, status, start, err)
	if err != nil || held == nil {
		return held, err
	}
	// Acquiring is only half the lock's life: the caller holds the returned
	// handle for as long as the work runs, and the interesting failure — the
	// lease going away underneath it — happens after this returns.
	return l.observe(ctx, held, key), nil
}

func (l *observedLocker) TryAcquire(ctx context.Context, key string, opts ...Option) (Lock, bool, error) {
	ctx, span := l.begin(ctx, "try_acquire", key)
	start := time.Now()
	held, ok, err := l.inner.TryAcquire(ctx, key, opts...)
	status := statusOf(err, err == nil && ok)
	l.end(ctx, span, "try_acquire", OperationResult{Status: status, Duration: time.Since(start), Err: err})
	l.record(ctx, "try_acquire", key, status, start, err)
	if err != nil || !ok || held == nil {
		return held, ok, err
	}
	return l.observe(ctx, held, key), ok, nil
}

func (l *observedLocker) Close() error { return l.inner.Close() }

// observe wraps a held lock so its release and its loss are reported through
// the same system the acquisition was.
func (l *observedLocker) observe(ctx context.Context, inner Lock, key string) Lock {
	if l.obs != nil {
		l.obs.Held(ctx, l.system, 1)
	}
	// The loss is reported from a goroutine that outlives the call that
	// acquired the lock, so the caller's context is long done by then. Carry
	// the acquisition's span context forward so the loss lands in the same
	// trace, but nothing else: rebuilding it on Background avoids pinning the
	// caller's request-scoped values (or passing a cancelled ctx around) for as
	// long as the lock is held.
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
	if firstUnlock && h.locker.obs != nil {
		h.locker.obs.Held(ctx, h.locker.system, -1)
	}

	ctx, span := h.locker.begin(ctx, "unlock", h.key)
	start := time.Now()
	err := h.inner.Unlock(ctx)
	status := statusOf(err, err == nil)
	h.locker.end(ctx, span, "unlock", OperationResult{Status: status, Duration: time.Since(start), Err: err})
	h.locker.record(ctx, "unlock", h.key, status, start, err)
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
		log.Debug(h.ctx, lockTag, "lock released", func() []log.Field {
			return append(fields(), log.String("status", "resigned"))
		})
		return
	}

	if h.locker.obs != nil {
		h.locker.obs.Held(h.ctx, h.locker.system, -1)
		h.locker.obs.Lost(h.ctx, h.locker.system)
	}
	log.Warn(h.ctx, lockTag, "lock lost while still held", append(fields(),
		log.String("status", "lost"))...)
}
