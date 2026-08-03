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

package cache

import (
	"context"
	"errors"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/spring/data/cache"
)

// NewCache wraps a *bigcache.BigCache as a cache.Cache. The "bigcache" driver
// registered in the starter's root package wires it over the BigCache bean
// selected by beanID; use it directly for programmatic construction too.
func NewCache(c *bigcache.BigCache) cache.Cache {
	return &bigcacheCache{c}
}

type bigcacheCache struct{ c *bigcache.BigCache }

// Get decodes the value under key into val (a pointer) using codec (default
// JSON). A missing key is reported as [cache.ErrMiss].
//
// Note: BigCache expires entries by a single global LifeWindow set at
// construction, not per entry, so there is no per-call TTL on Set/SetBytes.
func (b *bigcacheCache) Get(ctx context.Context, key string, val any, codec ...cache.Codec) error {
	data, err := b.c.Get(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return cache.ErrMiss
	}
	if err != nil {
		return err
	}
	return cache.ResolveCodec(codec).Unmarshal(data, val)
}

// GetBytes returns the raw bytes under key. A missing key is reported as
// (nil, [cache.ErrMiss]).
func (b *bigcacheCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	data, err := b.c.Get(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Set encodes val with codec (default JSON) and stores it under key. ttl is
// ignored: BigCache expires entries by the global LifeWindow, not per entry —
// configure lifetime via ${spring.bigcache} LifeWindow instead.
func (b *bigcacheCache) Set(ctx context.Context, key string, val any, ttl time.Duration, codec ...cache.Codec) error {
	data, err := cache.ResolveCodec(codec).Marshal(val)
	if err != nil {
		return err
	}
	return b.c.Set(key, data)
}

// SetBytes stores the raw bytes under key. ttl is ignored (see Set).
func (b *bigcacheCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return b.c.Set(key, val)
}

// Delete removes key. Deleting an absent key is not an error.
func (b *bigcacheCache) Delete(ctx context.Context, key string) error {
	err := b.c.Delete(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil
	}
	return err
}
