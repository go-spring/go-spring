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

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/cache"
)

// NewByteCache wraps a *bigcache.BigCache as a [cache.ByteCache] - the raw
// bytes-native primitives the "bigcache" driver layers a typed [cache.Cache]
// façade over. The driver registered in the starter's root package selects the
// BigCache bean by beanID; call this directly to build a ByteCache for ad-hoc
// use.
func NewByteCache(c *bigcache.BigCache) cache.ByteCache {
	return &bigcacheCache{c}
}

type bigcacheCache struct{ c *bigcache.BigCache }

// GetBytes returns the raw bytes under key. A missing key is reported as
// (nil, [cache.ErrMiss]).
//
// Note: BigCache expires entries by a single global LifeWindow set at
// construction, not per entry, so SetBytes ignores the per-call TTL.
func (b *bigcacheCache) GetBytes(_ context.Context, key string) ([]byte, error) {
	data, err := b.c.Get(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SetBytes stores the raw bytes under key. ttl is ignored: BigCache expires
// entries by the global LifeWindow, not per entry - configure lifetime via
// ${spring.bigcache} LifeWindow instead.
func (b *bigcacheCache) SetBytes(_ context.Context, key string, val []byte, _ time.Duration) error {
	return b.c.Set(key, val)
}

// Delete removes key. Deleting an absent key is not an error.
func (b *bigcacheCache) Delete(_ context.Context, key string) error {
	err := b.c.Delete(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil
	}
	return err
}
