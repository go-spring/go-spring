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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/cloud/scheduling"
	"go-spring.org/stdlib/testing/assert"
)

func mustJob(t *testing.T, name string, trigger scheduling.Trigger, run func(context.Context) error, opts ...scheduling.Option) *scheduling.Job {
	t.Helper()
	j, err := scheduling.NewJob(name, trigger, run, opts...)
	assert.Error(t, err).Nil()
	return j
}

// A job that cannot fire must not be buildable: NewJob rejects each of these,
// so the mistake surfaces at construction rather than as a task that silently
// never fires.
func TestNewJobRejectsAnUnusableRegistration(t *testing.T) {
	run := func(context.Context) error { return nil }

	_, err := scheduling.NewJob("", scheduling.FixedRate(time.Second), run)
	assert.Error(t, err).NotNil("empty name")
	assert.Error(t, err).Matches("job name must not be empty")

	_, err = scheduling.NewJob("j", nil, run)
	assert.Error(t, err).NotNil("nil trigger")
	assert.Error(t, err).Matches("has no trigger")

	_, err = scheduling.NewJob("j", scheduling.FixedRate(time.Second), nil)
	assert.Error(t, err).NotNil("nil run")
	assert.Error(t, err).Matches("nil run function")
}

func TestScheduleRejectsDuplicates(t *testing.T) {
	s := scheduling.NewScheduler()

	job := func(context.Context) error { return nil }
	j1, err := scheduling.NewJob("dup", scheduling.FixedRate(time.Second), job)
	assert.Error(t, err).Nil()
	_, err = s.Schedule(j1)
	assert.Error(t, err).Nil()
	j2, err := scheduling.NewJob("dup", scheduling.FixedRate(time.Second), job)
	assert.Error(t, err).Nil()
	_, err = s.Schedule(j2)
	assert.Error(t, err).Is(scheduling.ErrDuplicateName)
}

func TestScheduleAfterStopIsRejected(t *testing.T) {
	s := scheduling.NewScheduler()
	assert.Error(t, s.Start(context.Background())).Nil()

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	_, err := s.Schedule(mustJob(t, "late", scheduling.FixedRate(time.Second),
		func(context.Context) error { return nil }))
	assert.Error(t, err).Is(scheduling.ErrStopped)
}

func TestFixedRateFires(t *testing.T) {
	s := scheduling.NewScheduler()
	var count atomic.Int64
	_, err := s.Schedule(mustJob(t, "tick", scheduling.FixedRate(20*time.Millisecond),
		func(context.Context) error { count.Add(1); return nil }))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(110 * time.Millisecond) // ~5 fires
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	got := count.Load()
	assert.That(t, got >= 3).True("expected at least 3 fires")
}

