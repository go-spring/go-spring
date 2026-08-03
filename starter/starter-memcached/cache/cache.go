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

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/spring/data/cache"
)

// NewCache wraps a *memcache.Client as a cache.Cache. The "memcached" driver
// registered in the starter's root package wires it over the memcache client
// bean selected by beanID; use it directly for programmatic construction too.
func NewCache(c *memcache.Client) cache.Cache {
	return &memcachedCache{c}
}

type memcachedCache struct{ c *memcache.Client }

// toExp converts a ttl to memcached's int32-seconds expiration. 0 means "never
// expire"; a positive sub-second ttl is rounded up to 1s so it is not silently
// treated as forever.
func toExp(ttl time.Duration) int32 {
	if ttl <= 0 {
		return 0
	}
	exp := int32(ttl.Seconds())
	if exp == 0 {
		exp = 1
	}
	return exp
}

// Get decodes the value under key into val (a pointer) using codec (default
// JSON). A missing key is reported as [cache.ErrMiss].
func (m *memcachedCache) Get(ctx context.Context, key string, val any, codec ...cache.Codec) error {
	item, err := m.c.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return cache.ErrMiss
	}
	if err != nil {
		return err
	}
	return cache.ResolveCodec(codec).Unmarshal(item.Value, val)
}

// GetBytes returns the raw bytes under key. A missing key is reported as
// (nil, [cache.ErrMiss]).
func (m *memcachedCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	item, err := m.c.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return item.Value, nil
}

// Set encodes val with codec (default JSON) and stores it under key for ttl.
// ttl is in seconds (sub-second rounded up to 1s); a non-positive ttl means the
// entry never expires.
func (m *memcachedCache) Set(ctx context.Context, key string, val any, ttl time.Duration, codec ...cache.Codec) error {
	data, err := cache.ResolveCodec(codec).Marshal(val)
	if err != nil {
		return err
	}
	return m.c.Set(&memcache.Item{Key: key, Value: data, Expiration: toExp(ttl)})
}

// SetBytes stores the raw bytes under key for ttl (see Set for ttl semantics).
func (m *memcachedCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return m.c.Set(&memcache.Item{Key: key, Value: val, Expiration: toExp(ttl)})
}

// Delete removes key. Deleting an absent key is not an error.
func (m *memcachedCache) Delete(ctx context.Context, key string) error {
	err := m.c.Delete(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil
	}
	return err
}
