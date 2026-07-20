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
	"go-spring.org/spring/data/cache"
)

// redisByteStore adapts a Redis client to cache.ByteStore, the byte-oriented
// backend seam of stdlib/cache. A redis.Nil reply (key absent) maps to found=
// false rather than an error, so the cache layer treats it as a plain miss.
type redisByteStore struct {
	client redis.UniversalClient
}

func (s redisByteStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (s redisByteStore) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	// A non-positive ttl means "no expiry": go-redis treats 0 as no TTL.
	if ttl < 0 {
		ttl = 0
	}
	return s.client.Set(ctx, key, data, ttl).Err()
}

func (s redisByteStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

// AsCache wraps a Redis client as a shared, remote cache.Cache. Values are
// serialized with codec (nil defaults to cache.JSONCodec), so a cached struct
// round-trips through JSON — see cache.Codec for the type-fidelity caveat. This
// is the far (shared) level of a multi-level cache; pair it with cache.Memory as
// the near level via cache.NewMultiLevel.
func AsCache(client redis.UniversalClient, codec cache.Codec) cache.Cache {
	return cache.FromByteStore(redisByteStore{client: client}, codec)
}
