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

package luohua

import (
	"context"
	"sync"
	"time"

	"go-spring.org/cloud/cache"
)

// luohuaCache is the company's standard backend: an in-process, TTL-aware,
// bytes-native store. It is deliberately trivial — the point is the seam, not
// the store. Luohua ships it as its uniform driver so a whole fleet can say
// `driver=luohua` in one place and swap the real store later behind the same
// cloud/cache.ByteCache contract.
type luohuaCache struct {
	mu sync.RWMutex
	m  map[string]luohuaValue
}

type luohuaValue struct {
	v   []byte
	exp time.Time // zero = no expiry
}

// NewLuohuaCache builds a typed *cache.Cache over the luohua in-memory store.
func NewLuohuaCache() *cache.Cache {
	return cache.New(&luohuaCache{m: make(map[string]luohuaValue)})
}

// GetBytes returns the stored bytes or cache.ErrMiss when absent/expired.
func (c *luohuaCache) GetBytes(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[key]
	if !ok || (!v.exp.IsZero() && time.Now().After(v.exp)) {
		return nil, cache.ErrMiss
	}
	return v.v, nil
}

// SetBytes stores bytes with an optional TTL (<= 0 means never expire).
func (c *luohuaCache) SetBytes(_ context.Context, key string, val []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.m[key] = luohuaValue{v: val, exp: exp}
	return nil
}

// Delete removes a key; deleting an absent key is not an error.
func (c *luohuaCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
	return nil
}
