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

// bytecache.go adapts the starter's *Client to the shared [cache.ByteCache]
// primitives the "memcached" cache driver layers its typed [cache.Cache] façade
// over.

package StarterMemcached

import (
	"context"
	"errors"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/cache"
)

type byteCache struct{ c *Client }

// NewByteCache wraps a *Client as a [cache.ByteCache]. Every operation flows
// through the wrapper's command seam (observe span + duration metric + access
// log + resilience), and honors ctx as far as gomemcache allows. The driver
// registered in the starter's root package selects the memcache client bean by
// beanID; call this directly to build a ByteCache for ad-hoc use.
func NewByteCache(c *Client) cache.ByteCache {
	return &byteCache{c}
}

// toExp converts a ttl in whole seconds to memcached's int32-seconds
// expiration. A non-positive value means "never expire" (0).
func toExp(ttlSeconds int) int32 {
	if ttlSeconds <= 0 {
		return 0
	}
	return int32(ttlSeconds)
}

// GetBytes returns the raw bytes under key, or (nil, [cache.ErrMiss]) when the
// key is absent.
func (m *byteCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	item, err := m.c.Get(ctx, key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return item.Value, nil
}

// SetBytes stores the raw bytes under key for ttlSeconds, a whole number of
// seconds. A non-positive value means the entry does not expire.
func (m *byteCache) SetBytes(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	return m.c.Set(ctx, &memcache.Item{Key: key, Value: val, Expiration: toExp(ttlSeconds)})
}

// Delete removes key. Deleting an absent key is not an error.
func (m *byteCache) Delete(ctx context.Context, key string) error {
	err := m.c.Delete(ctx, key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil
	}
	return err
}
