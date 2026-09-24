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

package lock_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/stdlib/testing/assert"
)

func TestNewElectionValidatesConfig(t *testing.T) {
	_, err := lock.NewElection(lock.ElectionConfig{Key: "k"})
	assert.That(t, err != nil && strings.Contains(err.Error(), "requires a Locker")).True()
	_, err = lock.NewElection(lock.ElectionConfig{Locker: lock.NewMemoryLocker()})
	assert.That(t, err != nil && strings.Contains(err.Error(), "requires a Key")).True()
}

func TestElectionSingleLeader(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 5
	var live atomic.Int32    // leaders active right now
	var maxLive atomic.Int32 // high-water mark of concurrent leaders

	var wg sync.WaitGroup
	elections := make([]*lock.Election, n)
	for i := range n {
		e, erre := lock.NewElection(lock.ElectionConfig{
			Locker:        m,
			Key:           "leader",
			TTL:           40 * time.Millisecond,
			RenewInterval: 10 * time.Millisecond,
			RetryInterval: 5 * time.Millisecond,
			OnStartedLeading: func(ctx context.Context) {
				cur := live.Add(1)
				for {
					old := maxLive.Load()
					if cur <= old || maxLive.CompareAndSwap(old, cur) {
						break
					}
				}
				<-ctx.Done()
				live.Add(-1)
			},
		})
		assert.That(t, erre).Nil()
		elections[i] = e
		wg.Go(func() {
			_ = e.Run(ctx)
		})
	}

	time.Sleep(200 * time.Millisecond)

	// Exactly one instance should report leadership at any moment.
	leaders := 0
	for _, e := range elections {
		if e.IsLeader() {
			leaders++
		}
	}
	assert.That(t, leaders).Equal(1)
	assert.That(t, maxLive.Load()).Equal(int32(1))

	cancel()
	wg.Wait()
}

func TestElectionFailover(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()

	ctx := t.Context()
	var resigned atomic.Int32

	// Leader A: short-lived context so it resigns and releases quickly.
	ctxA, cancelA := context.WithCancel(ctx)
	elected := make(chan struct{}, 1)
	a, erra := lock.NewElection(lock.ElectionConfig{
		Locker:           m,
		Key:              "leader",
		RetryInterval:    5 * time.Millisecond,
		OnStartedLeading: func(context.Context) { elected <- struct{}{} },
		OnStoppedLeading: func() { resigned.Add(1) },
	})
	assert.That(t, erra).Nil()
	go func() { _ = a.Run(ctxA) }()

	select {
	case <-elected:
	case <-time.After(time.Second):
		t.Fatal("A never became leader")
	}
	assert.That(t, a.IsLeader()).True()

	// B campaigns; it must not win while A holds.
	tookOver := make(chan struct{}, 1)
	b, errb := lock.NewElection(lock.ElectionConfig{
		Locker:           m,
		Key:              "leader",
		RetryInterval:    5 * time.Millisecond,
		OnStartedLeading: func(context.Context) { tookOver <- struct{}{} },
	})
	assert.That(t, errb).Nil()
	go func() { _ = b.Run(ctx) }()

	select {
	case <-tookOver:
		t.Fatal("B took leadership while A was still leader")
	case <-time.After(30 * time.Millisecond):
	}

	// A steps down; B must take over.
	cancelA()
	select {
	case <-tookOver:
	case <-time.After(time.Second):
		t.Fatal("B did not take over after A resigned")
	}
	assert.That(t, b.IsLeader()).True()
	assert.That(t, resigned.Load()).Equal(int32(1))
}

// TestElectionFailoverOnLeaseExpiry proves leadership transfers when the leader
// effectively dies without unlocking: its lease expires, a follower reclaims the
// key, and the dead leader's OnStartedLeading ctx is cancelled via Lost().
func TestElectionFailoverOnLeaseExpiry(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()

	// The "dead" leader: renew disabled, short TTL — it never refreshes.
	deadCtx, cancelDead := context.WithCancel(context.Background())
	defer cancelDead()
	lost := make(chan struct{}, 1)
	resigned := make(chan struct{}, 1)
	dead, errdead := lock.NewElection(lock.ElectionConfig{
		Locker:           m,
		Key:              "leader",
		TTL:              30 * time.Millisecond,
		RenewInterval:    -1, // no renew: lease expires under it
		RetryInterval:    5 * time.Millisecond,
		OnStartedLeading: func(ctx context.Context) { <-ctx.Done(); lost <- struct{}{} },
		OnStoppedLeading: func() { resigned <- struct{}{} },
	})
	assert.That(t, errdead).Nil()
	go func() { _ = dead.Run(deadCtx) }()
	time.Sleep(10 * time.Millisecond) // let it win

	// The follower campaigns and must take over once the lease expires.
	tookOver := make(chan struct{}, 1)
	follower, errfollower := lock.NewElection(lock.ElectionConfig{
		Locker:           m,
		Key:              "leader",
		RetryInterval:    5 * time.Millisecond,
		OnStartedLeading: func(context.Context) { tookOver <- struct{}{} },
	})
	assert.That(t, errfollower).Nil()
	go func() { _ = follower.Run(context.Background()) }()

	select {
	case <-lost:
	case <-time.After(2 * time.Second):
		t.Fatal("dead leader was not notified of loss after lease expiry")
	}
	select {
	case <-resigned:
	case <-time.After(2 * time.Second):
		t.Fatal("dead leader did not resign after lease expiry")
	}
	select {
	case <-tookOver:
	case <-time.After(2 * time.Second):
		t.Fatal("follower did not take over after lease expiry")
	}
	assert.That(t, follower.IsLeader()).True()
	cancelDead()
}
