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

// Package cache defines a framework-agnostic, zero-dependency abstraction for
// key/value caching, so a caching concern can be declared once and served by
// any backend.
//
// It is the provider-pluggable layer the aspect Cache marker
// ([go-spring.org/spring/aspect].Cache) plugs into: aspect keeps a minimal
// in-process [aspect.Store] seam, and [AsStore] adapts any [Cache] here to it,
// so the same @Cacheable-equivalent interceptor can be backed by an in-process
// map, Redis, memcached, or a multi-level combination without changing the
// business code.
//
// The abstraction mirrors [go-spring.org/spring/discovery] and
// [go-spring.org/spring/resilience]:
//
//   - [Cache] is the single interface a backend implements. Byte-oriented
//     backends (Redis, memcached, bigcache) implement the narrower [ByteStore]
//     and are lifted to a Cache with a [Codec] via [FromByteStore].
//   - Named backends are shared through a package-level registry
//     ([Register]/[Get]/[MustGet]) so a caller can select one by name.
//   - The bundled [Memory] cache has zero third-party dependencies, so the
//     framework caches out of the box and in tests; starters register their
//     backend (Redis, memcached, bigcache) under a name.
//   - [MultiLevel] composes several caches into a near-to-far hierarchy
//     (local + remote) with read-through backfill.
package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Cache is the single interface a caching backend implements. Implementations
// must be safe for concurrent use. A nil Cache is never valid; callers that
// want "no cache" should skip the lookup entirely rather than pass nil.
type Cache interface {
	// Get returns the value stored under key and whether it was present (and
	// not expired). A backend error (network, serialization, ...) is returned
	// with found=false; callers typically treat an error as a miss and proceed
	// to the source of truth.
	Get(ctx context.Context, key string) (val any, found bool, err error)

	// Set stores val under key for the given ttl. A non-positive ttl means the
	// entry does not expire.
	Set(ctx context.Context, key string, val any, ttl time.Duration) error

	// Delete removes key. Deleting an absent key is not an error.
	Delete(ctx context.Context, key string) error
}

var (
	mu       sync.RWMutex
	registry = map[string]Cache{}
)

// Register makes a Cache backend available under name. It panics if name is
// empty, c is nil, or name is already registered, mirroring the driver-registry
// idiom used elsewhere (discovery.Register, resilience.RegisterDriver) so
// duplicate wiring fails loudly at init.
func Register(name string, c Cache) {
	if name == "" {
		panic("cache: register with empty name")
	}
	if c == nil {
		panic("cache: register nil backend for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("cache: backend already registered: " + name)
	}
	registry[name] = c
}

// Get returns the Cache backend registered under name.
func Get(name string) (Cache, bool) {
	mu.RLock()
	defer mu.RUnlock()
	c, ok := registry[name]
	return c, ok
}

// MustGet returns the Cache backend registered under name, or an error that
// lists the available backends when none matches.
func MustGet(name string) (Cache, error) {
	if c, ok := Get(name); ok {
		return c, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("cache: no backend registered as %q (registered: %v)", name, names)
}
