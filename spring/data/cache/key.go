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
	"fmt"
	"strings"
)

// keySep separates the parts of a composed cache key. Colon is the de-facto
// convention across Redis/memcached tooling, so keys read naturally in those
// backends' inspection tools.
const keySep = ":"

// Key composes a cache key from a namespace and an arbitrary number of parts,
// joined by ":" (e.g. Key("user", 42, "profile") -> "user:42:profile"). It is
// the default key-generation strategy: deterministic, human-readable, and
// stable across processes so a shared remote cache is hit by every replica.
// Each part is rendered with %v; callers that need collision-free keys for
// large or structured arguments should hash them first (see [Namespace] for a
// bound form).
func Key(namespace string, parts ...any) string {
	if len(parts) == 0 {
		return namespace
	}
	var b strings.Builder
	b.WriteString(namespace)
	for _, p := range parts {
		b.WriteString(keySep)
		fmt.Fprintf(&b, "%v", p)
	}
	return b.String()
}

// Namespace returns a key builder bound to prefix, so a component can compose
// keys under its own namespace without repeating the prefix at every call site:
//
//	userKey := cache.Namespace("user")
//	userKey(42, "profile") // -> "user:42:profile"
func Namespace(prefix string) func(parts ...any) string {
	return func(parts ...any) string {
		return Key(prefix, parts...)
	}
}
