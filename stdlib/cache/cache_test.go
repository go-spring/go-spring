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

package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/stdlib/aspect"
	"go-spring.org/stdlib/cache"
	"go-spring.org/stdlib/testing/assert"
)

func TestKey(t *testing.T) {
	assert.That(t, cache.Key("user")).Equal("user")
	assert.That(t, cache.Key("user", 42, "profile")).Equal("user:42:profile")
	ns := cache.Namespace("order")
	assert.That(t, ns(7)).Equal("order:7")
}

func TestMemory(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory()

	_, ok, err := c.Get(ctx, "missing")
	assert.That(t, err).Nil()
	assert.That(t, ok).False()

	assert.That(t, c.Set(ctx, "k", 123, 0)).Nil()
	v, ok, err := c.Get(ctx, "k")
	assert.That(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, v).Equal(123)

	assert.That(t, c.Delete(ctx, "k")).Nil()
	_, ok, _ = c.Get(ctx, "k")
	assert.That(t, ok).False()
}

func TestMemoryExpiry(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory()
	assert.That(t, c.Set(ctx, "k", "v", time.Millisecond)).Nil()
	time.Sleep(5 * time.Millisecond)
	_, ok, _ := c.Get(ctx, "k")
	assert.That(t, ok).False()
}

// fakeByteStore is an in-memory ByteStore for exercising the codec path.
type fakeByteStore struct {
	m       map[string][]byte
	failGet bool
}

func newFakeByteStore() *fakeByteStore { return &fakeByteStore{m: map[string][]byte{}} }

func (f *fakeByteStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	if f.failGet {
		return nil, false, errors.New("boom")
	}
	v, ok := f.m[key]
	return v, ok, nil
}

func (f *fakeByteStore) Set(_ context.Context, key string, data []byte, _ time.Duration) error {
	f.m[key] = data
	return nil
}

func (f *fakeByteStore) Delete(_ context.Context, key string) error {
	delete(f.m, key)
	return nil
}

func TestFromByteStoreJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	c := cache.FromByteStore(newFakeByteStore(), nil) // nil -> JSONCodec

	assert.That(t, c.Set(ctx, "k", map[string]any{"a": float64(1)}, 0)).Nil()
	v, ok, err := c.Get(ctx, "k")
	assert.That(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, v).Equal(map[string]any{"a": float64(1)})

	_, ok, _ = c.Get(ctx, "absent")
	assert.That(t, ok).False()
}

func TestFromByteStoreErrorIsMiss(t *testing.T) {
	ctx := context.Background()
	fs := newFakeByteStore()
	fs.failGet = true
	c := cache.FromByteStore(fs, nil)
	_, ok, err := c.Get(ctx, "k")
	assert.That(t, ok).False()
	assert.That(t, err).NotNil()
}

func TestMultiLevelBackfill(t *testing.T) {
	ctx := context.Background()
	near := cache.NewMemory()
	far := cache.NewMemory()
	ml := cache.NewMultiLevel(time.Minute, near, far)

	// Seed only the far level; a read must hit far and backfill near.
	assert.That(t, far.Set(ctx, "k", "v", 0)).Nil()
	v, ok, err := ml.Get(ctx, "k")
	assert.That(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, v).Equal("v")

	nv, nok, _ := near.Get(ctx, "k")
	assert.That(t, nok).True()
	assert.That(t, nv).Equal("v")
}

func TestMultiLevelWriteThroughAndDelete(t *testing.T) {
	ctx := context.Background()
	near := cache.NewMemory()
	far := cache.NewMemory()
	ml := cache.NewMultiLevel(time.Minute, near, far)

	assert.That(t, ml.Set(ctx, "k", "v", 0)).Nil()
	_, ok, _ := near.Get(ctx, "k")
	assert.That(t, ok).True()
	_, ok, _ = far.Get(ctx, "k")
	assert.That(t, ok).True()

	assert.That(t, ml.Delete(ctx, "k")).Nil()
	_, ok, _ = near.Get(ctx, "k")
	assert.That(t, ok).False()
	_, ok, _ = far.Get(ctx, "k")
	assert.That(t, ok).False()
}

func TestMultiLevelPanicsWithoutLevels(t *testing.T) {
	assert.Panic(t, func() { cache.NewMultiLevel(time.Minute) }, "at least one level")
}

func TestRegistry(t *testing.T) {
	assert.Panic(t, func() { cache.Register("", cache.NewMemory()) }, "empty name")
	assert.Panic(t, func() { cache.Register("x", nil) }, "nil backend")

	cache.Register("mem-test", cache.NewMemory())
	assert.Panic(t, func() { cache.Register("mem-test", cache.NewMemory()) }, "already registered")

	_, ok := cache.Get("mem-test")
	assert.That(t, ok).True()
	_, err := cache.MustGet("nope")
	assert.That(t, err).NotNil()
}

func TestAsStoreBridgesAspectCache(t *testing.T) {
	ctx := context.Background()
	backing := cache.NewMemory()
	store := cache.AsStore(ctx, backing)

	calls := 0
	chain := aspect.NewChain(aspect.Cache(store,
		func(jp *aspect.Joinpoint) string { return "fixed-key" },
		time.Minute,
	))

	target := func(ctx context.Context) (int, error) {
		calls++
		return 99, nil
	}
	v1, err := aspect.Around(chain, ctx, "Compute", target)
	assert.That(t, err).Nil()
	assert.That(t, v1).Equal(99)

	// Second call must hit the cache and skip the target.
	v2, err := aspect.Around(chain, ctx, "Compute", target)
	assert.That(t, err).Nil()
	assert.That(t, v2).Equal(99)
	assert.That(t, calls).Equal(1)

	// The value is visible in the backing cache under the derived key.
	bv, ok, _ := backing.Get(ctx, "fixed-key")
	assert.That(t, ok).True()
	assert.That(t, bv).Equal(99)
}
