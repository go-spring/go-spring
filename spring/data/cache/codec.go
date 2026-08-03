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
	"encoding/json"
)

// Codec serializes cache values to and from bytes for the typed [Cache.Get]/
// [Cache.Set] methods. The wire form is untyped, so with the default
// [JSONCodec] a cached struct comes back as the JSON-decoded shape (e.g.
// map[string]any for objects, float64 for numbers) — keep cached values
// JSON-friendly, or pass a codec that preserves your types.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSONCodec is the default [Codec]: it encodes values with encoding/json. It is
// a usable zero-value; pass an instance to [Cache.Get]/[Cache.Set] only to
// override the default for a value that is not JSON-friendly.
type JSONCodec struct{}

// Marshal implements [Codec].
func (JSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal implements [Codec].
func (JSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// ResolveCodec returns the codec to use for a Get/Set call: the first provided
// codec, or [JSONCodec] when none is given. Backends call it to apply the
// default consistently instead of re-deriving it.
func ResolveCodec(codec []Codec) Codec {
	if len(codec) > 0 && codec[0] != nil {
		return codec[0]
	}
	return JSONCodec{}
}
