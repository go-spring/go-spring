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
	"encoding/json"
	"time"
)

// ByteStore is the narrow byte-oriented backend that remote caches expose
// natively (Redis GET/SET/DEL, memcached, bigcache all store []byte). A starter
// implements this single interface over its client and lifts it to a full
// [Cache] with [FromByteStore], so every remote backend shares the same
// serialization path instead of re-deriving one. Implementations must be safe
// for concurrent use.
type ByteStore interface {
	Get(ctx context.Context, key string) (data []byte, found bool, err error)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Codec serializes cache values to and from bytes for [ByteStore]-backed
// caches. Because the wire form is untyped, values round-trip through the
// codec's generic form: with the default [JSONCodec] a cached struct comes back
// as the JSON-decoded shape (e.g. map[string]any for objects). This matters for
// the far level of a [MultiLevel] cache — keep remotely cached values
// JSON-friendly, or register a codec that preserves your types. The near
// [Memory] level is unaffected: it stores the live value with its concrete type.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte) (any, error)
}

// JSONCodec is the default [Codec]: it encodes values with encoding/json.
// Decoding yields JSON's generic Go shapes (float64 for numbers, map[string]any
// for objects), so it is lossy for concrete struct types by design — see
// [Codec].
type JSONCodec struct{}

// Marshal implements [Codec].
func (JSONCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }

// Unmarshal implements [Codec].
func (JSONCodec) Unmarshal(data []byte) (any, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// byteCache adapts a [ByteStore] and a [Codec] into a [Cache].
type byteCache struct {
	store ByteStore
	codec Codec
}

// FromByteStore lifts a byte-oriented backend to a full [Cache] using codec to
// serialize values. A nil codec defaults to [JSONCodec]. This is the single
// entry point Redis/memcached/bigcache starters use to expose a cache.Cache.
func FromByteStore(store ByteStore, codec Codec) Cache {
	if codec == nil {
		codec = JSONCodec{}
	}
	return &byteCache{store: store, codec: codec}
}

func (c *byteCache) Get(ctx context.Context, key string) (any, bool, error) {
	data, ok, err := c.store.Get(ctx, key)
	if err != nil || !ok {
		return nil, false, err
	}
	v, err := c.codec.Unmarshal(data)
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

func (c *byteCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := c.codec.Marshal(val)
	if err != nil {
		return err
	}
	return c.store.Set(ctx, key, data, ttl)
}

func (c *byteCache) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, key)
}
