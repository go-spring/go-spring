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

package scheduling

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event describes one scheduled fire, delivered to an [Observer] for metrics or
// logging. A run that was skipped (concurrency policy or a lock held by another
// replica) has Skipped set and Reason populated; Start/Duration/Err are zero.
type Event struct {
	Name      string        // task name
	Scheduled time.Time     // the fire time this event corresponds to
	Start     time.Time     // when the run started (zero if skipped)
	Duration  time.Duration // how long the run took (zero if skipped)
	Err       error         // the job's error, if any
	Skipped   bool          // true if the fire did not run
	Reason    string        // "policy" or "lock" when Skipped
}

// Observer receives an [Event] after each fire. It must not block.
type Observer func(Event)

// SchedulerOption configures a [Scheduler] built by [NewScheduler].
type SchedulerOption func(*scheduler)

// WithObserver installs a hook called after every fire (run or skip). Use it to
// emit metrics or logs. Only one observer is kept; the last option wins.
func WithObserver(o Observer) SchedulerOption {
	return func(s *scheduler) { s.observer = o }
}

// serialTrigger marks a trigger whose next fire depends on the previous run
// having finished (fixed-delay). The scheduler runs such tasks strictly one at a
// time, so their [ConcurrencyPolicy] is irrelevant.
type serialTrigger interface{ serial() }

func (fixedDelay) serial() {}

// NewScheduler returns an in-process [Scheduler]. Nothing runs until Start.
func NewScheduler(opts ...SchedulerOption) Scheduler {
	s := &scheduler{tasks: make(map[string]*task)}
	for _, o := range opts {
		o(s)
	}
	return s
}

type scheduler struct {
	mu       sync.Mutex
	tasks    map[string]*task
	observer Observer

	started bool
	stopped bool
	ctx     context.Context
	cancel  context.CancelFunc
	loopWg  sync.WaitGroup // task loop goroutines
}

// Schedule implements [Scheduler].
func (s *scheduler) Schedule(name string, trigger Trigger, job Job, opts ...Option) (func(), error) {
	if trigger == nil {
		return nil, ErrNoTrigger
	}
	if job == nil {
		return nil, ErrNoJob
	}

	var o Options
	for _, fn := range opts {
		fn(&o)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil, fmt.Errorf("scheduling: scheduler already stopped")
	}
	if _, ok := s.tasks[name]; ok {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, name)
	}

	t := &task{
		name:     name,
		trigger:  trigger,
		job:      job,
		opts:     o,
		observer: s.observer,
	}
	_, t.isSerial = trigger.(serialTrigger)
	s.tasks[name] = t

	if s.started {
		s.launch(t)
	}

	var once sync.Once
	return func() {
		once.Do(func() { s.remove(name) })
	}, nil
}

// launch starts a task's loop goroutine. Caller holds s.mu.
func (s *scheduler) launch(t *task) {
	loopCtx, cancel := context.WithCancel(s.ctx)
	t.loopCancel = cancel
	s.loopWg.Go(func() {
		t.loop(loopCtx)
	})
}

// remove cancels a task's loop and drops it from the registry. In-flight runs
// finish on their own (their context is cancelled with the loop).
func (s *scheduler) remove(name string) {
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

// Start implements [Scheduler].
func (s *scheduler) Start(ctx context.Context) error {
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

// Stop implements [Scheduler]. It stops all loops, waits for in-flight runs to
// drain, and reports ctx.Err() if the deadline elapses first.
func (s *scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	if s.cancel != nil {
		s.cancel()
	}
	tasks := make([]*task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	started := s.started
	s.mu.Unlock()

	if !started {
		return nil
	}

	done := make(chan struct{})
	go func() {
		s.loopWg.Wait() // all loops have returned
		for _, t := range tasks {
			t.runWg.Wait() // all in-flight runs have finished
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// task is one scheduled job and its runtime state.
type task struct {
	name     string
	trigger  Trigger
	job      Job
	opts     Options
	observer Observer
	isSerial bool

	loopCancel context.CancelFunc
	runWg      sync.WaitGroup // in-flight runs, awaited on Stop

	mu             sync.Mutex
	lastScheduled  time.Time
	lastCompletion time.Time
	running        bool
	queued         bool
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

		next := t.trigger.Next(tc)
		if next.IsZero() {
			return // never fires again
		}

		d := time.Until(next)
		d = max(d, 0)
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		t.mu.Lock()
		t.lastScheduled = next
		t.mu.Unlock()

		if t.isSerial {
			t.runOnce(ctx, next) // blocks; updates lastCompletion
		} else {
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
				t.emitSkip(scheduled, "policy")
				return
			}
			t.queued = true
			t.mu.Unlock()
			return // the active worker will pick this up when it finishes
		}
		t.running = true
		t.mu.Unlock()
		t.runWg.Add(1)
		go t.queueWorker(ctx, scheduled)
	case Replace:
		t.mu.Lock()
		if t.replaceCancel != nil {
			t.replaceCancel() // pre-empt the in-flight run
		}
		runCtx, cancel := context.WithCancel(ctx)
		t.replaceCancel = cancel
		t.mu.Unlock()
		t.runWg.Go(func() {
			defer cancel()
			t.runOnce(runCtx, scheduled)
		})

	default: // Skip
		t.mu.Lock()
		if t.running {
			t.mu.Unlock()
			t.emitSkip(scheduled, "policy")
			return
		}
		t.running = true
		t.mu.Unlock()
		t.runWg.Go(func() {
			t.runOnce(ctx, scheduled)
			t.mu.Lock()
			t.running = false
			t.mu.Unlock()
		})
	}
}

// queueWorker drains the current run and at most one queued fire, sequentially.
func (t *task) queueWorker(ctx context.Context, scheduled time.Time) {
	defer t.runWg.Done()
	for {
		t.runOnce(ctx, scheduled)
		t.mu.Lock()
		if t.queued {
			t.queued = false
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
		l, ok, err := t.opts.Locker.TryAcquire(ctx, t.opts.LockKey)
		if err != nil {
			t.emit(Event{Name: t.name, Scheduled: scheduled, Err: err, Skipped: true, Reason: "lock"})
			return
		}
		if !ok {
			// Another replica holds the lock; this replica skips this fire.
			t.emitSkip(scheduled, "lock")
			return
		}
		defer func() { _ = l.Unlock(context.WithoutCancel(ctx)) }()
	}

	start := time.Now()
	err := safeRun(ctx, t.job)
	end := time.Now()

	t.mu.Lock()
	t.lastCompletion = end
	t.mu.Unlock()

	t.emit(Event{
		Name:      t.name,
		Scheduled: scheduled,
		Start:     start,
		Duration:  end.Sub(start),
		Err:       err,
	})
}

func (t *task) emitSkip(scheduled time.Time, reason string) {
	t.emit(Event{Name: t.name, Scheduled: scheduled, Skipped: true, Reason: reason})
}

func (t *task) emit(ev Event) {
	if t.observer != nil {
		t.observer(ev)
	}
}

// safeRun invokes job, converting a panic into an error so one bad run cannot
// kill the scheduler goroutine.
func safeRun(ctx context.Context, job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("scheduling: job panicked: %v", r)
		}
	}()
	return job(ctx)
}