func TestFixedDelayIsSerial(t *testing.T) {
	// A fixed-delay job that sleeps must never overlap: at most one runs at a
	// time, so the observed max concurrency is 1.
	s := scheduling.NewScheduler()
	var inFlight, maxSeen atomic.Int64
	_, err := s.Schedule(mustJob(t, "serial", scheduling.FixedDelay(10*time.Millisecond),
		func(context.Context) error {
			cur := inFlight.Add(1)
			for {
				m := maxSeen.Load()
				if cur <= m || maxSeen.CompareAndSwap(m, cur) {
					break
				}
			}
			time.Sleep(15 * time.Millisecond)
			inFlight.Add(-1)
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(120 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	assert.That(t, maxSeen.Load()).Equal(int64(1))
}

func TestConcurrencyPolicySkip(t *testing.T) {
	// A slow job under the default Skip policy must never overlap even at a fast
	// fire rate: concurrent runs are dropped.
	s := scheduling.NewScheduler()
	var inFlight, maxSeen, runs atomic.Int64
	_, err := s.Schedule(mustJob(t, "skip", scheduling.FixedRate(10*time.Millisecond),
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
		}, scheduling.WithConcurrencyPolicy(scheduling.Skip)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(150 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	assert.That(t, maxSeen.Load()).Equal(int64(1))
	// With Skip, far fewer runs than fires happen (fires every 10ms, run takes
	// 40ms), so runs should be well below the ~15 fire ticks.
	assert.That(t, runs.Load() < 10).True("expected skips to limit runs")
}

func TestConcurrencyPolicyReplace(t *testing.T) {
	// Under Replace a new fire cancels the in-flight run. The cancelled runs
	// observe ctx cancellation.
	s := scheduling.NewScheduler()
	var cancelled atomic.Int64
	_, err := s.Schedule(mustJob(t, "replace", scheduling.FixedRate(15*time.Millisecond),
		func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				cancelled.Add(1)
				return ctx.Err()
			case <-time.After(60 * time.Millisecond):
				return nil
			}
		}, scheduling.WithConcurrencyPolicy(scheduling.Replace)))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(120 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	assert.That(t, cancelled.Load() >= 1).True("expected at least one run to be replaced/cancelled")
}

func TestStopWaitsForInFlight(t *testing.T) {
	// Stop must block until an in-flight run finishes (within its deadline).
	s := scheduling.NewScheduler()
	var finished atomic.Bool
	_, err := s.Schedule(mustJob(t, "drain", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error {
			time.Sleep(60 * time.Millisecond)
			finished.Store(true)
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(15 * time.Millisecond) // ensure one run is in flight
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()
	assert.That(t, finished.Load()).True("Stop should have waited for the in-flight run")
}

func TestStopDeadlineExceeded(t *testing.T) {
	// If a run ignores cancellation and outlives the Stop deadline, Stop reports
	// the deadline error rather than blocking forever.
	s := scheduling.NewScheduler()
	_, err := s.Schedule(mustJob(t, "stubborn", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error {
			time.Sleep(500 * time.Millisecond) // ignores ctx
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(15 * time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).NotNil()
}

func TestCancelRemovesTask(t *testing.T) {
	s := scheduling.NewScheduler()
	var count atomic.Int64
	cancel, err := s.Schedule(mustJob(t, "temp", scheduling.FixedRate(10*time.Millisecond),
		func(context.Context) error { count.Add(1); return nil }))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(35 * time.Millisecond)
	cancel()
	stable := count.Load()
	time.Sleep(50 * time.Millisecond)
	assert.That(t, count.Load()).Equal(stable) // no more fires after cancel

	stopCtx, c := context.WithTimeout(context.Background(), time.Second)
	defer c()
	assert.Error(t, s.Stop(stopCtx)).Nil()
}

// stubLocker grants the lock to only the first caller, so multiple "replicas"
// sharing it de-duplicate to one runner.
type stubLocker struct {
	mu   sync.Mutex
	held map[string]bool
}

func newStubLocker() *stubLocker { return &stubLocker{held: map[string]bool{}} }

func (s *stubLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
	l, ok, err := s.TryAcquire(ctx, key, opts...)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, lock.ErrLockHeld
	}
	return l, nil
}

func (s *stubLocker) Close() error { return nil }

func (s *stubLocker) TryAcquire(_ context.Context, key string, _ ...lock.Option) (lock.Lock, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[key] {
		return nil, false, nil
	}
	s.held[key] = true
	return &stubLock{locker: s, key: key}, true, nil
}

type stubLock struct {
	locker *stubLocker
	key    string
}

func (s *stubLock) Key() string           { return s.key }
func (s *stubLock) Token() string         { return s.key }
func (s *stubLock) Lost() <-chan struct{} { return nil } // never closes: never lost

func (l *stubLock) Unlock(context.Context) error {
	l.locker.mu.Lock()
	defer l.locker.mu.Unlock()
	l.locker.held[l.key] = false
	return nil
}

func TestWithLockDeduplicates(t *testing.T) {
	// Two schedulers ("replicas") share one locker and one key. The lock grants
	// exclusive access, so the two replicas' runs never overlap: observed max
	// concurrency across both is 1, even though each fires independently.
	locker := newStubLocker()
	var inFlight, maxSeen, total atomic.Int64

	mk := func() *scheduling.Scheduler {
		s := scheduling.NewScheduler()
		_, err := s.Schedule(mustJob(t, "job", scheduling.FixedRate(15*time.Millisecond),
			func(context.Context) error {
				total.Add(1)
				cur := inFlight.Add(1)
				for {
					m := maxSeen.Load()
					if cur <= m || maxSeen.CompareAndSwap(m, cur) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				inFlight.Add(-1)
				return nil
			}, scheduling.WithLock(locker, "job", 0)))
		assert.Error(t, err).Nil()
		return s
	}

	sa, sb := mk(), mk()
	assert.Error(t, sa.Start(context.Background())).Nil()
	assert.Error(t, sb.Start(context.Background())).Nil()
	time.Sleep(150 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, sa.Stop(stopCtx)).Nil()
	assert.Error(t, sb.Stop(stopCtx)).Nil()

	assert.That(t, total.Load() >= 1).True("some runs should have happened")
	assert.That(t, maxSeen.Load()).Equal(int64(1))
}

func TestCancelIsIdempotent(t *testing.T) {
	s := scheduling.NewScheduler()
	cancel, err := s.Schedule(mustJob(t, "a", scheduling.FixedRate(time.Hour), func(context.Context) error { return nil }))
	assert.Error(t, err).Nil()
	cancel()
	cancel() // second call finds no task and is a no-op
}

func TestStopBeforeStart(t *testing.T) {
	s := scheduling.NewScheduler()
	_, err := s.Schedule(mustJob(t, "a", scheduling.FixedRate(time.Hour), func(context.Context) error { return nil }))
	assert.Error(t, err).Nil()
	assert.Error(t, s.Stop(context.Background())).Nil()
}

func TestStartIsIdempotent(t *testing.T) {
	s := scheduling.NewScheduler()
	assert.Error(t, s.Start(context.Background())).Nil()
	assert.Error(t, s.Start(context.Background())).Nil() // no double launch
	assert.Error(t, s.Stop(context.Background())).Nil()
}
