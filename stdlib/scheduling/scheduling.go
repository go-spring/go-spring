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

// Package scheduling defines a framework-agnostic, zero-dependency abstraction
// for running periodic and scheduled background jobs.
//
// It answers one question for a long-running service: "run this piece of work
// on a schedule — every N seconds, N seconds after the last run finished, or on
// a cron expression — for as long as the process lives, and stop it cleanly when
// the process is shutting down." This is the Go-idiomatic equivalent of Spring's
// @Scheduled / TaskScheduler: rather than reproducing that annotation machinery,
// a job is a plain function bound to a [Trigger] and driven by a [Scheduler]
// that participates in the application lifecycle.
//
// The three built-in triggers are:
//
//   - [FixedRate]: fire on a fixed period measured from each scheduled fire time,
//     independent of how long a run takes. Overlap is possible and is governed by
//     the task's [ConcurrencyPolicy].
//   - [FixedDelay]: fire a fixed interval after the previous run *finishes*. Runs
//     never overlap.
//   - [Cron]: fire on a standard 5-field cron expression (see [ParseCron]).
//
// The abstraction is deliberately split from any backend. A [Scheduler] runs
// entirely in-process; when the same job must run on only one replica of a
// multi-replica deployment, attach a distributed lock with [WithLock] (see
// [go-spring.org/stdlib/lock]) so only the lock holder executes each fire.
//
// Cron parsing lives in this package on purpose: keeping it here preserves the
// zero-dependency contract of stdlib and avoids pulling a third-party cron
// library into the foundation layer.
package scheduling

import (
	"context"
	"errors"
	"time"
)

// Job is a unit of scheduled work. It should honour ctx: when ctx is cancelled
// (application shutdown, per-run timeout, or a Replace concurrency policy
// pre-empting it) the job should stop promptly and return. A returned error is
// reported to the scheduler's error handler; it does not stop the schedule.
type Job func(ctx context.Context) error

// Runnable is an object form of [Job] for callers that prefer a method over a
// function value. [AsJob] adapts it to a [Job].
type Runnable interface {
	Run(ctx context.Context) error
}

// AsJob adapts a [Runnable] to a [Job].
func AsJob(r Runnable) Job {
	return func(ctx context.Context) error { return r.Run(ctx) }
}

// TriggerContext carries the timing history a [Trigger] needs to compute the
// next fire time. All times use the scheduler's clock. A zero LastScheduled or
// LastCompletion means the job has not fired yet.
type TriggerContext struct {
	// Now is the current time when the next fire is being computed.
	Now time.Time

	// LastScheduled is the time the previous fire was scheduled for (not when it
	// actually started). Fixed-rate and cron triggers anchor on it so drift does
	// not accumulate.
	LastScheduled time.Time

	// LastCompletion is the time the previous run finished. Fixed-delay anchors
	// on it so the gap is measured from the end of the last run.
	LastCompletion time.Time
}

// Trigger computes when a job should next fire.
type Trigger interface {
	// Next returns the next fire time strictly in the future relative to tc.Now,
	// or the zero time to indicate the job should never fire again.
	Next(tc TriggerContext) time.Time
}

// ConcurrencyPolicy decides what happens when a fixed-rate or cron job is due to
// fire while a previous run of the same job is still executing. It has no effect
// on fixed-delay jobs, which never overlap by construction.
type ConcurrencyPolicy int

const (
	// Skip drops the new fire when a run is already in progress (Kubernetes
	// CronJob "Forbid"). This is the default: it is the safest for jobs that must
	// not run twice at once.
	Skip ConcurrencyPolicy = iota

	// Queue lets at most one fire wait for the current run to finish, then runs
	// it. Additional fires that arrive while one is already queued are dropped.
	Queue

	// Replace cancels the in-flight run (via its context) and starts the new one
	// (Kubernetes CronJob "Replace"). Use it when only the latest run matters.
	Replace
)

