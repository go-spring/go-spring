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
	"testing"
	"time"

	"go-spring.org/spring/lock"
	"go-spring.org/stdlib/testing/assert"
)

func TestApplyDefaults(t *testing.T) {
	o := lock.Apply()
	assert.That(t, o.TTL).Equal(30 * time.Second)
	assert.That(t, o.RenewInterval).Equal(10 * time.Second) // TTL/3
	assert.That(t, o.RetryInterval).Equal(100 * time.Millisecond)
	assert.That(t, o.Token).NotEqual("")
}

func TestApplyOverrides(t *testing.T) {
	o := lock.Apply(
		lock.WithTTL(9*time.Second),
		lock.WithRenewInterval(-1),
		lock.WithRetryInterval(5*time.Millisecond),
		lock.WithToken("fixed"),
	)
	assert.That(t, o.TTL).Equal(9 * time.Second)
	assert.That(t, o.RenewInterval).Equal(time.Duration(-1)) // renew disabled
	assert.That(t, o.RetryInterval).Equal(5 * time.Millisecond)
	assert.That(t, o.Token).Equal("fixed")
}

func TestMemoryTryAcquireContended(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	l, ok, err := m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, l.Key()).Equal("k")

	_, ok, err = m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).False() // held

	assert.Error(t, l.Unlock(ctx)).Nil()

	_, ok, err = m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True() // free again
}

func TestMemoryUnlockIdempotent(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()
	l, _, _ := m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, l.Unlock(ctx)).Nil()
	assert.Error(t, l.Unlock(ctx)).Nil() // second unlock is a no-op
}

func TestMemoryUnlockClosesLost(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()
	l, _, _ := m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	_ = l.Unlock(ctx)
	select {
	case <-l.Lost():
	case <-time.After(time.Second):
		t.Fatal("Lost() not closed after Unlock")
	}
}

func TestMemoryExpiryReclaims(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	// Short TTL, renew disabled: the lock expires and can be reclaimed, and the
	// original holder is notified via Lost().
	l, ok, _ := m.TryAcquire(ctx, "k", lock.WithTTL(30*time.Millisecond), lock.WithRenewInterval(-1))
	assert.That(t, ok).True()

	time.Sleep(60 * time.Millisecond)
	l2, ok, _ := m.TryAcquire(ctx, "k", lock.WithTTL(time.Second), lock.WithRenewInterval(-1))
	assert.That(t, ok).True() // reclaimed after expiry
	select {
	case <-l.Lost():
	case <-time.After(time.Second):
		t.Fatal("expired holder not notified via Lost()")
	}
	_ = l2.Unlock(ctx)
}

func TestMemoryRenewKeepsOwnership(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	// TTL short but auto-renew on: ownership must persist past one TTL.
	l, ok, _ := m.TryAcquire(ctx, "k", lock.WithTTL(40*time.Millisecond), lock.WithRenewInterval(10*time.Millisecond))
	assert.That(t, ok).True()
	defer l.Unlock(ctx)

	time.Sleep(120 * time.Millisecond)
	_, ok, _ = m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.That(t, ok).False() // still held thanks to renew
}

func TestMemoryAcquireBlocksUntilReleased(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	l, _, _ := m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))

	acquired := make(chan lock.Lock, 1)
	go func() {
		l2, err := m.Acquire(ctx, "k", lock.WithRetryInterval(5*time.Millisecond), lock.WithRenewInterval(-1))
		if err == nil {
			acquired <- l2
		}
	}()

	select {
	case <-acquired:
		t.Fatal("Acquire returned while lock still held")
	case <-time.After(30 * time.Millisecond):
	}

	_ = l.Unlock(ctx)
	select {
	case l2 := <-acquired:
		_ = l2.Unlock(ctx)
	case <-time.After(time.Second):
		t.Fatal("Acquire did not proceed after release")
	}
}

func TestMemoryAcquireHonoursContext(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	held, _, _ := m.TryAcquire(context.Background(), "k", lock.WithRenewInterval(-1))
	defer held.Unlock(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := m.Acquire(ctx, "k", lock.WithRetryInterval(5*time.Millisecond), lock.WithRenewInterval(-1))
	assert.Error(t, err).NotNil()
}
