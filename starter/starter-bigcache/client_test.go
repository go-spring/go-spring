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
	defer func() { _ = c.Close() }()

	if b, err := c.Get(context.Background(), "k"); err != nil || string(b) != "v" {
		t.Fatalf("Get(k) = %q, %v; want \"v\", nil", b, err)
	}
	if _, err := c.Get(context.Background(), "absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Get(absent) = %v, want bigcache.ErrEntryNotFound", err)
	}

	if err := c.Set(context.Background(), "k2", []byte("v2")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b, err := c.Get(context.Background(), "k2"); err != nil || string(b) != "v2" {
		t.Fatalf("Get(k2) = %q, %v; want \"v2\", nil", b, err)
	}

	if err := c.Delete(context.Background(), "k2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Deleting an absent key reports the sentinel here; the adapter is what
	// turns it into a no-op.
	if err := c.Delete(context.Background(), "absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
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
	defer func() { _ = c.Close() }()

	if err := c.Set(context.Background(), "k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := c.Get(context.Background(), "k"); err != nil {
		t.Fatalf("Get hit: %v", err)
	}
	if _, err := c.Get(context.Background(), "absent"); !errors.Is(err, bigcache.ErrEntryNotFound) {
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
}

// TestInnerChainReorganize pins the seam [Cache.Inner] opens: a custom layer
// wraps the head it found — keeping the observation layer under it — and the
// command surface then runs through it. The layer here rewrites keys, which is
// exactly the kind of behavior change no bigcache hook could express.
func TestInnerChainReorganize(t *testing.T) {
	c := newTestCache(t, "hot") // holds k=v
	defer func() { _ = c.Close() }()

	c.InnerCache = prefixLayer{InnerCache: c.InnerCache, prefix: "app:"}

	// The old key is invisible through the layer; the prefixed one is not.
	if _, err := c.Get(context.Background(), "k"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Get(k) = %v, want miss under the rewritten namespace", err)
	}
	if err := c.Set(context.Background(), "k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b, err := c.Get(context.Background(), "k"); err != nil || string(b) != "v" {
		t.Fatalf("Get(k) = %q, %v; want \"v\" in the rewritten namespace", b, err)
	}
	// The raw cache sees the prefixed key, proving the layer sits over it.
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (k and app:k)", c.Len())
	}
}

// TestInnerChainExternalRebuild pins the full outside-the-package rebuild
// protocol: shallow-close the old head (the reporting goes, the instance
// stays), build the new chain over the same instance layer with a custom layer
// in the middle, and take over Inner.
func TestInnerChainExternalRebuild(t *testing.T) {
	c := newTestCache(t, "hot") // holds k=v
	defer func() { _ = c.Close() }()

	if err := c.InnerCache.Release(false); err != nil {
		t.Fatalf("shallow close: %v", err)
	}
	raw := NewRawCache(c.Client)
	obs, err := NewObsCache(&prefixLayer{InnerCache: raw, prefix: "app:"}, c.Client, "hot")
	if err != nil {
		t.Fatalf("NewObsCache: %v", err)
	}
	c.InnerCache = obs

	if _, err := c.Get(context.Background(), "k"); !errors.Is(err, bigcache.ErrEntryNotFound) {
		t.Fatalf("Get(k) = %v, want miss under the rewritten namespace", err)
	}
	if err := c.Set(context.Background(), "k", []byte("v")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if b, err := c.Get(context.Background(), "k"); err != nil || string(b) != "v" {
		t.Fatalf("Get(k) = %q, %v; want \"v\" in the rewritten namespace", b, err)
	}
}

// prefixLayer namespaces every key it passes down. It embeds the head it found,
// so it inherits nothing to override — every operation here rewrites its key.
type prefixLayer struct {
	InnerCache
	prefix string
}

func (p prefixLayer) Get(ctx context.Context, key string) ([]byte, error) {
	return p.InnerCache.Get(ctx, p.prefix+key)
}

func (p prefixLayer) Set(ctx context.Context, key string, entry []byte) error {
	return p.InnerCache.Set(ctx, p.prefix+key, entry)
}

func (p prefixLayer) Delete(ctx context.Context, key string) error {
	return p.InnerCache.Delete(ctx, p.prefix+key)
}

// TestDoubleClosePanics pins the hazard the delegate carries: bigcache's Close
// is not idempotent (it closes a channel), and Close is the one teardown path —
// the container's destroy registration is the same method — so calling it twice
// panics.
func TestDoubleClosePanics(t *testing.T) {
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
			t.Error("expected the second close to panic — Close is not idempotent")
		}
	}()
	_ = c.Close()
}