// String returns the policy name.
func (p ConcurrencyPolicy) String() string {
	switch p {
	case Skip:
		return "skip"
	case Queue:
		return "queue"
	case Replace:
		return "replace"
	default:
		return "unknown"
	}
}

// Locker acquires a named lock for multi-replica de-duplication. It is a minimal
// interface — just "try to take this key once" — declared here so the package
// keeps its zero-dependency contract. A [go-spring.org/stdlib/lock.Locker] does
// not satisfy it directly (its TryAcquire returns lock.Lock and takes
// lock.Option), so the integration layer adapts one, baking in TTL/renew
// choices; that keeps this abstraction free of the lock package.
type Locker interface {
	// TryAcquire attempts to take key once without blocking. ok reports whether
	// it was acquired; when ok is false the lock is held elsewhere. A non-nil err
	// signals a backend failure distinct from ordinary contention.
	TryAcquire(ctx context.Context, key string) (l Lock, ok bool, err error)
}

// Lock is the subset of a held lock the scheduler uses: it only needs to release
// it. It matches the shape of lock.Lock's Unlock method.
type Lock interface {
	Unlock(ctx context.Context) error
}

// Options controls a single scheduled task. A zero Options is valid; see each
// field for its default.
type Options struct {
	// Policy governs overlapping runs of a fixed-rate/cron job. Defaults to Skip.
	Policy ConcurrencyPolicy

	// Timeout, when positive, bounds a single run: the job's context is cancelled
	// after it elapses. Zero means no per-run timeout.
	Timeout time.Duration

	// Locker and LockKey, when both set, make each fire acquire the named lock
	// first and skip the run if another replica holds it, so a job runs on only
	// one replica at a time.
	Locker  Locker
	LockKey string
}

// Option mutates [Options].
type Option func(*Options)

// WithConcurrencyPolicy sets how overlapping runs are handled.
func WithConcurrencyPolicy(p ConcurrencyPolicy) Option {
	return func(o *Options) { o.Policy = p }
}

// WithTimeout bounds a single run; the job's context is cancelled after d.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

// WithLock makes each fire acquire key on locker first and only run on the
// replica that wins it. It is how a job scheduled on every replica of a
// deployment runs on exactly one at a time.
func WithLock(locker Locker, key string) Option {
	return func(o *Options) {
		o.Locker = locker
		o.LockKey = key
	}
}

// Errors returned by [Scheduler.Schedule].
var (
	// ErrNoTrigger is returned when Schedule is called with a nil trigger. A task
	// with no trigger would never fire, which is almost always a configuration
	// mistake, so it is rejected rather than silently accepted.
	ErrNoTrigger = errors.New("scheduling: nil trigger")

	// ErrNoJob is returned when Schedule is called with a nil job.
	ErrNoJob = errors.New("scheduling: nil job")

	// ErrDuplicateName is returned when Schedule is called with a name already in
	// use on the same scheduler.
	ErrDuplicateName = errors.New("scheduling: duplicate task name")
)

// Scheduler runs jobs against their triggers. Implementations must be safe for
// concurrent use.
type Scheduler interface {
	// Schedule registers a job under a unique name with a trigger. It fails fast
	// on a nil trigger or job (a misconfigured task that could never fire), and
	// on a duplicate name. The returned cancel function removes the task and, if
	// the scheduler is running, stops its loop; it is idempotent.
	//
	// Scheduling before Start records the task; scheduling after Start also
	// launches its loop immediately.
	Schedule(name string, trigger Trigger, job Job, opts ...Option) (cancel func(), err error)

	// Start begins running all registered tasks. It returns once the loops are
	// launched; it does not block. Calling Start twice is a no-op after the
	// first. ctx bounds the lifetime of every task loop.
	Start(ctx context.Context) error

	// Stop halts scheduling of new fires and waits for in-flight runs to finish,
	// bounded by ctx. It returns ctx.Err() if the deadline elapses before every
	// run drains. After Stop the scheduler cannot be restarted.
	Stop(ctx context.Context) error
}
