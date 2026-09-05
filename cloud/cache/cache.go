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

// Package cache defines a backend-pluggable abstraction for key/value caching.
//
// A backend implements [ByteCache], the raw byte primitives a remote client
// maps 1:1 to its native API. [Cache] wraps a ByteCache and adds typed
// Get/Set through a pluggable [Codec], defaulting to [JSONCodec]; the raw
// methods stay promoted for callers that already hold bytes. A missing key is
// reported as [ErrMiss], distinct from a backend error, so a read-through
// falls back to the source of truth only on a real miss.
//
// The package is container-free: the driver registry and the ${spring.cache}
// module that turn config entries into Cache beans live in starter-cache.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss is returned by Get/GetBytes when the key is absent. It is distinct
// from a backend error: a caller falls back to the source of truth only on a
// miss, never on a real failure.
var ErrMiss = errors.New("cache: miss")

// ByteCache is the bytes-native interface a caching backend implements.
// Implementations must be safe for concurrent use; a nil ByteCache is never
// valid.
type ByteCache interface {
	// GetBytes returns the raw bytes stored under key, or (nil, [ErrMiss])
	// when the key is absent.
	GetBytes(ctx context.Context, key string) ([]byte, error)

	// SetBytes stores the raw bytes under key. A non-positive ttl means the
	// entry does not expire.
	SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// Delete removes key. Deleting an absent key is not an error.
	Delete(ctx context.Context, key string) error
}

// Cache is the typed façade over a [ByteCache]: it embeds the backend and adds
// Get/Set, which marshal through the [Codec] fixed at construction. Since the
// embedded ByteCache must be safe for concurrent use, a Cache wrapping it is
// too.
type Cache struct {
	ByteCache

	// cfg is fixed once at [New]; a per-call option works on a copy.
	cfg config
}

// config carries a [Cache]'s configured behaviour.
type config struct {
	codec Codec
}

// Option customizes a [Cache]. Passed to [New] it sets the cache's default;
// passed to [Cache.Get] / [Cache.Set] it overrides that single call.
type Option func(*config)

// WithCodec sets the codec the typed Get/Set marshal through. A nil codec is
// ignored, so passing a conditionally resolved codec needs no guard.
func WithCodec(c Codec) Option {
	return func(cfg *config) {
		if c != nil {
			cfg.codec = c
		}
	}
}

// New wraps bc in a [Cache]: the config starts at the defaults (JSON codec)
// and opts replace them.
func New(bc ByteCache, opts ...Option) *Cache {
	cfg := config{codec: JSONCodec{}}
	for _, o := range opts {
		o(&cfg)
	}
	return &Cache{ByteCache: bc, cfg: cfg}
}

// configFor copies the cache's config and applies the per-call opts on top.
func (c Cache) configFor(opts []Option) config {
	cfg := c.cfg
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// Get decodes the value stored under key into val, which must be a pointer.
// A missing key is reported as [ErrMiss]; any other error is a backend
// failure.
//
// opts override the cache's config for this call:
//
//	c.Get(ctx, "icon:42", &icon, cache.WithCodec(gobCodec))
func (c Cache) Get(ctx context.Context, key string, val any, opts ...Option) error {
	b, err := c.GetBytes(ctx, key)
	if err != nil {
		return err
	}
	return c.configFor(opts).codec.Unmarshal(b, val)
}

// Set encodes val and stores it under key. A non-positive ttl means the entry
// does not expire. opts override the cache's config for this call, as on
// [Cache.Get].
func (c Cache) Set(ctx context.Context, key string, val any, ttl time.Duration, opts ...Option) error {
	b, err := c.configFor(opts).codec.Marshal(val)
	if err != nil {
		return err
	}
	return c.SetBytes(ctx, key, b, ttl)
}
