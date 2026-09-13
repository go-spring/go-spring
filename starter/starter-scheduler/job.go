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

package StarterScheduler

import (
	"context"
	"fmt"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/cloud/scheduling"
	"go-spring.org/spring/gs"
)

// Job is the seam between the application and the scheduler: one Job bean per
// unit of work, carrying both what to run ([Job.Run]) and when to run it
// ([Job.Trigger], [Job.Spec]). This mirrors the "app owns the work, starter owns
// the runner" split used by the server starters — with the schedule declared
// next to the work it describes, rather than in a config file keyed by name.
//
// Register a Job with [Provide]:
//
//	scheduler.Provide("cleanup", svc.Cleanup, scheduler.Every(5*time.Minute))
type Job interface {
	// JobName is the job's name: the scheduler's task name, and the default lock
	// key.
	JobName() string
	// Trigger says when the job fires. It is never nil — a job without a trigger
	// cannot be registered.
	Trigger() scheduling.Trigger
	// Spec carries the execution options declared at registration.
	Spec() JobSpec
	// Run performs the work. It should honour ctx for cancellation.
	Run(ctx context.Context) error
}

// JobSpec is the scheduling behaviour a job declares when it is registered. Its
// zero value is valid: no timeout, the default concurrency policy, no
// cross-replica lock.
type JobSpec struct {
	// Timeout, when positive, bounds a single run: the job's context is cancelled
	// after it elapses.
	Timeout time.Duration

	// Concurrency governs overlapping fixed-rate/cron runs: [scheduling.Skip]
	// (the zero value), [scheduling.Queue] or [scheduling.Replace]. It has no
	// effect on an [After] job, which never overlaps.
	Concurrency scheduling.ConcurrencyPolicy

	// Lock names a lock.Locker bean (contributed by starter-lock-{redis,etcd,consul})
	// that de-duplicates this job across replicas: each fire acquires the lock and
	// only the holder runs. Empty runs the job on every replica.
	Lock string

	// LockKey is the key acquired on the locker; empty means the job name, so two
	// jobs sharing a locker do not collide.
	LockKey string

	// LockTTL is the lease duration for the acquired lock. Zero keeps the locker's
	// own default.
	LockTTL time.Duration
}

// JobOption declares one aspect of a job at registration. The trigger options
// ([Every], [After], [Cron]) are the ones a job cannot be registered without.
type JobOption func(*namedJob)

// Every fires the job every d, measured from each scheduled fire time, so a slow
// run does not push later fires back. Runs may overlap; use [WithConcurrency] to
// say what happens when they do. The first fire is one interval after the
// scheduler starts. It panics — at registration, i.e. during startup — if d is
// not positive.
func Every(d time.Duration) JobOption {
	return trigger(scheduling.FixedRate(d))
}

// After fires the job d after the previous run finishes, so two runs never
// overlap. It panics if d is not positive.
func After(d time.Duration) JobOption {
	return trigger(scheduling.FixedDelay(d))
}

// Cron fires the job on a standard 5-field cron expression (see
// [scheduling.ParseCron]). There is no seconds field — sub-minute cadence is what
// [Every] and [After] are for. It panics on an unparsable expression.
func Cron(expr string) JobOption {
	tr, err := scheduling.ParseCron(expr)
	if err != nil {
		// The error already names the scheduling package it came from.
		panic(err)
	}
	return trigger(tr)
}

// trigger is the body the three trigger options share. Applying two triggers to
// one job is a mistake options cannot catch structurally, so it panics at
// registration rather than letting the last one silently win.
func trigger(tr scheduling.Trigger) JobOption {
	return func(j *namedJob) {
		if j.trigger != nil {
			panic(fmt.Sprintf("scheduler: job %q sets more than one trigger", j.name))
		}
		j.trigger = tr
	}
}

// WithTimeout bounds a single run: the job's context is cancelled after d.
func WithTimeout(d time.Duration) JobOption {
	return func(j *namedJob) { j.spec.Timeout = d }
}

// WithConcurrency sets how overlapping runs of a fixed-rate/cron job are handled.
func WithConcurrency(p scheduling.ConcurrencyPolicy) JobOption {
	return func(j *namedJob) { j.spec.Concurrency = p }
}

