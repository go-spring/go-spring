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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/experimental/session"
)

// newTestStore starts an in-process Redis and returns a ByteStore over its
// client, plus the miniredis server for TTL manipulation.
func newTestStore(t *testing.T, prefix string) (*redisByteStore, *miniredis.Miniredis) {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &redisByteStore{client: client, prefix: prefix}, srv
}

func TestByteStoreGetSetDelete(t *testing.T) {
	bs, _ := newTestStore(t, "session:")
	ctx := context.Background()

	// A missing id is found=false and no error.
	if _, found, err := bs.Get(ctx, "nope"); err != nil || found {
		t.Fatalf("get missing: found=%v err=%v", found, err)
	}

	if err := bs.Set(ctx, "s1", []byte(`{"attributes":{"user":"alice"}}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	data, found, err := bs.Get(ctx, "s1")
	if err != nil || !found {
		t.Fatalf("get s1: found=%v err=%v", found, err)
	}
	if string(data) != `{"attributes":{"user":"alice"}}` {
		t.Fatalf("roundtrip mismatch: %s", data)
	}
	if err := bs.Delete(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ = bs.Get(ctx, "s1"); found {
		t.Fatal("delete did not remove the key")
	}
	// Deleting an absent id is not an error.
	if err := bs.Delete(ctx, "s1"); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
}

func TestByteStoreKeyPrefix(t *testing.T) {
	bs, srv := newTestStore(t, "appA:")
	ctx := context.Background()

	if err := bs.Set(ctx, "s1", []byte("v"), 0); err != nil {
		t.Fatal(err)
	}
	if !srv.Exists("appA:s1") {
		t.Fatal("expected key appA:s1 in redis")
	}
	if srv.Exists("s1") {
		t.Fatal("unprefixed key should not exist")
	}
}

func TestByteStoreTTL(t *testing.T) {
	bs, srv := newTestStore(t, "session:")
	ctx := context.Background()

	// A positive ttl becomes the Redis key TTL: expiry is enforced by Redis.
	if err := bs.Set(ctx, "expiring", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := srv.TTL("session:expiring"); got < 30*time.Second || got > time.Minute {
		t.Fatalf("ttl = %v, want ~1m", got)
	}
	srv.FastForward(2 * time.Minute)
	if srv.Exists("session:expiring") {
		t.Fatal("key should have expired after fast-forward")
	}

	// A non-positive ttl means no expiry.
	if err := bs.Set(ctx, "forever", []byte("v"), 0); err != nil {
		t.Fatal(err)
	}
	if err := bs.Set(ctx, "negative", []byte("v"), -time.Second); err != nil {
		t.Fatal(err)
	}
	srv.FastForward(time.Hour)
	if !srv.Exists("session:forever") || !srv.Exists("session:negative") {
		t.Fatal("non-positive ttl should map to no expiry")
	}
}

func TestStoreLiftsToSessionStore(t *testing.T) {
	// The exported Store lifts the ByteStore via session.FromByteStore, so the
	// full SessionStore contract (Load/Save/Delete with JSON encoding) is
	// inherited from the shared serialization path.
	bs, _ := newTestStore(t, "session:")
	store := &Store{SessionStore: session.FromByteStore(bs)}
	var _ session.SessionStore = store
}
