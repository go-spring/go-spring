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

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/cache"
)

// NewByteCache wraps a redis.UniversalClient as a [cache.ByteCache] - the raw
// bytes-native primitives the "go-redis" driver layers a typed [cache.Cache]
// façade over. The driver registered in the starter's root package selects the
// client bean by beanID; call this directly to build a ByteCache for ad-hoc
// use.
func NewByteCache(c redis.UniversalClient) cache.ByteCache {
	return &redisCache{c}
}

type redisCache struct{ c redis.UniversalClient }

// GetBytes returns the raw bytes under key. A redis.Nil reply (key absent) is
// reported as (nil, [cache.ErrMiss]) - a plain miss, not a backend error - so
// callers fall through to the source of truth.
func (c *redisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	b, err := c.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// SetBytes stores the raw bytes under key for ttl. A non-positive ttl means no
// expiry (go-redis treats 0 as no TTL).
func (c *redisCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if ttl < 0 {
		ttl = 0
	}
	return c.c.Set(ctx, key, val, ttl).Err()
}

// Delete removes key. Deleting an absent key is not an error.
func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.c.Del(ctx, key).Err()
}
