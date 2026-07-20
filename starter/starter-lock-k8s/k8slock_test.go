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

package StarterLockK8s

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"

	"go-spring.org/spring/lock"
	"go-spring.org/stdlib/testing/assert"
)

// newTestLocker builds a Locker backed by the client-go fake clientset so the
// Lease acquire/renew/release logic runs without a cluster.
func newTestLocker() *k8sLocker {
	return newK8sLockerWithClient(fake.NewSimpleClientset(), "default", "lock-")
}

// TestTryAcquireContendedThenReleased proves the core mutual-exclusion contract:
// the first TryAcquire wins, a second contends (ok=false, no lease created twice),
// and after Unlock the key is free to take again.
func TestTryAcquireContendedThenReleased(t *testing.T) {
	ctx := context.Background()
	l := newTestLocker()
	defer func() { assert.Error(t, l.Close()).Nil() }()

	held, ok, err := l.TryAcquire(ctx, "job")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.String(t, held.Key()).Equal("job")

	// Contention: a second acquirer with a different identity cannot take it.
	_, ok2, err := l.TryAcquire(ctx, "job", lock.WithToken("other"))
	assert.Error(t, err).Nil()
	assert.That(t, ok2).False()

	// Release, then it becomes available again.
	assert.Error(t, held.Unlock(ctx)).Nil()

	held3, ok3, err := l.TryAcquire(ctx, "job", lock.WithToken("other"))
	assert.Error(t, err).Nil()
	assert.That(t, ok3).True()
	assert.String(t, held3.Token()).Equal("other")
	assert.Error(t, held3.Unlock(ctx)).Nil()
}

// TestReentrantSameToken proves a holder can re-acquire its own lease (renewal
// path) without contending with itself.
func TestReentrantSameToken(t *testing.T) {
	ctx := context.Background()
	l := newTestLocker()
	defer func() { assert.Error(t, l.Close()).Nil() }()

	h1, ok, err := l.TryAcquire(ctx, "job", lock.WithToken("me"))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	h2, ok, err := l.TryAcquire(ctx, "job", lock.WithToken("me"))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	assert.Error(t, h1.Unlock(ctx)).Nil()
	assert.Error(t, h2.Unlock(ctx)).Nil()
}

// TestUnlockIsIdempotent proves a second Unlock is a no-op and Lost() is closed
// after release.
func TestUnlockIsIdempotent(t *testing.T) {
	ctx := context.Background()
	l := newTestLocker()
	defer func() { assert.Error(t, l.Close()).Nil() }()

	held, ok, err := l.TryAcquire(ctx, "job")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	assert.Error(t, held.Unlock(ctx)).Nil()
	assert.Error(t, held.Unlock(ctx)).Nil()

	select {
	case <-held.Lost():
	case <-time.After(time.Second):
		t.Fatal("Lost() not closed after Unlock")
	}
}

// TestAcquireBlocksUntilReleased proves the blocking Acquire returns once the
// current holder releases, exercising the retry loop.
func TestAcquireBlocksUntilReleased(t *testing.T) {
	ctx := context.Background()
	l := newTestLocker()
	defer func() { assert.Error(t, l.Close()).Nil() }()

	first, ok, err := l.TryAcquire(ctx, "job")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	got := make(chan lock.Lock, 1)
	go func() {
		h, err := l.Acquire(ctx, "job", lock.WithToken("waiter"), lock.WithRetryInterval(20*time.Millisecond))
		if err == nil {
			got <- h
		}
	}()

	// The waiter must not proceed while the lock is held.
	select {
	case <-got:
		t.Fatal("Acquire returned while lock was held")
	case <-time.After(100 * time.Millisecond):
	}

	assert.Error(t, first.Unlock(ctx)).Nil()

	select {
	case h := <-got:
		assert.String(t, h.Token()).Equal("waiter")
		assert.Error(t, h.Unlock(ctx)).Nil()
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire did not return after release")
	}
}

// TestElectionElectsOneLeader proves stdlib/lock's Election runs unchanged on
// the K8s-Lease backend: exactly one campaigner becomes leader.
func TestElectionElectsOneLeader(t *testing.T) {
	l := newTestLocker()
	defer func() { assert.Error(t, l.Close()).Nil() }()

	elected := make(chan struct{}, 1)
	e := lock.NewElection(lock.ElectionConfig{
		Locker:        l,
		Key:           "leader",
		RetryInterval: 20 * time.Millisecond,
		OnElected:     func(ctx context.Context) { elected <- struct{}{} },
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.Run(ctx) }()

	select {
	case <-elected:
		assert.That(t, e.IsLeader()).True()
	case <-time.After(2 * time.Second):
		t.Fatal("no leader elected")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("election did not stop")
	}
}
