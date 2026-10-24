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

package bulkhead_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/bulkhead"
)

// mustAcquire takes a slot for a caller that is expected to be admitted and
// returns its release function.
func mustAcquire(t *testing.T, b *bulkhead.Bulkhead) func() {
	t.Helper()
	release, ok := b.Acquire(context.Background())
	if !ok {
		t.Fatal("Acquire refused a caller that should have been admitted")
	}
	return release
}

// TestNewPanicsOnNonPositiveMaxConcurrent pins the constructor invariant: no
// isolation is a nil *Bulkhead at the holder, never a disabled instance.
func TestNewPanicsOnNonPositiveMaxConcurrent(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New(0) did not panic")
		}
	}()
	_ = bulkhead.New(0, bulkhead.WithQueueTimeout(time.Second))
}

func TestFullRejectsImmediately(t *testing.T) {
	b := bulkhead.New(1)
	release := mustAcquire(t, b)
	if _, ok := b.Acquire(context.Background()); ok {
		t.Fatal("Acquire admitted a caller while the only slot was held")
	}
	release()
	mustAcquire(t, b)() // the slot is reusable after release
}

// Each successful Acquire owns exactly one slot and its release returns exactly
// one: capacity is conserved across the pair. Releasing twice or not at all is
// outside the contract and would break this conservation silently.
func TestReleaseReturnsExactlyOneSlot(t *testing.T) {
	b := bulkhead.New(2)
	first := mustAcquire(t, b)
	second := mustAcquire(t, b)

	first()
	refill := mustAcquire(t, b) // exactly one slot came back
	if _, ok := b.Acquire(context.Background()); ok {
		t.Fatal("a single release returned more than one slot")
	}

	refill()
	second()
	if s := b.Stats(); s.InUse != 0 {
		t.Fatalf("drained stats = %+v", s)
	}
}

func TestQueueWaitsForRelease(t *testing.T) {
	const queueTimeout = 2 * time.Second
	b := bulkhead.New(1, bulkhead.WithQueueTimeout(queueTimeout))
	release := mustAcquire(t, b)
	go func() {
		time.Sleep(20 * time.Millisecond)
		release()
	}()
	queued := mustAcquire(t, b)
	queued()
	if got := b.QueueTimeout(); got != queueTimeout {
		t.Fatalf("QueueTimeout() = %v, want %v", got, queueTimeout)
	}
}

func TestQueueTimesOut(t *testing.T) {
	const queueTimeout = 20 * time.Millisecond
	b := bulkhead.New(1, bulkhead.WithQueueTimeout(queueTimeout))
	release := mustAcquire(t, b)

	start := time.Now()
	if _, ok := b.Acquire(context.Background()); ok {
		t.Fatal("Acquire admitted a caller after the queue timeout")
	}
	if elapsed := time.Since(start); elapsed < queueTimeout {
		t.Fatalf("Acquire gave up after %v, before the queue timeout of %v", elapsed, queueTimeout)
	}

	release()
	mustAcquire(t, b)() // a timed-out caller must not have consumed the slot
}

func TestQueueHonorsCallerDeadline(t *testing.T) {
	const (
		queueTimeout   = 5 * time.Second
		callerDeadline = 20 * time.Millisecond
	)
	b := bulkhead.New(1, bulkhead.WithQueueTimeout(queueTimeout))
	release := mustAcquire(t, b)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), callerDeadline)
	defer cancel()
	start := time.Now()
	if _, ok := b.Acquire(ctx); ok {
		t.Fatal("Acquire admitted a caller whose own context expired while waiting")
	}
	if elapsed := time.Since(start); elapsed >= queueTimeout {
		t.Fatalf("Acquire waited %v; the caller's deadline did not end the wait", elapsed)
	}
}

func TestQueueHonorsCanceledContext(t *testing.T) {
	b := bulkhead.New(1, bulkhead.WithQueueTimeout(time.Minute))
	release := mustAcquire(t, b)
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := b.Acquire(ctx); ok {
		t.Fatal("Acquire admitted a caller whose context was already canceled")
	}
}

// A caller that can no longer be served must not occupy isolation capacity,
// even in fail-fast mode where there is no queue to time out.
func TestDoneContextIsRefusedWithoutTakingASlot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := bulkhead.New(1)
	if _, ok := b.Acquire(ctx); ok {
		t.Fatal("a done context must not acquire a slot")
	}
	mustAcquire(t, b)() // the refusal must not have consumed the slot
}

// Two callers queued behind one slot are served in turn: the holder's release
// goes to a waiter, and that waiter's release serves the next one.
func TestQueueServesWaitingCallersInTurn(t *testing.T) {
	const queueTimeout = 2 * time.Second
	b := bulkhead.New(1, bulkhead.WithQueueTimeout(queueTimeout))
	release := mustAcquire(t, b)

	var (
		wg       sync.WaitGroup
		admitted = make(chan struct{}, 2)
	)
	for range 2 {
		wg.Go(func() {
			r, ok := b.Acquire(context.Background())
			if !ok {
				return
			}
			admitted <- struct{}{}
			time.Sleep(time.Millisecond) // hold the slot briefly to serialize the waiters
			r()
		})
	}

	release()
	for range 2 {
		select {
		case <-admitted:
		case <-time.After(queueTimeout):
			t.Fatal("a waiting caller was not admitted")
		}
	}
	wg.Wait()
}

// The bound is the whole point of the stage: no matter how many callers
// contend, in-flight work must never exceed maxConcurrent.
func TestInFlightNeverExceedsMaxConcurrent(t *testing.T) {
	const (
		maxConcurrent = 4
		callers       = 64
	)
	b := bulkhead.New(maxConcurrent)

	var (
		wg       sync.WaitGroup
		inFlight atomic.Int32
		peak     atomic.Int32
	)
	for range callers {
		wg.Go(func() {
			release, ok := b.Acquire(context.Background())
			if !ok {
				return // full: fail fast, as configured
			}
			defer release()

			n := inFlight.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			inFlight.Add(-1)
		})
	}
	wg.Wait()

	if got := peak.Load(); got == 0 {
		t.Fatal("no caller was admitted; the test exercised nothing")
	} else if got > maxConcurrent {
		t.Fatalf("peak in-flight = %d, want <= %d", got, maxConcurrent)
	}
}

// TestStatsTracksInUseAndQueued pins the occupancy a state observer reports:
// held slots, queued callers, and total capacity.
func TestStatsTracksInUseAndQueued(t *testing.T) {
	b := bulkhead.New(1, bulkhead.WithQueueTimeout(time.Second))
	if s := b.Stats(); s.InUse != 0 || s.Queued != 0 || s.Capacity != 1 {
		t.Fatalf("fresh stats = %+v", s)
	}

	release := mustAcquire(t, b)
	if s := b.Stats(); s.InUse != 1 || s.Queued != 0 {
		t.Fatalf("held slot stats = %+v", s)
	}

	done := make(chan struct{})
	go func() {
		if r, ok := b.Acquire(context.Background()); ok {
			r()
		}
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		if b.Stats().Queued == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the second caller never showed up as queued")
		}
		time.Sleep(time.Millisecond)
	}

	release()
	<-done
	if s := b.Stats(); s.InUse != 0 || s.Queued != 0 {
		t.Fatalf("drained stats = %+v", s)
	}
}
