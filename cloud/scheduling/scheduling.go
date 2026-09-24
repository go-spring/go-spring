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

// Package scheduling defines a framework-agnostic abstraction for running
// periodic and scheduled background jobs.
//
// It answers one question for a long-running service: "run this piece of work
// on a schedule — every N seconds, N seconds after the last run finished, or on
// a cron expression — for as long as the process lives, and stop it cleanly when
// the process is shutting down." A job is a plain function bound to a [Trigger]
// and driven by a [Scheduler] that participates in the application lifecycle.
//
// The three built-in triggers are:
//
//   - [FixedRate]: fire on a fixed period measured from each scheduled fire time,
//     independent of how long a run takes. Overlap is possible and is governed by
//     the task's [ConcurrencyPolicy].
//   - [FixedDelay]: fire a fixed interval after the previous run *finishes*. Runs
//     never overlap.
//   - [ParseCron]: fire on a standard 5-field cron expression.
//
// The abstraction is deliberately split from any backend. A [Scheduler] runs
// entirely in-process; when the same job must run on only one replica of a
// multi-replica deployment, attach a [lock.Locker] to the [Job] (see
// [WithLock]) so only the lock holder executes each fire.
package scheduling

import (
	"context"
	"errors"
	"sync"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/goutil"
	"go-spring.org/stdlib/timeutil"
)

var (
	// ErrDuplicateName is returned when Schedule is called with a name already in
	// use on the same scheduler.
	ErrDuplicateName = errors.New("scheduling: duplicate task name")

	// ErrStopped is returned when Schedule is called on a scheduler that has
	// already been stopped. A stopped scheduler cannot be restarted, so it accepts
	// no new tasks.
	ErrStopped = errors.New("scheduling: scheduler already stopped")

	// ErrJobPanicked is wrapped around the panic value when a job panics. A panic
	// is reported like any other failed run, so this is what tells the two apart:
	// errors.Is(err, ErrJobPanicked) is true only when the run panicked, letting
	// metrics and logs count panics separately from returned errors.
	ErrJobPanicked = errors.New("scheduling: job panicked")
)

// Job is one scheduled unit of work: the registration bundle of everything
// [Scheduler.Schedule] needs — the name, the run function, the trigger, the
// execution options, and optionally the distributed lock. Build it with
// [NewJob]; the fields are unexported, so a Job cannot be assembled wrongly.
//
// The run function should honour ctx: when ctx is cancelled (application
// shutdown, per-run timeout, or a Replace concurrency policy pre-empting it)
// it should stop promptly and return. A returned error is reported in the
// fire's metrics and log line; it does not stop the schedule.
type Job struct {
	name    string
	run     func(ctx context.Context) error
	trigger Trigger
	opts    []Option
}

// NewJob builds a [Job]. It returns an error on an empty name, a nil run
// function or a nil trigger: each of those would otherwise become a job that
// silently never fires. The options cover execution — [WithTimeout],
// [WithConcurrencyPolicy] — and the distributed lock ([WithLock]).
func NewJob(name string, trigger Trigger, run func(ctx context.Context) error, opts ...Option) (*Job, error) {
	if name == "" {
		return nil, errutil.Explain(nil, "scheduling: job name must not be empty")
	}
	if run == nil {
		return nil, errutil.Explain(nil, "scheduling: job %q has a nil run function", name)
	}
	if trigger == nil {
		return nil, errutil.Explain(nil, "scheduling: job %q has no trigger; pass FixedRate, FixedDelay, After or ParseCron", name)
	}
	return &Job{name: name, trigger: trigger, run: run, opts: opts}, nil
}

// Name returns the job's name: the scheduler's task name, and the default
// lock key.
func (j *Job) Name() string { return j.name }

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

// Options controls a single scheduled task's execution. A zero Options is
// valid; see each field for its default.
type Options struct {
	// Policy governs overlapping runs of a fixed-rate/cron job. Defaults to Skip.
	Policy ConcurrencyPolicy

	// Timeout, when positive, bounds a single run: the job's context is cancelled
	// after it elapses. Zero means no per-run timeout.
	Timeout time.Duration

	// Locker, when set, de-duplicates fires across replicas: each fire acquires
	// the lock and only the holder runs. It is the cloud/lock type directly —
	// the type the lock backends and starter-lock-* beans already are.
	Locker lock.Locker

	// LockKey is the key acquired on the locker; empty means the job name.
	LockKey string

	// LockTTL is the lease duration, auto-renewed while the job holds it; zero
	// keeps the locker's own default.
	LockTTL time.Duration
}

// Option mutates [Options].
type Option func(*Options)

