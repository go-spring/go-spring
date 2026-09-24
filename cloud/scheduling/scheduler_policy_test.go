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
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/cloud/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

// TestConcurrencyPolicyQueue verifies the Queue semantics end to end: a queued
// fire runs right after the in-flight one finishes (still sequentially), and a
// fire arriving while one is already queued is skipped.
func TestConcurrencyPolicyQueue(t *testing.T) {
	rdr := withMeter(t)
	s := scheduling.NewScheduler()

	var inFlight, maxSeen, runs atomic.Int64
	_, err := s.Schedule(mustJob(t, "q", scheduling.FixedRate(10*time.Millisecond),
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
		}, scheduling.WithConcurrencyPolicy(scheduling.Queue)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(150 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	// Queued runs still execute strictly one at a time.
	assert.That(t, maxSeen.Load()).Equal(int64(1))

	// With 40ms runs on a 10ms rate, most fires find a run in flight and one
	// already queued, so skipped_policy must happen; but some fires did queue
	// and run sequentially (more runs than the skip-only pattern would allow).
	got := fireCounts(t, rdr)
	skips := got["q|skipped_policy"]
	assert.That(t, skips > 0).True("expected extra fires beyond one queued to be skipped")
	assert.That(t, runs.Load() > 1).True("expected queued fires to run after the in-flight one")
}

// TestWithTimeoutCancelsRun verifies a per-run timeout cancels the job's context
// and the failure is reported as an error-status fire.
func TestWithTimeoutCancelsRun(t *testing.T) {
	rdr := withMeter(t)
	s := scheduling.NewScheduler()

	var gotErr atomic.Value
	_, err := s.Schedule(mustJob(t, "slow", scheduling.FixedRate(20*time.Millisecond),
		func(ctx context.Context) error {
			<-ctx.Done() // honour cancellation
			gotErr.Store(ctx.Err())
			return ctx.Err()
		}, scheduling.WithTimeout(15*time.Millisecond)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(60 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	assert.That(t, gotErr.Load()).Equal(context.DeadlineExceeded)
	assert.That(t, fireCounts(t, rdr)["slow|error"] >= 1).True("timed-out runs must report status error")
}

// TestWithLockSkipRecordsStatus verifies a fire skipped because another replica
// holds the lock reports status skipped_lock.
func TestWithLockSkipRecordsStatus(t *testing.T) {
	rdr := withMeter(t)
	locker := newStubLocker()
	// Pre-hold the key so the scheduler can never acquire it.
	l, ok, err := locker.TryAcquire(context.Background(), "held")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	defer func() { _ = l.Unlock(context.Background()) }()

	s := scheduling.NewScheduler()
	_, err = s.Schedule(mustJob(t, "locked", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { return nil }, scheduling.WithLock(locker, "held", 0)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(45 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	got := fireCounts(t, rdr)
	assert.That(t, got["locked|skipped_lock"] >= 1).True("lock contention must report skipped_lock")
	assert.That(t, got["locked|ok"] == 0).True("a pre-held lock must never let the job run")
}

// errLocker always fails, simulating a lock backend outage.
type errLocker struct{}

func (errLocker) TryAcquire(context.Context, string, ...lock.Option) (lock.Lock, bool, error) {
	return nil, false, context.DeadlineExceeded
}

func (errLocker) Acquire(context.Context, string, ...lock.Option) (lock.Lock, error) {
	return nil, context.DeadlineExceeded
}

func (errLocker) Close() error { return nil }

// TestWithLockAcquireErrorSkips verifies a locker error (distinct from ordinary
// contention) also skips the run and reports skipped_lock.
func TestWithLockAcquireErrorSkips(t *testing.T) {
	rdr := withMeter(t)
	s := scheduling.NewScheduler()
	_, err := s.Schedule(mustJob(t, "err", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { return nil }, scheduling.WithLock(errLocker{}, "k", 0)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(35 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	got := fireCounts(t, rdr)
	assert.That(t, got["err|skipped_lock"] >= 1).True("a locker error must skip and report skipped_lock")
	assert.That(t, got["err|ok"] == 0).True("a failing locker must never let the job run")
}

// TestMultipleJobsRunConcurrently registers several fast jobs on one scheduler
// and asserts they all progress together; run under -race this also exercises
// the shared scheduler state across task loops.
func TestMultipleJobsRunConcurrently(t *testing.T) {
	s := scheduling.NewScheduler()
	const jobs = 5
	counts := make([]atomic.Int64, jobs)
	for i := range jobs {
		_, err := s.Schedule(mustJob(t, fmt.Sprintf("job-%d", i), scheduling.FixedRate(10*time.Millisecond),
			func(context.Context) error { counts[i].Add(1); return nil }))
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
	_, err := s.Schedule(mustJob(t, "late", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { count.Add(1); return nil }))
	assert.Error(t, err).Nil()

	time.Sleep(45 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()
	assert.That(t, count.Load() >= 2).True("late-registered task should fire")
}

// TestJobPanicDoesNotKillLoop verifies a panicking job is classified as a
// panic-status fire — the classification rides errors.Is(ErrJobPanicked) — and
// the schedule keeps firing afterwards.
func TestJobPanicDoesNotKillLoop(t *testing.T) {
	rdr := withMeter(t)
	s := scheduling.NewScheduler()

	var panics atomic.Int64
	_, err := s.Schedule(mustJob(t, "bad", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error {
			if panics.Add(1) <= 2 {
				panic("boom")
			}
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(60 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	got := fireCounts(t, rdr)
	assert.That(t, got["bad|panic"]).Equal(int64(2))
	assert.That(t, got["bad|ok"] >= 1).True("loop must survive panics and keep firing")
}

// TestQueueDropsParkedFireOnStop pins the shutdown contract: a fire parked in
// the Queue slot while the scheduler stops is dropped silently — it must not be
// fed an already-cancelled context (which a Locker would report as a bogus
// lock failure polluting the skip metrics).
func TestQueueDropsParkedFireOnStop(t *testing.T) {
	rdr := withMeter(t)
	s := scheduling.NewScheduler()

	firstRun := make(chan struct{})
	_, err := s.Schedule(mustJob(t, "q-stop", scheduling.FixedRate(10*time.Millisecond),
		func(ctx context.Context) error {
			select {
			case <-firstRun:
			default:
				close(firstRun)
			}
			<-ctx.Done() // the first run occupies the worker until shutdown
			return ctx.Err()
		}, scheduling.WithConcurrencyPolicy(scheduling.Queue)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	<-firstRun
	time.Sleep(30 * time.Millisecond) // ≥2 more fires: one parks in the queue slot, the rest skip

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	got := fireCounts(t, rdr)
	// The first run ends with the loop context cancelled, so it reports as
	// error, not ok; either way the parked fire never executed: exactly one
	// non-skipped fire in total.
	ran := got["q-stop|ok"] + got["q-stop|error"] + got["q-stop|panic"]
	assert.That(t, ran == 1).True(fmt.Sprintf("the parked fire must be dropped on stop, counts=%v", got))
}
