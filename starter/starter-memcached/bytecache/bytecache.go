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

package bytecache

import (
	"context"
	"errors"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/cache"
)

// NewByteCache wraps a *memcache.Client as a [cache.ByteCache] - the raw
// bytes-native primitives the "memcached" driver layers a typed [cache.Cache]
// façade over. The driver registered in the starter's root package selects the
// memcache client bean by beanID; call this directly to build a ByteCache for
// ad-hoc use.
func NewByteCache(c *memcache.Client) cache.ByteCache {
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

// GetBytes returns the raw bytes under key. A missing key is reported as
// (nil, [cache.ErrMiss]).
func (m *memcachedCache) GetBytes(_ context.Context, key string) ([]byte, error) {
	item, err := m.c.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return item.Value, nil
}

// SetBytes stores the raw bytes under key for ttl. ttl is in seconds
// (sub-second rounded up to 1s); a non-positive ttl means the entry never
// expires.
func (m *memcachedCache) SetBytes(_ context.Context, key string, val []byte, ttl time.Duration) error {
	return m.c.Set(&memcache.Item{Key: key, Value: val, Expiration: toExp(ttl)})
}

// Delete removes key. Deleting an absent key is not an error.
func (m *memcachedCache) Delete(_ context.Context, key string) error {
	err := m.c.Delete(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil
	}
	return err
}
