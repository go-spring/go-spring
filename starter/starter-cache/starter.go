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

// Package StarterCache is the gs wiring between ${spring.cache} properties and
// the container-free cache abstraction (cloud/cache): it owns the
// backend driver registry and the conditional module that turns each
// spring.cache.<name> entry into a Cache bean. Backend starters (bigcache,
// memcached, redigo, go-redis) register their drivers here, so importing any
// of them is what makes this module's init fire — no separate blank import.
//
//	spring.cache.main.driver=go-redis:main
//
// exposes a Cache bean named "main" backed by the "main" redis client,
// selected by the "<driver>:<beanID>" format.
package StarterCache

import (
	"fmt"
	"sort"
	"strings"
	"sync"

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

// Driver builds the gs.ModuleFunc that provides a [cache.Cache] bean for one
// backend instance. The beanID selects which backend bean (e.g. a
// *redis.Client) the cache wraps; a starter registers a Driver under a backend
// name via [RegisterDriver]. The returned module constructs a
// [cache.ByteCache] adapter around the backend client and embeds it in a
// [cache.Cache] struct to expose the typed Get/Set surface.
type Driver func(beanID string) gs.ModuleFunc

var (
	driverMu sync.RWMutex
	drivers  = map[string]Driver{}
)

// RegisterDriver installs d as the Driver for name (e.g. "go-redis",
// "redigo", "bigcache", "memcached"). It panics on an empty name, a nil
// driver, or a duplicate — the driver-registry idiom, so a mis-wired backend
// starter fails loudly at init rather than silently.
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

// GetDriver returns the Driver registered under name, or an error that lists
// the available drivers when none matches — so a typo or a missing backend
// starter import is obvious at startup.
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