// WithConcurrencyPolicy sets how overlapping runs are handled.
func WithConcurrencyPolicy(p ConcurrencyPolicy) Option {
	return func(o *Options) { o.Policy = p }
}

// WithTimeout bounds a single run; the job's context is cancelled after d.
//
// The timeout is the run's value deadline — "how late a result is no longer
// worth waiting for" — not pacing (that is the trigger's job). Pick it from
// data: a few times the run's P99 from scheduling.run.duration, loose rather
// than tight, since it is the last fence for a stuck run, not a throttle for a
// slow one. Under Skip/Queue it should stay below the fire interval, or every
// fire while a slow run holds the slot is skipped; under fixed-delay the next
// fire waits for completion anyway, so the bound is purely about cutting
// losses. A timeout only helps a job that honours its ctx: cancellation is a
// signal, nothing force-stops a goroutine. A job whose every internal call
// already carries its own deadline does not need this.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

// WithLock de-duplicates the job across replicas with a distributed lock: each
// fire acquires the lock and only the holder runs. It takes the cloud/lock
// [lock.Locker] directly — the type the lock backends and
// starter-lock-{redis,etcd,consul} beans already are — so no adapter stands
// between the caller and the lock. The key defaults to the job name when
// empty; ttl is the lease duration (zero keeps the locker's own default),
// auto-renewed while the job holds it.
func WithLock(l lock.Locker, key string, ttl time.Duration) Option {
	return func(o *Options) {
		o.Locker, o.LockKey, o.LockTTL = l, key, ttl
	}
}

// event describes one scheduled fire, recorded by the package's built-in
// instrumentation. A fire either runs or is skipped. A skipped event has Skipped
// set and Reason naming why, with Start and Duration left zero.
type event struct {
	// Name is the task name.
	Name string

	// Scheduled is the fire time this event corresponds to. A fire that waited
	// under [Queue] still reports its own time, not the one it waited behind.
	Scheduled time.Time

	// Start is when the run started; zero for a skipped fire.
	Start time.Time

	// Duration is how long the run took; zero for a skipped fire.
	Duration time.Duration

	// Err is the job's error on a run — one that panicked reports an error
	// wrapping [ErrJobPanicked] — or the locker's error on a fire skipped because
	// acquiring the lock failed; nil otherwise.
	Err error

	// Skipped is true when the fire did not run.
	Skipped bool

	// Reason names why a skipped fire did not run: "policy" (a concurrency policy
	// dropped it) or "lock" (another replica held the lock, or acquiring it
	// failed).
	Reason string
}

// NewScheduler returns a [Scheduler]. Nothing runs until Start.
func NewScheduler() *Scheduler {
	return &Scheduler{tasks: make(map[string]*task)}
}

// Scheduler runs jobs against their triggers; it is safe for concurrent use.
// mu guards every field below it, including the task registry and the lifecycle
// flags; a task's own runtime state is guarded separately by [task].mu, so
// managing tasks never contends with running them.
type Scheduler struct {
	mu    sync.Mutex
	tasks map[string]*task // by task name

	started bool // Start has run; task loops are live
	stopped bool // Stop has run; Schedule is refused and no restart is possible
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup // task loops and in-flight runs, awaited by Stop
}

// Schedule registers a [Job]. A Job is built by [NewJob], which has already
// rejected an empty name, a nil run function and a nil trigger. Schedule
// itself fails only on a duplicate name and once the scheduler has been
// stopped ([ErrStopped]). The returned cancel function removes the task and,
// if the scheduler is running, stops its loop; it is idempotent.
//
// Scheduling before Start records the task; scheduling after Start also
// launches its loop immediately.
func (s *Scheduler) Schedule(j *Job) (func(), error) {
	var o Options
	for _, fn := range j.opts {
		fn(&o)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil, errutil.Explain(ErrStopped, "task %q", j.name)
	}
	if _, ok := s.tasks[j.name]; ok {
		return nil, errutil.Explain(ErrDuplicateName, "task %q", j.name)
	}

	t := &task{
		job:  j,
		opts: o,
		wg:   &s.wg,
	}
	// A fixedDelay trigger runs serially: the loop runs it inline so the next
	// fire is measured from its completion, making the ConcurrencyPolicy moot.
	t.isSerial = isFixedDelay(j.trigger)
	s.tasks[j.name] = t

	if s.started {
		s.launch(t)
	}

	// remove is itself idempotent (a missing name is a no-op), so the returned
	// cancel needs no extra guard.
	return func() { s.remove(j.name) }, nil
}

