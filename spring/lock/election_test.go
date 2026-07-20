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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/spring/lock"
	"go-spring.org/stdlib/testing/assert"
)

func TestNewElectionPanics(t *testing.T) {
	assert.Panic(t, func() {
		lock.NewElection(lock.ElectionConfig{Key: "k"})
	}, "requires a Locker")
	assert.Panic(t, func() {
		lock.NewElection(lock.ElectionConfig{Locker: lock.NewMemoryLocker()})
	}, "requires a Key")
}

func TestElectionSingleLeader(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 5
	var live atomic.Int32   // leaders active right now
	var maxLive atomic.Int32 // high-water mark of concurrent leaders

	var wg sync.WaitGroup
	elections := make([]*lock.Election, n)
	for i := range n {
		e := lock.NewElection(lock.ElectionConfig{
			Locker:        m,
			Key:           "leader",
			TTL:           40 * time.Millisecond,
			RenewInterval: 10 * time.Millisecond,
			RetryInterval: 5 * time.Millisecond,
			OnElected: func(ctx context.Context) {
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
	a := lock.NewElection(lock.ElectionConfig{
		Locker:        m,
		Key:           "leader",
		RetryInterval: 5 * time.Millisecond,
		OnElected:     func(context.Context) { elected <- struct{}{} },
		OnResigned:    func() { resigned.Add(1) },
	})
	go func() { _ = a.Run(ctxA) }()

	select {
	case <-elected:
	case <-time.After(time.Second):
		t.Fatal("A never became leader")
	}
	assert.That(t, a.IsLeader()).True()

	// B campaigns; it must not win while A holds.
	tookOver := make(chan struct{}, 1)
	b := lock.NewElection(lock.ElectionConfig{
		Locker:        m,
		Key:           "leader",
		RetryInterval: 5 * time.Millisecond,
		OnElected:     func(context.Context) { tookOver <- struct{}{} },
	})
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
