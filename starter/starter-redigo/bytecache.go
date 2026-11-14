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

package StarterRedigo

import (
	"context"
	"errors"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/cache"
)

type byteCache struct{ pool *Pool }

// NewByteCache wraps a governed [Pool] as a [cache.ByteCache] — the byte-level
// primitives over which the "redigo" cache driver (registered in the starter's
// root package) layers a typed [cache.Cache] façade. The driver picks the pool
// bean by beanID; call this directly only for ad-hoc use.
//
// Each GET/SET/DEL issues one command on the pool's virtual conn, so it flows
// the full chain — traced, metered, access-logged, resilience-guarded and
// addressed per round — with no extra wiring here. Each method honors its ctx
// (the span links to the caller's trace and a deadline can interrupt the op).
func NewByteCache(pool *Pool) cache.ByteCache {
	return &byteCache{pool}
}

// GetBytes returns the raw bytes under key, or (nil, [cache.ErrMiss]) when the
// key is absent.
func (c *byteCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	b, err := redis.Bytes(c.pool.Get().DoContext(ctx, "GET", key))
	if errors.Is(err, redis.ErrNil) {
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
	if ttlSeconds > 0 {
		_, err := c.pool.Get().DoContext(ctx, "SET", key, val, "EX", ttlSeconds)
		return err
	}
	_, err := c.pool.Get().DoContext(ctx, "SET", key, val)
	return err
}

// Delete removes key. Deleting an absent key is not an error.
func (c *byteCache) Delete(ctx context.Context, key string) error {
	_, err := c.pool.Get().DoContext(ctx, "DEL", key)
	return err
}
