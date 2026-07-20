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

package StarterMemcached

import (
	"context"
	"errors"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/spring/data/cache"
)

// memcachedByteStore adapts a memcached client to cache.ByteStore. A cache miss
// (memcache.ErrCacheMiss) maps to found=false rather than an error.
type memcachedByteStore struct {
	client *memcache.Client
}

func (s memcachedByteStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	item, err := s.client.Get(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return item.Value, true, nil
}

func (s memcachedByteStore) Set(_ context.Context, key string, data []byte, ttl time.Duration) error {
	// memcached expiration is in seconds; 0 means "never expire". Round a
	// sub-second positive ttl up to 1s so it is not silently treated as forever.
	var exp int32
	if ttl > 0 {
		if exp = int32(ttl.Seconds()); exp == 0 {
			exp = 1
		}
	}
	return s.client.Set(&memcache.Item{Key: key, Value: data, Expiration: exp})
}

func (s memcachedByteStore) Delete(_ context.Context, key string) error {
	err := s.client.Delete(key)
	if errors.Is(err, memcache.ErrCacheMiss) {
		return nil // deleting an absent key is not an error
	}
	return err
}

// AsCache wraps a memcached client as a shared, remote cache.Cache. Values are
// serialized with codec (nil defaults to cache.JSONCodec) — see cache.Codec for
// the type-fidelity caveat. Use it as the far level of cache.NewMultiLevel.
func AsCache(client *memcache.Client, codec cache.Codec) cache.Cache {
	return cache.FromByteStore(memcachedByteStore{client: client}, codec)
}
