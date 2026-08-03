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

	"github.com/gomodule/redigo/redis"
	"go-spring.org/spring/data/cache"
)

// NewCache wraps a *redis.Pool as a cache.Cache. The "redigo" driver
// registered in the starter's root package wires it over the pool bean selected
// by beanID; use it directly for programmatic construction too.
func NewCache(pool *redis.Pool) cache.Cache {
	return &redigoCache{pool}
}

type redigoCache struct{ pool *redis.Pool }

// Get decodes the value under key into val (a pointer) using codec (default
// JSON). A redis.ErrNil reply (key absent) is reported as [cache.ErrMiss] — a
// plain miss, not a backend error.
func (c *redigoCache) Get(ctx context.Context, key string, val any, codec ...cache.Codec) error {
	conn := c.pool.Get()
	defer conn.Close()
	b, err := redis.Bytes(conn.Do("GET", key))
	if errors.Is(err, redis.ErrNil) {
		return cache.ErrMiss
	}
	if err != nil {
		return err
	}
	return cache.ResolveCodec(codec).Unmarshal(b, val)
}

// GetBytes returns the raw bytes under key. A redis.ErrNil reply (key absent)
// is reported as (nil, [cache.ErrMiss]).
func (c *redigoCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	conn := c.pool.Get()
	defer conn.Close()
	b, err := redis.Bytes(conn.Do("GET", key))
	if errors.Is(err, redis.ErrNil) {
		return nil, cache.ErrMiss
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Set encodes val with codec (default JSON) and stores it under key for ttl.
// A non-positive ttl means no expiry. ttl is applied in whole seconds; a
// positive sub-second ttl is rounded up to 1s.
func (c *redigoCache) Set(ctx context.Context, key string, val any, ttl time.Duration, codec ...cache.Codec) error {
	b, err := cache.ResolveCodec(codec).Marshal(val)
	if err != nil {
		return err
	}
	return c.set(key, b, ttl)
}

// SetBytes stores the raw bytes under key for ttl (see Set for ttl semantics).
func (c *redigoCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.set(key, val, ttl)
}

// set borrows a connection and stores b under key for ttl; shared by Set and
// SetBytes. ttl<=0 means no expiry; otherwise it is rounded up to whole seconds
// (min 1s) and applied via SET ... EX.
func (c *redigoCache) set(key string, b []byte, ttl time.Duration) error {
	conn := c.pool.Get()
	defer conn.Close()
	if ttl > 0 {
		sec := int(ttl.Seconds())
		if sec < 1 {
			sec = 1
		}
		_, err := conn.Do("SET", key, b, "EX", sec)
		return err
	}
	_, err := conn.Do("SET", key, b)
	return err
}

// Delete removes key. Deleting an absent key is not an error.
func (c *redigoCache) Delete(ctx context.Context, key string) error {
	conn := c.pool.Get()
	defer conn.Close()
	_, err := conn.Do("DEL", key)
	return err
}