// launch starts a task's loop goroutine. Caller holds s.mu.
func (s *Scheduler) launch(t *task) {
	loopCtx, cancel := context.WithCancel(s.ctx)
	t.loopCancel = cancel
	s.wg.Go(func() {
		t.loop(loopCtx)
	})
}

// remove cancels a task's loop and drops it from the registry. In-flight runs
// finish on their own (their context is cancelled with the loop).
func (s *Scheduler) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[name]
	if !ok {
		return
	}
	delete(s.tasks, name)
	if t.loopCancel != nil {
		t.loopCancel()
	}
}

// Start begins running all registered tasks. It returns once the loops are
// launched; it does not block. Calling Start twice is a no-op after the
// first. ctx bounds the lifetime of every task loop.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return nil
	}
	s.started = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	for _, t := range s.tasks {
		s.launch(t)
	}
	return nil
}

// Stop halts scheduling of new fires and waits for in-flight runs to finish,
// bounded by ctx. It reports a DeadlineExceeded-wrapping error if the deadline
// elapses first. After Stop the scheduler cannot be restarted.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	stopped, started, cancel := s.stopped, s.started, s.cancel
	s.stopped = true
	s.mu.Unlock()

	if cancel != nil {
		cancel() // stop every loop; in-flight runs finish on their own
	}
	if stopped || !started {
		return nil
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait() // every loop and every in-flight run has finished
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return errutil.Explain(ctx.Err(), "scheduling: stop timed out waiting for tasks to drain")
	}
}

// task is one scheduled job and its runtime state. job, opts and isSerial are
// set once at creation and read without locking; everything below mu is shared
// between the task's loop goroutine and its run goroutines, so reads and
// writes there must hold mu.
type task struct {
	job      *Job
	opts     Options
	isSerial bool // a fixedDelay trigger: runs inline, never overlaps

	loopCancel context.CancelFunc // set by launch, cancelled by remove; guarded by Scheduler.mu
	wg         *sync.WaitGroup    // the scheduler's; run goroutines enter it, awaited by Stop

	mu             sync.Mutex
	lastScheduled  time.Time          // start of the last fire, fed back to the trigger
	lastCompletion time.Time          // end of the last run; fixed-delay anchors on it
	running        bool               // a run is in flight (Skip/Queue bookkeeping)
	queued         bool               // one fire is parked behind the in-flight run (Queue)
	queuedAt       time.Time          // the parked fire's own time, so it reports it (Queue)
	replaceCancel  context.CancelFunc // cancels the in-flight run under Replace
}

// loop computes each fire time from the trigger, waits for it, then dispatches
// the run. For a serial (fixed-delay) trigger it runs synchronously so the next
// fire is measured from completion; otherwise it dispatches per concurrency
// policy without blocking the loop.
func (t *task) loop(ctx context.Context) {
	for {
		t.mu.Lock()
		tc := TriggerContext{
			Now:            time.Now(),
			LastScheduled:  t.lastScheduled,
			LastCompletion: t.lastCompletion,
		}
		t.mu.Unlock()

		next := t.job.trigger.Next(tc)
		if next.IsZero() {
			// A zero next means the trigger can never fire again. For a one-shot
			// [After] trigger that is the normal end of the job; otherwise it is
			// in practice an impossible cron date (e.g. Feb 30), i.e. a
			// misconfiguration that registration could not catch. The task would
			// otherwise go silently dead, so say so once, on the default tag like
			// the other lifecycle lines.
			_, oneShot := t.job.trigger.(after)
			if !oneShot && ctx.Err() == nil {
				log.Warn(context.Background(), log.TagAppDef,
					log.String("task", t.job.name),
					log.Msg("scheduler: task's trigger reports no further fire time"))
			}
			return
		}

		d := max(time.Until(next), 0)
		if !timeutil.Sleep(ctx, d) {
			return
		}

		t.mu.Lock()
		t.lastScheduled = next
		t.mu.Unlock()

		if t.isSerial {
			// Serial (fixed-delay): run inline in the loop, so the loop does not
			// even ask for the next fire until this run finishes — the gap is
			// measured from completion and two runs can never overlap.
			t.runOnce(ctx, next)
		} else {
			// Concurrent (fixed-rate / cron): hand the fire to dispatch and move
			// straight on — the loop must stay free to keep computing fire times,
			// and overlaps are settled there by the ConcurrencyPolicy.
			t.dispatch(ctx, next)
		}
	}
}

