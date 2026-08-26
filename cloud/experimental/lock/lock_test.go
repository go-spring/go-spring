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
	"fmt"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/experimental/lock"
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

func TestMemoryConcurrentTryAcquireSingleWinner(t *testing.T) {
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	// N goroutines race for the same key with distinct tokens: exactly one must
	// win, and the winner's token must be the one recorded on the handle.
	const n = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	var held lock.Lock
	for i := range n {
		wg.Go(func() {
			l, ok, err := m.TryAcquire(ctx, "race", lock.WithToken(fmt.Sprintf("t%d", i)), lock.WithRenewInterval(-1))
			mu.Lock()
			defer mu.Unlock()
			if err == nil && ok {
				winners++
				held = l
			}
		})
	}
	wg.Wait()

	assert.That(t, winners).Equal(1)
	assert.That(t, held.Token()).NotEqual("")
	assert.Error(t, held.Unlock(ctx)).Nil()
}

func TestMemoryUnlockAfterTakeoverReturnsErrNotHeld(t *testing.T) {
	// Token fencing: the first holder's lease expires, a second acquisition
	// reclaims the key, and the stale holder's Unlock must surface ErrNotHeld
	// instead of silently releasing the new owner's lock.
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	stale, ok, _ := m.TryAcquire(ctx, "fence", lock.WithToken("stale"),
		lock.WithTTL(20*time.Millisecond), lock.WithRenewInterval(-1))
	assert.That(t, ok).True()

	time.Sleep(40 * time.Millisecond)
	fresh, ok, _ := m.TryAcquire(ctx, "fence", lock.WithToken("fresh"), lock.WithRenewInterval(-1))
	assert.That(t, ok).True()

	err := stale.Unlock(ctx)
	assert.Error(t, err).Is(lock.ErrNotHeld)

	// The takeover must not have released the fresh holder's lock.
	_, ok, _ = m.TryAcquire(ctx, "fence", lock.WithRenewInterval(-1))
	assert.That(t, ok).False()
	assert.Error(t, fresh.Unlock(ctx)).Nil()
}

func TestMemoryDistinctTokensPerAcquisition(t *testing.T) {
	// Without an explicit token each acquisition draws its own fencing token.
	m := lock.NewMemoryLocker()
	defer m.Close()
	ctx := context.Background()

	l1, _, _ := m.TryAcquire(ctx, "k1", lock.WithRenewInterval(-1))
	l2, _, _ := m.TryAcquire(ctx, "k2", lock.WithRenewInterval(-1))
	defer l2.Unlock(ctx)
	defer l1.Unlock(ctx)
	assert.That(t, l1.Token()).NotEqual("")
	assert.That(t, l1.Token()).NotEqual(l2.Token())
}

func TestMemoryCloseStopsRenew(t *testing.T) {
	// After Close, renewal stops: a held lock with a short TTL expires on its
	// own even though the handle was never unlocked.
	m := lock.NewMemoryLocker()
	ctx := context.Background()

	l, ok, _ := m.TryAcquire(ctx, "k", lock.WithTTL(40*time.Millisecond), lock.WithRenewInterval(10*time.Millisecond))
	assert.That(t, ok).True()
	_ = l

	assert.Error(t, m.Close()).Nil()
	time.Sleep(80 * time.Millisecond)

	_, ok, _ = m.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.That(t, ok).True() // lease expired once renew loops stopped
}

func TestMemoryCloseIdempotent(t *testing.T) {
	m := lock.NewMemoryLocker()
	assert.Error(t, m.Close()).Nil()
	assert.Error(t, m.Close()).Nil() // second Close is a no-op
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
