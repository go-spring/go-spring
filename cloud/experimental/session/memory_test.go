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

package session

import (
	"context"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// entryCount reports how many sessions the store currently holds. The background
// sweeper only ever removes records, so reading it under the store's lock is
// enough to observe a sweep.
func entryCount(m *Memory) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// waitUntil polls cond until it holds or the deadline passes. The sweeper runs on
// its own goroutine, so its effect is observed by waiting rather than by reading
// a fixed clock.
func waitUntil(t *testing.T, deadline time.Duration, cond func() bool) bool {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// An expired entry must be reclaimed even when nobody ever reads it again — the
// case a distributed backend covers with its key TTL.
func TestMemoryCleanupSweepsExpiredWithoutLoad(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	store.StartCleanup(10 * time.Millisecond)
	defer store.Stop()

	s := newSession()
	assert.That(t, store.Save(ctx, s, 20*time.Millisecond)).Nil()
	assert.That(t, entryCount(store)).Equal(1)

	assert.That(t, waitUntil(t, time.Second, func() bool { return entryCount(store) == 0 })).True()

	// The id is still absent for a reader, not just trimmed from the map.
	_, found, err := store.Load(ctx, s.ID())
	assert.That(t, err).Nil()
	assert.That(t, found).False()
}

// A non-positive interval disables the sweeper: expiry stays lazy (dropped on the
// next Load) and the store keeps records that nothing reads.
func TestMemoryCleanupDisabled(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	store.StartCleanup(0)
	store.Stop() // no sweeper to stop: must be a no-op, not a panic

	assert.That(t, store.sweep).Nil()

	s := newSession()
	assert.That(t, store.Save(ctx, s, 10*time.Millisecond)).Nil()
	time.Sleep(50 * time.Millisecond)
	assert.That(t, entryCount(store)).Equal(1)

	// The lazy path still hides the expired session and reclaims it.
	_, found, err := store.Load(ctx, s.ID())
	assert.That(t, err).Nil()
	assert.That(t, found).False()
	assert.That(t, entryCount(store)).Equal(0)
}

// StartCleanup is one-shot and Stop is idempotent; both are safe to call more
// than once and in either order.
func TestMemoryCleanupLifecycle(t *testing.T) {
	store := NewMemory()
	store.StartCleanup(50 * time.Millisecond)
	store.StartCleanup(50 * time.Millisecond) // second call keeps the first sweeper

	first := store.sweep
	assert.That(t, first != nil).True()

	store.Stop()
	store.Stop() // idempotent
	assert.That(t, store.sweep).Equal(first)

	// Cleanup does not resume once stopped, and the store still serves traffic.
	store.StartCleanup(10 * time.Millisecond)
	assert.That(t, store.sweep).Equal(first)
}
