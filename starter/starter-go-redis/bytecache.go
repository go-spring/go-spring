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

package StarterGoRedis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/cache"
)

type byteCache struct{ c *Client }

// NewByteCache wraps a [Client] as a [cache.ByteCache] - the raw bytes-native
// primitives the "go-redis" driver layers a typed [cache.Cache] façade over.
// Every operation flows through the wrapper's command seam, where the starter
// declares the command's identity and the resilience layer emits the span,
// duration metrics and access log. The driver registered in the
// starter's root package selects the client bean by beanID; call this directly
// to build a ByteCache for ad-hoc use.
func NewByteCache(c *Client) cache.ByteCache {
	return &byteCache{c}
}

// GetBytes returns the raw bytes under key, or (nil, [cache.ErrMiss]) when the
// key is absent.
func (c *byteCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	b, err := c.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// SetBytes stores the raw bytes under key for ttlSeconds, a whole number of
// seconds. A non-positive value means the entry does not expire.
func (c *byteCache) SetBytes(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}
	return c.c.Set(ctx, key, val, time.Duration(ttlSeconds)*time.Second).Err()
}

// Delete removes key. Deleting an absent key is not an error.
func (c *byteCache) Delete(ctx context.Context, key string) error {
	return c.c.Del(ctx, key).Err()
}
