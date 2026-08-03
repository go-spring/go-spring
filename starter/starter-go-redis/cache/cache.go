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

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/data/cache"
)

// NewCache wraps a *redis.Client as a cache.Cache. The "go-redis" driver
// registered in the starter's root package wires it over the client bean
// selected by beanID; use it directly for programmatic construction too.
func NewCache(c *redis.Client) cache.Cache {
	return &redisCache{c}
}

type redisCache struct{ c *redis.Client }

// Get decodes the value under key into val (a pointer) using codec (default
// JSON). A redis.Nil reply (key absent) is reported as [cache.ErrMiss] — a
// plain miss, not a backend error — so callers fall through to the source of
// truth.
func (c *redisCache) Get(ctx context.Context, key string, val any, codec ...cache.Codec) error {
	b, err := c.c.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return cache.ErrMiss
	}
	if err != nil {
		return err
	}
	return cache.ResolveCodec(codec).Unmarshal(b, val)
}

// GetBytes returns the raw bytes under key. A redis.Nil reply (key absent) is
// reported as (nil, [cache.ErrMiss]).
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

// Set encodes val with codec (default JSON) and stores it under key for ttl.
// A non-positive ttl means no expiry (go-redis treats 0 as no TTL).
func (c *redisCache) Set(ctx context.Context, key string, val any, ttl time.Duration, codec ...cache.Codec) error {
	b, err := cache.ResolveCodec(codec).Marshal(val)
	if err != nil {
		return err
	}
	return c.set(ctx, key, b, ttl)
}

// SetBytes stores the raw bytes under key for ttl. A non-positive ttl means no
// expiry (go-redis treats 0 as no TTL).
func (c *redisCache) SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.set(ctx, key, val, ttl)
}

// set stores b under key for ttl; shared by Set and SetBytes.
func (c *redisCache) set(ctx context.Context, key string, b []byte, ttl time.Duration) error {
	if ttl < 0 {
		ttl = 0
	}
	return c.c.Set(ctx, key, b, ttl).Err()
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.c.Del(ctx, key).Err()
}
