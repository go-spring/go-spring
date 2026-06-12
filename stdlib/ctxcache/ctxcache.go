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

// Package ctxcache provides a strongly-typed, context-scoped cache for
// request-scoped data.
//
// ctxcache attaches a concurrency-safe, write-once key–value store to a
// context.Context, allowing values to be implicitly propagated across call
// boundaries without polluting function signatures.
//
// Keys are identified by a combination of a string name and a Go type via
// generics, ensuring type safety and preventing collisions between values
// of different types—even if they share the same string identifier.
//
// Each key may be assigned exactly once. After a value is set, it can be
// retrieved multiple times until the cache is cleared.
//
// The cache lifecycle is explicitly controlled by a cancel function returned
// by Init. When the cancel function is called, all cached values are removed
// and the cache becomes permanently unusable.
//
// ctxcache is not a general-purpose cache. It is designed for structured,
// short-lived, in-process data bound to a context's lifetime, such as
// authenticated users, permissions, trace metadata, or computed intermediates.
//
// Typical usage:
//
//  1. Initialize the cache at the request boundary (e.g. HTTP middleware).
//  2. Set each value at most once using Set.
//  3. Retrieve values using Get in downstream code.
//  4. Defer the cancel function returned by Init to clean up request-scoped data.
package ctxcache

import (
	"context"
	"fmt"
	"sync"

	"go-spring.org/stdlib/errutil"
)

var (
	ErrCacheNotInitialized = errutil.Explain(nil, "cache not initialized")
	ErrCacheAlreadyCleared = errutil.Explain(nil, "cache already cleared")
	ErrKeyNotSet           = errutil.Explain(nil, "key not set")
	ErrKeyAlreadySet       = errutil.Explain(nil, "key already set")
)

type cacheKeyType struct{}

var cacheKey = cacheKeyType{}

// getCache retrieves the Cache attached to the given context, if any.
func getCache(ctx context.Context) (*Cache, bool) {
	cache, ok := ctx.Value(&cacheKey).(*Cache)
	return cache, ok
}

// Cache holds context-scoped data associated with a context.Context.
//
// Internally, Cache maintains a mutex-protected map keyed by strongly typed
// keys. The cache is propagated through the context without appearing in
// function signatures, making it suitable for passing data across modules.
//
// A Cache is write-once per key: each key may be assigned a value exactly once.
// Values can be read multiple times until the cache is cleared.
//
// Once cleared, the cache becomes permanently unusable; subsequent Get or Set
// operations will return ErrCacheAlreadyCleared.
type Cache struct {
	mutex   sync.Mutex
	values  map[any]any
	cleared bool
}

// Clear removes all cached values and marks the cache as cleared.
//
// Clear is idempotent. Only the first call performs cleanup; subsequent calls
// have no effect.
//
// After Clear is called, the cache is permanently unusable. All future Get or
// Set operations will return ErrCacheAlreadyCleared.
func (cache *Cache) Clear() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return
	}

	cache.values = nil
	cache.cleared = true
}

// Init attaches a Cache to the given context and returns the new context along
// with a cancel function.
//
// Only one Cache may be attached to a context. Repeated calls to Init with the
// same context are safe: if a Cache already exists, Init returns the original
// context and a no-op cancel function.
//
// The returned cancel function should typically be deferred at the request
// boundary (e.g. in HTTP middleware) to ensure request-scoped data is cleaned up.
//
// When a Cache is newly created, the cancel function clears the Cache.
// Calling the cancel function multiple times is safe.
func Init(ctx context.Context) (_ context.Context, cancel func()) {
	if _, ok := getCache(ctx); ok {
		return ctx, func() {}
	}
	m := &Cache{values: make(map[any]any)}
	return context.WithValue(ctx, &cacheKey, m), m.Clear
}

// TypedKey represents a strongly typed cache key.
//
// A TypedKey is defined by a string identifier and a Go type parameter.
// Keys with the same string but different type parameters are considered
// distinct and do not collide.
type TypedKey[T any] struct {
	Key string
}

func (k TypedKey[T]) String() string {
	var zero T
	return fmt.Sprintf("%s(%T)", k.Key, zero)
}

// Get retrieves the value associated with the given key.
//
// Returns an error if:
//   - the cache is not initialized,
//   - the cache has already been cleared, or
//   - no value has been set for the given key.
func Get[T any](ctx context.Context, key string) (T, error) {
	var zero T

	k := TypedKey[T]{Key: key}
	cache, ok := getCache(ctx)
	if !ok {
		return zero, errutil.Explain(ErrCacheNotInitialized, "ctxcache: get %s error", k)
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return zero, errutil.Explain(ErrCacheAlreadyCleared, "ctxcache: get %s error", k)
	}

	v, ok := cache.values[k]
	if !ok {
		return zero, errutil.Explain(ErrKeyNotSet, "ctxcache: get %s error", k)
	}

	return v.(T), nil
}

// Set assigns a value to the given key.
//
// Each key may be assigned exactly once. Subsequent attempts to set the same
// key return ErrKeyAlreadySet.
//
// Returns an error if:
//   - the cache is not initialized, or
//   - the cache has already been cleared.
func Set[T any](ctx context.Context, key string, value T) error {
	k := TypedKey[T]{Key: key}
	cache, ok := getCache(ctx)
	if !ok {
		return errutil.Explain(ErrCacheNotInitialized, "ctxcache: set %s error", k)
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	if cache.cleared {
		return errutil.Explain(ErrCacheAlreadyCleared, "ctxcache: set %s error", k)
	}

	if _, ok = cache.values[k]; ok {
		return errutil.Explain(ErrKeyAlreadySet, "ctxcache: set %s error", k)
	}

	cache.values[k] = value
	return nil
}
