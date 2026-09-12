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

package StarterLockRedis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"go-spring.org/cloud/lock"
	"go-spring.org/stdlib/testing/assert"
)

// newTestLocker starts an in-process Redis (miniredis) and builds a redisLocker
// over it, bypassing the gs wiring so the lock semantics run standalone. The
// stop func shuts the server down when the test is done.
func newTestLocker(t *testing.T, cfg Config) (*redisLocker, *miniredis.Miniredis, func()) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	l := &redisLocker{cfg: cfg, client: client, stop: make(chan struct{})}
	return l, srv, func() {
		_ = l.Close()
		_ = client.Close()
	}
}

func TestRedis_TryAcquireContendedThenReleased(t *testing.T) {
	l, _, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	held, ok, err := l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, held.Key()).Equal("k")
	assert.That(t, held.Token()).NotEqual("")

	_, ok, err = l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).False() // contended

	assert.Error(t, held.Unlock(ctx)).Nil()
	held2, ok, err := l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True() // free again
	assert.That(t, held2.Token()).NotEqual(held.Token())
	assert.Error(t, held2.Unlock(ctx)).Nil()
}

func TestRedis_KeyPrefixSeparatesApps(t *testing.T) {
	l, _, done := newTestLocker(t, Config{KeyPrefix: "app1:"})
	defer done()
	l2 := &redisLocker{cfg: Config{KeyPrefix: "app2:"}, client: l.client, stop: make(chan struct{})}
	defer func() { _ = l2.Close() }()
	ctx := context.Background()

	h1, ok, err := l.TryAcquire(ctx, "jobs", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	defer h1.Unlock(ctx)

	// A different prefix means a different Redis key: no contention.
	h2, ok, err := l2.TryAcquire(ctx, "jobs", lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.Error(t, h2.Unlock(ctx)).Nil()
}

func TestRedis_UnlockIdempotent(t *testing.T) {
	l, _, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	held, _, _ := l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.Error(t, held.Unlock(ctx)).Nil()
	assert.Error(t, held.Unlock(ctx)).Nil() // second Unlock is a no-op
	select {
	case <-held.Lost():
	case <-time.After(time.Second):
		t.Fatal("Lost() not closed after Unlock")
	}
}

// TestRedis_TokenFencingAfterTakeover proves the compare-and-DEL fencing: when
// the lease expired and another owner took the key, the stale holder's Unlock
// reports lock.ErrNotHeld and does not delete the new owner's key.
func TestRedis_TokenFencingAfterTakeover(t *testing.T) {
	l, srv, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	stale, ok, err := l.TryAcquire(ctx, "k", lock.WithToken("stale"),
		lock.WithTTL(30*time.Millisecond), lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	// FastForward expires miniredis TTLs deterministically, no wall-clock sleep.
	srv.FastForward(60 * time.Millisecond)
	fresh, ok, err := l.TryAcquire(ctx, "k", lock.WithToken("fresh"), lock.WithRenewInterval(-1))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	assert.Error(t, stale.Unlock(ctx)).Is(lock.ErrNotHeld)

	// The stale unlock must not have released the fresh hold.
	_, ok, _ = l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.That(t, ok).False()
	assert.Error(t, fresh.Unlock(ctx)).Nil()
}

// TestRedis_RenewKeepsOwnership proves the compare-and-PEXPIRE loop: with a
// short TTL and auto-renew on, ownership outlives several TTLs.
func TestRedis_RenewKeepsOwnership(t *testing.T) {
	l, _, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	held, ok, err := l.TryAcquire(ctx, "k",
		lock.WithTTL(60*time.Millisecond), lock.WithRenewInterval(20*time.Millisecond))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	defer held.Unlock(ctx)

	time.Sleep(200 * time.Millisecond) // several TTLs elapse
	_, ok, _ = l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))
	assert.That(t, ok).False() // renew loop kept the lease alive
}

// TestRedis_LostFiresWhenKeyVanishes proves the renew loop detects a takeover
// (key gone or re-owned) and closes Lost() so the critical section can bail.
func TestRedis_LostFiresWhenKeyVanishes(t *testing.T) {
	l, _, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	held, ok, err := l.TryAcquire(ctx, "k",
		lock.WithTTL(30*time.Second), lock.WithRenewInterval(20*time.Millisecond))
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()

	// Simulate an external takeover: delete the key out from under the holder.
	assert.Error(t, l.client.Del(ctx, "k").Err()).Nil()

	select {
	case <-held.Lost():
	case <-time.After(2 * time.Second):
		t.Fatal("Lost() not closed after the key vanished")
	}
}

func TestRedis_AcquireBlocksUntilReleased(t *testing.T) {
	l, _, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	held, _, _ := l.TryAcquire(ctx, "k", lock.WithRenewInterval(-1))

	acquired := make(chan lock.Lock, 1)
	go func() {
		l2, err := l.Acquire(ctx, "k", lock.WithRetryInterval(10*time.Millisecond), lock.WithRenewInterval(-1))
		if err == nil {
			acquired <- l2
		}
	}()

	select {
	case <-acquired:
		t.Fatal("Acquire returned while lock still held")
	case <-time.After(40 * time.Millisecond):
	}

	assert.Error(t, held.Unlock(ctx)).Nil()
	select {
	case l2 := <-acquired:
		assert.Error(t, l2.Unlock(ctx)).Nil()
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire did not proceed after release")
	}
}

// TestRedis_ConcurrentTryAcquireSingleWinner races many acquisitions for one
// key: exactly one wins and the winner's token owns the Redis key.
func TestRedis_ConcurrentTryAcquireSingleWinner(t *testing.T) {
	l, _, done := newTestLocker(t, Config{})
	defer done()
	ctx := context.Background()

	const n = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	var held lock.Lock
	for i := range n {
		wg.Go(func() {
			lk, ok, err := l.TryAcquire(ctx, "race",
				lock.WithToken(fmt.Sprintf("t%d", i)), lock.WithRenewInterval(-1))
			mu.Lock()
			defer mu.Unlock()
			if err == nil && ok {
				winners++
				held = lk
			}
		})
	}
	wg.Wait()

	assert.That(t, winners).Equal(1)
	assert.Error(t, held.Unlock(ctx)).Nil()
}
