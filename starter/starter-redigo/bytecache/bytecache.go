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

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/cache"
)

type redigoCache struct{ pool *redis.Pool }

// NewByteCache wraps a *redis.Pool as a [cache.ByteCache] — the byte-level
// primitives over which the "redigo" cache driver (registered in the starter's
// root package) layers a typed [cache.Cache] façade. The driver picks the pool
// bean by beanID; call this directly only for ad-hoc use.
//
// It takes the native *redis.Pool on purpose, not the starter's wrapper type:
// (a) bytecache is a child of the starter package, so referencing the wrapper
// would form an import cycle; and (b) when the pool comes from a registered
// starter instance, Init has wrapped its Dial so every command below flows
// through obsConn — each GET/SET/DEL is traced, metered, access-logged and
// resilience-guarded with no extra wiring here. Each method honors its ctx via
// redis.DoContext, so a caller deadline can interrupt the op and (in the
// starter path) the span links to the caller's trace. A pool passed in directly
// (ad-hoc, not starter-registered) is not Dial-wrapped, so those ops run
// uninstrumented (but ctx deadlines still apply).
func NewByteCache(pool *redis.Pool) cache.ByteCache {
	return &redigoCache{pool}
}

// GetBytes returns the raw bytes under key. A redis.ErrNil reply (key absent)
// is reported as (nil, [cache.ErrMiss]) - a plain miss, not a backend error.
func (c *redigoCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	conn := c.pool.Get()
	defer conn.Close()
	b, err := redis.Bytes(redis.DoContext(conn, ctx, "GET", key))
	if errors.Is(err, redis.ErrNil) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// SetBytes stores the raw bytes under key for ttl. A non-positive ttl means no
// expiry. ttl is applied in whole seconds; a positive sub-second ttl is
// rounded up to 1s.
func (c *redigoCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	conn := c.pool.Get()
	defer conn.Close()
	if ttl > 0 {
		sec := max(int(ttl.Seconds()), 1)
		_, err := redis.DoContext(conn, ctx, "SET", key, val, "EX", sec)
		return err
	}
	_, err := redis.DoContext(conn, ctx, "SET", key, val)
	return err
}

// Delete removes key. Deleting an absent key is not an error.
func (c *redigoCache) Delete(ctx context.Context, key string) error {
	conn := c.pool.Get()
	defer conn.Close()
	_, err := redis.DoContext(conn, ctx, "DEL", key)
	return err
}
