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

package StarterSessionRedis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/session"
)

// Store is a Redis-backed [session.SessionStore]. It embeds the interface value
// produced by [session.FromByteStore] over a [redisByteStore], so it is a named,
// exportable concrete type (a gs bean provider cannot return an unexported
// interface impl) while all session (de)serialization stays in the stdlib.
type Store struct {
	session.SessionStore
}

// newStore builds a Store over an already-constructed *redis.Client. The client's
// lifecycle (Ping, Close, ...) is owned by starter-go-redis; this store never
// closes it, hence no destroy hook.
func newStore(c Config, client *redis.Client) *Store {
	bs := &redisByteStore{client: client, prefix: c.KeyPrefix}
	return &Store{SessionStore: session.FromByteStore(bs)}
}

// redisByteStore implements [session.ByteStore] on a *redis.Client. Sessions are
// stored as opaque JSON bytes (produced by session.FromByteStore) under the
// prefixed id, with the idle timeout applied as the Redis key TTL — so expiry and
// sliding renewal are enforced by Redis itself.
type redisByteStore struct {
	client *redis.Client
	prefix string
}

func (s *redisByteStore) key(id string) string { return s.prefix + id }

func (s *redisByteStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
	b, err := s.client.Get(ctx, s.key(id)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (s *redisByteStore) Set(ctx context.Context, id string, data []byte, ttl time.Duration) error {
	// A non-positive ttl maps to Redis expiration 0, which means "no expiry".
	if ttl < 0 {
		ttl = 0
	}
	return s.client.Set(ctx, s.key(id), data, ttl).Err()
}

func (s *redisByteStore) Delete(ctx context.Context, id string) error {
	return s.client.Del(ctx, s.key(id)).Err()
}

// Ensure interface compliance at compile time.
var _ session.ByteStore = (*redisByteStore)(nil)
