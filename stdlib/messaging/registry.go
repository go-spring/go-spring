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

package messaging

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Binder{}
)

// RegisterBinder makes a [Binder] available under name. It panics if name is
// empty, b is nil, or name is already registered, mirroring the driver-registry
// idiom used elsewhere (discovery.Register, resilience.RegisterDriver) so
// duplicate wiring fails loudly at init.
//
// Binders are usually connection-bound and wired as beans via a starter's
// NewBinder constructor; this registry is the parity seam for applications that
// select a single process-wide binder by configured name.
func RegisterBinder(name string, b Binder) {
	if name == "" {
		panic("messaging: register with empty name")
	}
	if b == nil {
		panic("messaging: register nil binder for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("messaging: binder already registered: " + name)
	}
	registry[name] = b
}

// GetBinder returns the [Binder] registered under name.
func GetBinder(name string) (Binder, bool) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := registry[name]
	return b, ok
}

// MustGetBinder returns the [Binder] registered under name, or an error that
// lists the available binders when none matches.
func MustGetBinder(name string) (Binder, error) {
	if b, ok := GetBinder(name); ok {
		return b, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("messaging: no binder registered as %q (registered: %v)", name, names)
}
