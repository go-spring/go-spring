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

package validation

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Driver{}
)

// RegisterDriver makes a [Driver] available under name. It panics if name is
// empty, d is nil, or name is already registered, mirroring the driver-registry
// idiom used elsewhere (resilience.RegisterDriver, discovery.Register) so
// duplicate wiring fails loudly at init.
func RegisterDriver(name string, d Driver) {
	if name == "" {
		panic("validation: register with empty name")
	}
	if d == nil {
		panic("validation: register nil driver for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("validation: driver already registered: " + name)
	}
	registry[name] = d
}

// GetDriver returns the [Driver] registered under name.
func GetDriver(name string) (Driver, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	return d, ok
}

// MustGetDriver returns the [Driver] registered under name, or an error that
// lists the available drivers when none matches.
func MustGetDriver(name string) (Driver, error) {
	if d, ok := GetDriver(name); ok {
		return d, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("validation: no driver registered as %q (registered: %v)", name, names)
}
