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

package security

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]TokenValidator{}
)

// RegisterValidator makes a [TokenValidator] available under name. It panics if
// name is empty, v is nil, or name is already registered, mirroring the
// driver-registry idiom used elsewhere (discovery.Register, resilience.
// RegisterDriver) so duplicate wiring fails loudly at init.
//
// A resource-server starter registers its validator once at construction so that
// method-level guards ([Require]) can resolve it by name without importing any
// concrete verification library.
func RegisterValidator(name string, v TokenValidator) {
	if name == "" {
		panic("security: register with empty name")
	}
	if v == nil {
		panic("security: register nil validator for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("security: validator already registered: " + name)
	}
	registry[name] = v
}

// GetValidator returns the [TokenValidator] registered under name.
func GetValidator(name string) (TokenValidator, bool) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := registry[name]
	return v, ok
}

// MustGetValidator returns the [TokenValidator] registered under name, or an
// error that lists the available validators when none matches.
func MustGetValidator(name string) (TokenValidator, error) {
	if v, ok := GetValidator(name); ok {
		return v, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("security: no validator registered as %q (registered: %v)", name, names)
}