// WithLock de-duplicates the job across replicas with the named lock.Locker bean:
// each fire acquires the lock and only the holder runs. A bean *name* is enough
// here because the bean does not exist yet at registration time — the scheduler
// resolves it when it starts. The key defaults to the job name; see [WithLockKey]
// and [WithLockTTL].
func WithLock(bean string) JobOption {
	return func(j *namedJob) { j.spec.Lock = bean }
}

// WithLockKey overrides the key acquired on the locker. Give jobs that share a
// locker distinct keys.
func WithLockKey(key string) JobOption {
	return func(j *namedJob) { j.spec.LockKey = key }
}

// WithLockTTL sets the lease duration for the acquired lock. It should exceed a
// typical run so the lease is not lost mid-run; the lock package auto-renews it
// while the job holds it. Zero — the default — keeps the locker's own default.
func WithLockTTL(ttl time.Duration) JobOption {
	return func(j *namedJob) { j.spec.LockTTL = ttl }
}

// NewJob builds a [Job] bean from a name, a function and its schedule. It panics
// on an empty name, a nil function, or a missing or duplicated trigger: each of
// those would otherwise become a job that silently never fires.
//
// NewJob returns the [Job] value only; callers who use it directly must register
// it with the container themselves, naming the bean and exporting it as Job so
// the scheduler can collect it:
//
//	gs.Provide(scheduler.NewJob("cleanup", svc.Cleanup, scheduler.Every(time.Minute))).
//	    Name("cleanup").Export(gs.As[scheduler.Job]())
//
// Prefer [Provide], which does exactly that in one call.
func NewJob(name string, fn func(ctx context.Context) error, opts ...JobOption) Job {
	if name == "" {
		panic("scheduler: job name must not be empty")
	}
	if fn == nil {
		panic("scheduler: job function must not be nil")
	}
	j := &namedJob{name: name, fn: fn}
	for _, o := range opts {
		o(j)
	}
	if j.trigger == nil {
		panic(fmt.Sprintf("scheduler: job %q has no trigger; pass Every, After or Cron", name))
	}
	return j
}

// Provide registers a scheduled job in one call: it wraps fn in a [Job] bean,
// names the bean after the job, and exports it as Job so the scheduler collects
// it. The schedule is declared here, beside the work it describes:
//
//	scheduler.Provide("cleanup", svc.Cleanup, scheduler.Every(5*time.Minute))
//
// It panics on a bad name, a nil function or a missing/duplicated trigger (see
// [NewJob]). For advanced cases that need conditions or lifecycle hooks, register
// with [NewJob] and gs.Provide directly.
func Provide(name string, fn func(ctx context.Context) error, opts ...JobOption) {
	gs.Provide(NewJob(name, fn, opts...)).Name(name).Export(gs.As[Job]())
}

// namedJob is the [Job] implementation [NewJob] returns.
type namedJob struct {
	name    string
	fn      func(ctx context.Context) error
	trigger scheduling.Trigger
	spec    JobSpec
}

func (j *namedJob) JobName() string               { return j.name }
func (j *namedJob) Trigger() scheduling.Trigger   { return j.trigger }
func (j *namedJob) Spec() JobSpec                 { return j.spec }
func (j *namedJob) Run(ctx context.Context) error { return j.fn(ctx) }

// lockerAdapter adapts a lock.Locker to the minimal scheduling.Locker the
// scheduler needs, baking in the lease options so the scheduler abstraction
// stays free of the lock package. The lock is auto-renewed by the lock package
// while held, so a long-running job keeps the lease.
type lockerAdapter struct {
	l    lock.Locker
	opts []lock.Option
}

func (a lockerAdapter) TryAcquire(ctx context.Context, key string) (scheduling.Lock, bool, error) {
	l, ok, err := a.l.TryAcquire(ctx, key, a.opts...)
	if !ok || l == nil {
		return nil, ok, err
	}
	return l, true, nil
}

func lockTTLOption(ttl time.Duration) []lock.Option {
	if ttl <= 0 {
		return nil
	}
	return []lock.Option{lock.WithTTL(ttl)}
}
