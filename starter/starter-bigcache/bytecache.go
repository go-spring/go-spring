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

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/cache"
)

type byteCache struct{ c *Cache }

// NewByteCache wraps a *Cache as a [cache.ByteCache] - the raw bytes-native
// primitives the "bigcache" driver layers a typed [cache.Cache] façade over.
// Every operation flows through the wrapper's command seam (declared operation +
// resilience; the span/access log are emitted there). The driver registered in the
// starter's root package selects the BigCache bean by beanID; call this directly
// to build a ByteCache for ad-hoc use.
func NewByteCache(c *Cache) cache.ByteCache {
	return &byteCache{c}
}

// GetBytes returns the raw bytes under key, or (nil, [cache.ErrMiss]) when the
// key is absent.
func (b *byteCache) GetBytes(_ context.Context, key string) ([]byte, error) {
	data, err := b.c.Get(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SetBytes stores the raw bytes under key for ttlSeconds, a whole number of
// seconds. BigCache ignores it - entries expire by the global LifeWindow set at
// construction - so configure the lifetime via ${spring.bigcache} instead.
func (b *byteCache) SetBytes(_ context.Context, key string, val []byte, _ int) error {
	return b.c.Set(key, val)
}

// Delete removes key. Deleting an absent key is not an error.
func (b *byteCache) Delete(_ context.Context, key string) error {
	err := b.c.Delete(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil
	}
	return err
}
