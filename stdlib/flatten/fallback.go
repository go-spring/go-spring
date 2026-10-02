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

package flatten

import "strings"

// FallbackStorage wraps another Storage so that an instance inherits any key it
// does not define from the family-wide default bucket. The instance name is
// dropped on the way: spring.X.instances.<name>.<k> reads spring.X.default.<k>.
//
// A single value is overridden one at a time: an instance's leaf replaces that
// leaf and nothing else, so the default still supplies the values around it —
// the other leaves of the same struct, the other entries of the same map. An
// array is overridden whole: as soon as an instance defines one element, the
// array is the instance's.
type FallbackStorage struct {
	Storage
	Instances string // the instances bucket, e.g. "spring.X.instances"
	Defaults  string // the family-wide default bucket, e.g. "spring.X.default"
}

// WithFallback wraps s so that a key under instances falls back to the same key
// under defaults. A key under neither bucket is passed through unchanged.
func WithFallback(s Storage, instances, defaults string) *FallbackStorage {
	return &FallbackStorage{Storage: s, Instances: instances, Defaults: defaults}
}

// fallbackKey returns the family-wide key that holds key's value, and whether
// key has one at all — every key of an instance entry inherits from the default
// bucket, one value at a time.
func (s *FallbackStorage) fallbackKey(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, s.Instances+".")
	if !ok {
		return "", false // not an instance key
	}
	name, below, ok := strings.Cut(rest, ".")
	if !ok {
		return "", false // the instance node itself, not a key inside it
	}
	// An array is inherited whole, so one of its elements (hosts[1]) falls back
	// only when the instance defines no element of that array.
	if i := strings.IndexByte(below, '['); i > 0 {
		if s.Storage.Exists(s.Instances + "." + name + "." + below[:i]) {
			return "", false
		}
	}
	return s.Defaults + "." + below, true
}

// Exists reports whether key exists, in the instance or in the default bucket
// it inherits from.
func (s *FallbackStorage) Exists(key string) bool {
	if s.Storage.Exists(key) {
		return true
	}
	fallback, ok := s.fallbackKey(key)
	if !ok {
		return false
	}
	return s.Storage.Exists(fallback)
}

// Value returns key's leaf value, from the instance or from the default bucket
// it inherits from.
func (s *FallbackStorage) Value(key string) (string, bool) {
	if v, ok := s.Storage.Value(key); ok {
		return v, true
	}
	fallback, ok := s.fallbackKey(key)
	if !ok {
		return "", false
	}
	return s.Storage.Value(fallback)
}

// MapKeys collects key's child keys — the instance's and, for the entries it
// does not define, the default bucket's; an entry is inherited one at a time.
func (s *FallbackStorage) MapKeys(key string, result map[string]struct{}) bool {
	found := s.Storage.MapKeys(key, result)
	fallback, ok := s.fallbackKey(key)
	if !ok {
		return found
	}
	return s.Storage.MapKeys(fallback, result) || found
}

// SliceEntries collects key's slice entries, from the instance's subtree or
// from the default bucket's — never both. Entries read from the default bucket
// are re-keyed under key, since the caller looks them up by the key it asked for.
func (s *FallbackStorage) SliceEntries(key string, result map[string]string) bool {
	if s.Storage.Exists(key) {
		return s.Storage.SliceEntries(key, result)
	}
	fallback, ok := s.fallbackKey(key)
	if !ok {
		return false
	}
	entries := map[string]string{}
	if !s.Storage.SliceEntries(fallback, entries) {
		return false
	}
	for k, v := range entries {
		result[key+k[len(fallback):]] = v
	}
	return true
}
