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

// Package cache defines a backend-pluggable abstraction for
// key/value caching, so a caching concern can be declared once and served by
// any backend.
//
// A backend implements the single [Cache] interface: typed [Cache.Get]/
// [Cache.Set] (values cross the bytes/any boundary through a pluggable [Codec],
// default [JSONCodec]) plus raw [Cache.GetBytes]/[Cache.SetBytes] for callers
// that already hold bytes. A missing key is reported as [ErrMiss], distinct
// from a backend error, so callers fall through to the source of truth only on
// a real miss.
//
// Backends are selected through a driver registry, mirroring the
// [go-spring.org/spring/discovery] and resilience driver idioms:
//
//   - A starter registers a named [Driver] with [RegisterDriver] (e.g.
//     "go-redis", "redigo", "bigcache", "memcached"). A Driver takes a backend beanID and
//     returns a gs.ModuleFunc that provides a [Cache] bean wrapping that
//     backend instance.
//   - The package's own init module reads ${spring.cache} and, for each
//     entry's "driver" field formatted "<driver>:<beanID>", looks up the Driver
//     and invokes it — so `spring.cache.main.driver=go-redis:main` exposes a
//     Cache bean backed by the "main" redis client.
//   - [RegisterDriver]/[GetDriver] are the registry surface.
package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	gs.Module(gs.OnProperty("spring.cache"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]struct {
			Driver string `value:"${driver:=}"`
		}
		if err := conf.Bind(p, &m, "${spring.cache}"); err != nil {
			return err
		}
		for _, c := range m {
			driverName, beanID, ok := strings.Cut(c.Driver, ":")
			if !ok || driverName == "" || beanID == "" {
				return fmt.Errorf("cache: invalid driver %q (want \"<driver>:<beanID>\", e.g. \"go-redis:main\")", c.Driver)
			}
			d, err := GetDriver(driverName)
			if err != nil {
				return err
			}
			if err := d(beanID)(r, p); err != nil {
				return err
			}
		}
		return nil
	})
}

// Cache is the single interface a caching backend implements. Implementations
// must be safe for concurrent use. A nil Cache is never valid; callers that
// want "no cache" should skip the lookup entirely rather than pass nil.
type Cache interface {
	// Get decodes the value stored under key into val (which must be a
	// pointer), using codec (default [JSONCodec]) to cross the bytes/any
	// boundary. A missing key is reported as [ErrMiss]; any other error is a
	// backend failure. On a miss the caller typically falls through to the
	// source of truth.
	Get(ctx context.Context, key string, val any, codec ...Codec) error

	// GetBytes returns the raw bytes stored under key, bypassing the codec. A
	// missing key is reported as (nil, [ErrMiss]).
	GetBytes(ctx context.Context, key string) ([]byte, error)

	// Set encodes val with codec (default [JSONCodec]) and stores it under key
	// for ttl. A non-positive ttl means the entry does not expire.
	Set(ctx context.Context, key string, val any, ttl time.Duration, codec ...Codec) error

	// SetBytes stores the raw bytes under key for ttl, bypassing the codec. A
	// non-positive ttl means the entry does not expire.
	SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// Delete removes key. Deleting an absent key is not an error.
	Delete(ctx context.Context, key string) error
}

// ErrMiss is returned by Get/GetBytes when the key is absent (a cache miss).
// It is distinct from a backend error (network, serialization, ...): callers
// fall through to the source of truth only on a miss, not on a real failure.
var ErrMiss = errors.New("cache: miss")

// Driver builds the gs.ModuleFunc that provides a [Cache] bean for one backend
// instance. The beanID selects which backend bean (e.g. a *redis.Client) the
// cache wraps; a starter registers a Driver under a backend name via
// [RegisterDriver].
type Driver func(beanID string) gs.ModuleFunc

var (
	driverMu sync.RWMutex
	drivers  = map[string]Driver{}
)

func RegisterDriver(name string, d Driver) {
	if name == "" {
		panic("cache: register driver with empty name")
	}
	if d == nil {
		panic("cache: register nil driver for " + name)
	}
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, ok := drivers[name]; ok {
		panic("cache: driver already registered: " + name)
	}
	drivers[name] = d
}

// GetDriver returns the Driver registered under name, or an error that
// lists the available drivers when none matches.
func GetDriver(name string) (Driver, error) {
	driverMu.RLock()
	defer driverMu.RUnlock()
	if d, ok := drivers[name]; ok {
		return d, nil
	}
	names := make([]string, 0, len(drivers))
	for k := range drivers {
		names = append(names, k)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("cache: no driver registered as %q (registered: %v)", name, names)
}