// dispatch runs a non-serial fire according to the concurrency policy.
func (t *task) dispatch(ctx context.Context, scheduled time.Time) {
	switch t.opts.Policy {
	case Queue:
		t.mu.Lock()
		if t.running {
			if t.queued {
				t.mu.Unlock()
				recordSkip(scheduled, t.job.name, "policy")
				return
			}
			t.queued, t.queuedAt = true, scheduled
			t.mu.Unlock()
			return // the active worker will pick this up when it finishes
		}
		t.running = true
		t.mu.Unlock()
		t.wg.Go(func() { t.queueWorker(ctx, scheduled) })
	case Replace:
		t.mu.Lock()
		if t.replaceCancel != nil {
			t.replaceCancel() // pre-empt the in-flight run
		}
		runCtx, cancel := context.WithCancel(ctx)
		t.replaceCancel = cancel
		t.mu.Unlock()
		t.wg.Go(func() {
			defer cancel()
			t.runOnce(runCtx, scheduled)
		})

	default: // Skip
		t.mu.Lock()
		if t.running {
			t.mu.Unlock()
			recordSkip(scheduled, t.job.name, "policy")
			return
		}
		t.running = true
		t.mu.Unlock()
		t.wg.Go(func() {
			t.runOnce(ctx, scheduled)
			t.mu.Lock()
			t.running = false
			t.mu.Unlock()
		})
	}
}

// queueWorker drains the current run and at most one queued fire, sequentially.
// Each run is attributed to the fire that scheduled it, so a queued fire reports
// its own time rather than the one it waited behind.
func (t *task) queueWorker(ctx context.Context, scheduled time.Time) {
	for {
		t.runOnce(ctx, scheduled)
		t.mu.Lock()
		if t.queued {
			scheduled, t.queued = t.queuedAt, false
			// A fire parked while the scheduler is stopping stays parked: running
			// it would feed an already-cancelled context into TryAcquire and log
			// a bogus lock failure on every shutdown. It is dropped — not run,
			// not counted as a skip — and said so at Debug, the same level as a
			// routine swallow.
			if ctx.Err() != nil {
				t.running = false
				t.mu.Unlock()
				log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
					return []log.Field{
						log.String("task", t.job.name),
						log.Msg("scheduler: queued fire dropped on stop"),
					}
				})
				return
			}
			t.mu.Unlock()
			continue
		}
		t.running = false
		t.mu.Unlock()
		return
	}
}

// runOnce performs a single execution: optional distributed lock, optional
// per-run timeout, the job itself (panic-guarded), and completion bookkeeping.
func (t *task) runOnce(parent context.Context, scheduled time.Time) {
	ctx := parent
	if t.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, t.opts.Timeout)
		defer cancel()
	}

	if t.opts.Locker != nil {
		key := t.opts.LockKey
		if key == "" {
			key = t.job.name
		}
		var opts []lock.Option
		if t.opts.LockTTL > 0 {
			opts = append(opts, lock.WithTTL(t.opts.LockTTL))
		}
		l, ok, err := t.opts.Locker.TryAcquire(ctx, key, opts...)
		if err != nil {
			record(event{Name: t.job.name, Scheduled: scheduled, Err: err, Skipped: true, Reason: "lock"})
			return
		}
		if !ok {
			// Another replica holds the lock; this replica skips this fire.
			recordSkip(scheduled, t.job.name, "lock")
			return
		}
		// An unlock failure is not the run's outcome (the run already finished
		// and reported), so it does not touch the event; but it means the lock
		// now relies on its TTL expiring, which the holder should know about.
		defer func() {
			if err := l.Unlock(context.WithoutCancel(ctx)); err != nil {
				log.Warn(context.Background(), log.TagAppDef,
					log.String("task", t.job.name),
					log.String("key", key),
					log.Err(err),
					log.Msg("scheduler: lock release failed; the lock now relies on its TTL"))
			}
		}()
	}

	ctx, span := traceRun(ctx, t.job.name)
	start := time.Now()
	err := safeRun(ctx, t.job.run)
	end := time.Now()
	endRun(span, err)

	t.mu.Lock()
	t.lastCompletion = end
	t.mu.Unlock()

	record(event{
		Name:      t.job.name,
		Scheduled: scheduled,
		Start:     start,
		Duration:  end.Sub(start),
		Err:       err,
	})
}

// safeRun invokes job, converting a panic into an error so one bad run cannot
// kill the scheduler goroutine. The error wraps [ErrJobPanicked], so a caller can
// tell a panicking run from one that merely returned an error. The panic is also
// reported through the shared panic chain (goutil), so the framework's
// log/metrics bridge sees it.
func safeRun(ctx context.Context, run func(ctx context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			goutil.ReportPanic(ctx, r)
			err = errutil.Explain(ErrJobPanicked, "job run: %v", r)
		}
	}()
	return run(ctx)
}
