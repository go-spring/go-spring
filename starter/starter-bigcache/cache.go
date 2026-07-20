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
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/spring/cache"
)

// bigcacheByteStore adapts a BigCache instance to cache.ByteStore, so a purely
// in-process cache satisfies the same seam as Redis/memcached and can serve as
// the near level of a multi-level cache.
//
// BigCache expires entries by a single global LifeWindow set at construction,
// not per entry, so the per-call ttl is intentionally ignored here — configure
// the lifetime via ${spring.bigcache} LifeWindow instead.
type bigcacheByteStore struct {
	cache *bigcache.BigCache
}

func (s bigcacheByteStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	b, err := s.cache.Get(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (s bigcacheByteStore) Set(_ context.Context, key string, data []byte, _ time.Duration) error {
	return s.cache.Set(key, data)
}

func (s bigcacheByteStore) Delete(_ context.Context, key string) error {
	err := s.cache.Delete(key)
	if errors.Is(err, bigcache.ErrEntryNotFound) {
		return nil // deleting an absent key is not an error
	}
	return err
}

// AsCache wraps a BigCache instance as an in-process cache.Cache. Values are
// serialized with codec (nil defaults to cache.JSONCodec) so the returned Cache
// matches the remote backends; when using it purely as a local level you may
// prefer cache.Memory, which stores values without serialization and keeps
// their concrete type.
func AsCache(bc *bigcache.BigCache, codec cache.Codec) cache.Cache {
	return cache.FromByteStore(bigcacheByteStore{cache: bc}, codec)
}
