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

package scheduling_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/experimental/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

// TestConcurrencyPolicyQueue verifies the Queue semantics end to end: a queued
// fire runs right after the in-flight one finishes (still sequentially), and a
// fire arriving while one is already queued is skipped.
func TestConcurrencyPolicyQueue(t *testing.T) {
	var mu sync.Mutex
	var events []scheduling.Event
	s := scheduling.NewScheduler(scheduling.WithObserver(func(ev scheduling.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}))

	var inFlight, maxSeen, runs, skips atomic.Int64
	_, err := s.Schedule("q", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error {
			runs.Add(1)
			cur := inFlight.Add(1)
			for {
				m := maxSeen.Load()
				if cur <= m || maxSeen.CompareAndSwap(m, cur) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			inFlight.Add(-1)
			return nil
		}, scheduling.WithConcurrencyPolicy(scheduling.Queue))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(150 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	// Queued runs still execute strictly one at a time.
	assert.That(t, maxSeen.Load()).Equal(int64(1))

	mu.Lock()
	defer mu.Unlock()
	for _, ev := range events {
		if ev.Skipped {
			skips.Add(1)
			assert.String(t, ev.Reason).Equal("policy")
		}
	}
	// With 40ms runs on a 10ms rate, most fires find a run in flight and one
	// already queued, so skips must happen; but some fires did queue and run
	// sequentially (more runs than the skip-only pattern would allow).
	assert.That(t, skips.Load() > 0).True("expected extra fires beyond one queued to be skipped")
	assert.That(t, runs.Load() > 1).True("expected queued fires to run after the in-flight one")
}

// TestWithTimeoutCancelsRun verifies a per-run timeout cancels the job's context
// and the failure surfaces on the observer event.
func TestWithTimeoutCancelsRun(t *testing.T) {
	var mu sync.Mutex
	var events []scheduling.Event
	s := scheduling.NewScheduler(scheduling.WithObserver(func(ev scheduling.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}))

	var gotErr atomic.Value
	_, err := s.Schedule("slow", scheduling.FixedRate(20*time.Millisecond),
		func(ctx context.Context) error {
			<-ctx.Done() // honour cancellation
			gotErr.Store(ctx.Err())
			return ctx.Err()
		}, scheduling.WithTimeout(15*time.Millisecond))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(60 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	assert.That(t, gotErr.Load()).Equal(context.DeadlineExceeded)

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(events) >= 1).True()
	assert.That(t, events[0].Skipped).False()
	assert.Error(t, events[0].Err).Is(context.DeadlineExceeded)
}

// TestWithLockSkipEmitsEvent verifies a fire skipped because another replica
// holds the lock emits a Skipped event with reason "lock".
func TestWithLockSkipEmitsEvent(t *testing.T) {
	locker := newStubLocker()
	// Pre-hold the key so the scheduler can never acquire it.
	l, ok, err := locker.TryAcquire(context.Background(), "held")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	defer func() { _ = l.Unlock(context.Background()) }()

	var mu sync.Mutex
	var events []scheduling.Event
	s := scheduling.NewScheduler(scheduling.WithObserver(func(ev scheduling.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}))
	_, err = s.Schedule("locked", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { return nil },
		scheduling.WithLock(locker, "held"))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(45 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(events) >= 1).True()
	for _, ev := range events {
		assert.That(t, ev.Skipped).True()
		assert.String(t, ev.Reason).Equal("lock")
		assert.That(t, ev.Start.IsZero()).True("skipped run must not record a start")
	}
}

// errLocker always fails, simulating a lock backend outage.
type errLocker struct{}

func (errLocker) TryAcquire(context.Context, string) (scheduling.Lock, bool, error) {
	return nil, false, context.DeadlineExceeded
}

// TestWithLockAcquireErrorSkips verifies a locker error is surfaced on the event
// (distinct from ordinary contention) and the run is skipped.
func TestWithLockAcquireErrorSkips(t *testing.T) {
	var mu sync.Mutex
	var events []scheduling.Event
	s := scheduling.NewScheduler(scheduling.WithObserver(func(ev scheduling.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}))
	_, err := s.Schedule("err", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { return nil },
		scheduling.WithLock(errLocker{}, "k"))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(35 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(events) >= 1).True()
	for _, ev := range events {
		assert.That(t, ev.Skipped).True()
		assert.String(t, ev.Reason).Equal("lock")
		assert.Error(t, ev.Err).Is(context.DeadlineExceeded)
	}
}

// TestMultipleJobsRunConcurrently registers several fast jobs on one scheduler
// and asserts they all progress together; run under -race this also exercises
// the shared scheduler state across task loops.
func TestMultipleJobsRunConcurrently(t *testing.T) {
	s := scheduling.NewScheduler()
	const jobs = 5
	counts := make([]atomic.Int64, jobs)
	for i := range jobs {
		i := i
		_, err := s.Schedule(fmt.Sprintf("job-%d", i), scheduling.FixedRate(10*time.Millisecond),
			func(context.Context) error { counts[i].Add(1); return nil })
		assert.Error(t, err).Nil()
	}

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(80 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	for i := range jobs {
		assert.That(t, counts[i].Load() >= 3).True(fmt.Sprintf("job %d should have fired", i))
	}
}

// TestScheduleAfterStartLaunchesLoop verifies a task registered after Start also
// runs (dynamic registration).
func TestScheduleAfterStartLaunchesLoop(t *testing.T) {
	s := scheduling.NewScheduler()
	assert.Error(t, s.Start(context.Background())).Nil()

	var count atomic.Int64
	_, err := s.Schedule("late", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { count.Add(1); return nil })
	assert.Error(t, err).Nil()

	time.Sleep(45 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()
	assert.That(t, count.Load() >= 2).True("late-registered task should fire")
}

// TestJobPanicDoesNotKillLoop verifies a panicking job is converted into an
// event error and the schedule keeps firing afterwards.
func TestJobPanicDoesNotKillLoop(t *testing.T) {
	var mu sync.Mutex
	var events []scheduling.Event
	s := scheduling.NewScheduler(scheduling.WithObserver(func(ev scheduling.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}))

	var panics atomic.Int64
	_, err := s.Schedule("bad", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error {
			if panics.Add(1) <= 2 {
				panic("boom")
			}
			return nil
		})
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(60 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(events) >= 3).True("loop must survive panics and keep firing")
	sawPanicErr := false
	for _, ev := range events {
		if ev.Err != nil && !ev.Skipped {
			sawPanicErr = true
			assert.Error(t, ev.Err).Matches("job panicked")
		}
	}
	assert.That(t, sawPanicErr).True("panic should surface as an event error")
}
