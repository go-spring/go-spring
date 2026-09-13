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

// Package registry is a generic, named-driver registry shared by the trace and
// metric exporter registries so the two pillars stay structurally identical and
// cannot drift. It mirrors the panic-on-duplicate idiom used elsewhere in the
// framework (starter-gateway's RegisterFilter, cloud/cache RegisterDriver): a mis-wired or duplicate registration fails
// loudly at init.
package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry is a concurrency-safe, name-keyed map of factory values of type T.
// Each pillar (trace, metric) constructs one with a category string used in
// panic and error messages so the two registries report consistently without
// duplicating the bookkeeping.
type Registry[T any] struct {
	category string

	mu   sync.RWMutex
	reg  map[string]T
	once sync.Once
}

// New returns a Registry whose panic/error messages name category (e.g.
// "trace", "metric").
func New[T any](category string) *Registry[T] {
	return &Registry[T]{category: category}
}

// Register makes f available under name. It panics on empty name, a nil f (when
// T is a func/interface type whose zero value is nil — checked via reflection
// is avoided; callers pass a non-nil factory), or a duplicate name, so a
// mis-wired or duplicate registration fails loudly at init.
func (r *Registry[T]) Register(name string, f T) {
	if name == "" {
		panic(r.category + ": register exporter with empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lazyInit()
	if _, ok := r.reg[name]; ok {
		panic(r.category + ": exporter already registered: " + name)
	}
	r.reg[name] = f
}

func (r *Registry[T]) lazyInit() {
	r.once.Do(func() { r.reg = make(map[string]T) })
}

// Lookup returns the factory registered under name.
func (r *Registry[T]) Lookup(name string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.reg[name]
	return f, ok
}

// Delete removes the factory registered under name. It is intended for test
// cleanup of scratch factories; built-in exporters are registered at init and
// never deleted.
func (r *Registry[T]) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.reg, name)
}

// Names returns the sorted registered names, for inclusion in "unknown
// exporter" error messages.
func (r *Registry[T]) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.reg))
	for n := range r.reg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// UnknownErr builds the error returned when name matches no registered factory,
// listing the available ones so the misconfig is self-diagnosing.
func (r *Registry[T]) UnknownErr(name string) error {
	return fmt.Errorf("observability: unknown %s exporter %q (registered: %s)",
		r.category, name, strings.Join(r.Names(), ", "))
}
