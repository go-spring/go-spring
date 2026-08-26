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

// Package cache defines a backend-pluggable abstraction for
// key/value caching, so a caching concern can be declared once and served by
// any backend.
//
// A backend implements the single [ByteCache] interface: the raw
// [ByteCache.GetBytes]/[ByteCache.SetBytes]/[ByteCache.Delete] primitives a
// remote client maps 1:1 to its native API. A [Cache] struct wraps a ByteCache
// and layers a pluggable [Codec] (default [JSONCodec]) on top, exposing typed
// [Cache.Get]/[Cache.Set] that cross the bytes/any boundary. The codec is a
// construction attribute — set once via [WithCodec] on [New] — not a per-call
// parameter; a cache that must hold values in different formats uses the raw
// GetBytes/SetBytes with an explicit codec at the call site. A missing key is
// reported as [ErrMiss], distinct from a backend error, so callers fall through
// to the source of truth only on a real miss.
//
// This package is container-free: it imports no IoC concepts at all. The gs
// wiring that turns ${spring.cache} entries into Cache beans — the driver
// registry ("go-redis", "redigo", "bigcache", "memcached", ...) and the
// ${spring.cache} module — lives in the starter-cache module, next to nothing
// else; each backend starter registers its driver there.
// `spring.cache.main.driver=go-redis:main` exposes a Cache bean backed by the
// "main" redis client, exactly as before the split.
package cache

import (
	"context"
	"errors"
	"time"
)

// ByteCache is the bytes-native interface a caching backend implements: the
// raw primitives a remote client (Redis, memcached, bigcache) maps 1:1 to its
// native API. Implementations must be safe for concurrent use. A nil ByteCache
// is never valid; callers that want "no cache" should skip the lookup entirely
// rather than pass nil.
type ByteCache interface {
	// GetBytes returns the raw bytes stored under key. A missing key is
	// reported as (nil, [ErrMiss]).
	GetBytes(ctx context.Context, key string) ([]byte, error)

	// SetBytes stores the raw bytes under key for ttl. A non-positive ttl
	// means the entry does not expire.
	SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// Delete removes key. Deleting an absent key is not an error.
	Delete(ctx context.Context, key string) error
}

// Cache is the typed façade over a [ByteCache]. It embeds a ByteCache and adds
// [Cache.Get]/[Cache.Set], which cross the bytes/any boundary through the
// [Codec] fixed at construction (default [JSONCodec]); the raw
// [ByteCache.GetBytes]/[ByteCache.SetBytes]/[ByteCache.Delete] methods are
// promoted unchanged for callers that already hold bytes — or need to mix
// formats under one backend with an explicit codec. Since the embedded
// ByteCache must be safe for concurrent use, a Cache wrapping it is too.
type Cache struct {
	ByteCache

	// codec is resolved once at construction; a nil codec means [JSONCodec].
	codec Codec
}

// New wraps bc in a [Cache] with its codec fixed by opts (default
// [JSONCodec] via [WithCodec]). It is the canonical constructor: a Cache
// built by hand must at least set the embedded ByteCache before use.
func New(bc ByteCache, opts ...Option) *Cache {
	c := &Cache{ByteCache: bc}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Option customizes a [Cache] at construction.
type Option func(*Cache)

// WithCodec sets the [Codec] the [Cache]'s typed Get/Set use to cross the
// bytes/any boundary. A nil codec is ignored, so passing a conditionally
// resolved codec needs no guard.
func WithCodec(c Codec) Option {
	return func(cc *Cache) {
		if c != nil {
			cc.codec = c
		}
	}
}

// codecOr returns the cache's codec, or the default [JSONCodec] when none was
// set at construction.
func (c Cache) codecOr() Codec { return resolveCodec(c.codec) }

// Get decodes the value stored under key into val (which must be a pointer),
// using the cache's [Codec] (default [JSONCodec]) to cross the bytes/any
// boundary. A missing key is reported as [ErrMiss]; any other error is a
// backend failure. On a miss the caller typically falls through to the source
// of truth.
func (c Cache) Get(ctx context.Context, key string, val any) error {
	b, err := c.GetBytes(ctx, key)
	if err != nil {
		return err
	}
	return c.codecOr().Unmarshal(b, val)
}

// Set encodes val with the cache's [Codec] (default [JSONCodec]) and stores it
// under key for ttl. A non-positive ttl means the entry does not expire.
func (c Cache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	b, err := c.codecOr().Marshal(val)
	if err != nil {
		return err
	}
	return c.SetBytes(ctx, key, b, ttl)
}

// ErrMiss is returned by Get/GetBytes when the key is absent (a cache miss).
// It is distinct from a backend error (network, serialization, ...): callers
// fall through to the source of truth only on a miss, not on a real failure.
var ErrMiss = errors.New("cache: miss")
