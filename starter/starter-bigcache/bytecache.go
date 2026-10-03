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
// Every operation flows through the wrapper's command surface, which emits the
// operation's span and metrics there (see [statObserver]), so the façade inherits
// them. The starter's own registration calls this to expose each instance as a
// [cache.Cache] bean; call it directly to build a ByteCache for ad-hoc use.
func NewByteCache(c *Cache) cache.ByteCache {
	return &byteCache{c}
}

// GetBytes returns the raw bytes under key, or (nil, [cache.ErrMiss]) when the
// key is absent.
//
// The context is passed on rather than dropped: it is the caller's, and the
// wrapper it reaches opens the operation's span with it.
func (b *byteCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	data, err := b.c.Get(ctx, key)
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
func (b *byteCache) SetBytes(ctx context.Context, key string, val []byte, _ int) error {
	return b.c.Set(ctx, key, val)
}

// Delete removes key. Deleting an absent key is not an error.
func (b *byteCache) Delete(ctx context.Context, key string) error {
	err := b.c.Delete(ctx, key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil
	}
	return err
}
