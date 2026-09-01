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

package session

import "encoding/json"

// Get returns the attribute stored under key decoded into T. Methods cannot
// carry type parameters in Go, so typed access is a package-level function
// rather than a Session method.
//
// If the stored value is already a T it is returned as-is (the in-process
// [Memory] store preserves original types); otherwise the value is re-encoded
// through JSON before decoding, so a session loaded from a byte-oriented
// backend (whose attributes come back as map[string]any / float64) still
// yields the intended type. The boolean reports whether the key was present;
// a decode failure of a present value is returned as an error.
func Get[T any](s *Session, key string) (val T, ok bool, err error) {
	s.mu.RLock()
	v, present := s.attrs[key]
	s.mu.RUnlock()
	if !present {
		return val, false, nil
	}
	if t, is := v.(T); is {
		return t, true, nil
	}
	data, e := json.Marshal(v)
	if e != nil {
		return val, true, e
	}
	if e := json.Unmarshal(data, &val); e != nil {
		return val, true, e
	}
	return val, true, nil
}

// Set stores val under key, marking the session dirty so it is persisted on
// write-back. It is the typed counterpart of [Get]; see there for the decoding
// contract. The type parameter keeps the stored form self-describing for the
// JSON round-trip path.
func Set[T any](s *Session, key string, val T) {
	s.mu.Lock()
	s.attrs[key] = val
	s.modified = true
	s.mu.Unlock()
}
