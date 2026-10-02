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

package StarterBigCache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
)

// TestCommandSurface pins what the three observed operations do to the cache,
// and that they stay below the abstraction: the wrapper returns bigcache's own
// sentinel, and it is [NewByteCache] that folds it into [cache.ErrMiss] for the
// façade above.
func TestCommandSurface(t *testing.T) {
	c := newTestCache(t, "hot") // holds k=v
	defer func() { _ = c.Destroy() }()

	if b, err := c.Get("k"); err != nil || string(b) != "v" {
		t.Fatalf("Get(k) = %q, %v; want \"v\", nil", b, err)
	}
	if _, err := c.Get("absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Get(absent) = %v, want bigcache.ErrEntryNotFound", err)
	}

	if err := c.Set("k2", []byte("v2")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b, err := c.Get("k2"); err != nil || string(b) != "v2" {
		t.Fatalf("Get(k2) = %q, %v; want \"v2\", nil", b, err)
	}

	if err := c.Delete("k2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Deleting an absent key reports the sentinel here; the adapter is what
	// turns it into a no-op.
	if err := c.Delete("absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Delete(absent) = %v, want bigcache.ErrEntryNotFound", err)
	}
}

// TestDelegations pins that the raw methods are re-exposed unchanged: they are
// passthroughs, so each returns exactly what the underlying cache returns.
func TestDelegations(t *testing.T) {
	conf := bigcache.DefaultConfig(time.Minute)
	conf.Shards = 16
	// Per-key statistics are what Stats() and KeyMetadata() read, so the
	// instance under test has them on.
	conf.StatsEnabled = true

	raw, err := bigcache.New(context.Background(), conf)
	if err != nil {
		t.Fatalf("bigcache.New: %v", err)
	}
	c, err := NewCache(raw, "hot")
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	defer func() { _ = c.Destroy() }()

	if err := c.Set("k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := c.Get("k"); err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if _, err := c.Get("absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Get miss: %v", err)
	}

	if got := c.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1", got)
	}
	if got := c.Capacity(); got <= 0 {
		t.Fatalf("Capacity = %d, want > 0", got)
	}

	stats := c.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("Stats = %+v, want 1 hit and 1 miss", stats)
	}
	if got := c.KeyMetadata("k").RequestCount; got == 0 {
		t.Fatal("KeyMetadata(k).RequestCount = 0, want the access to be recorded")
	}

	entries := 0
	it := c.Iterator()
	for it.SetNext() {
		entries++
	}
	if entries != 1 {
		t.Fatalf("Iterator walked %d entries, want 1", entries)
	}

	if err := c.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if got := c.Len(); got != 0 {
		t.Fatalf("Len after Reset = %d, want 0", got)
	}
}

// TestCloseAndDestroyAreAlternatives pins the hazard the delegate carries:
// bigcache's Close is not idempotent (it closes a channel), so a cache handed to
// the container must not be closed by hand — the container's Destroy is what
// closes it, and a second close panics.
func TestCloseAndDestroyAreAlternatives(t *testing.T) {
	raw, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	if err != nil {
		t.Fatalf("bigcache.New: %v", err)
	}
	c, err := NewCache(raw, "hot")
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Error("expected the second close to panic — Close and Destroy are alternatives, not a sequence")
		}
	}()
	_ = c.Destroy()
}
