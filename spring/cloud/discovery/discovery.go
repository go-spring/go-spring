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

// Package discovery defines a framework-agnostic, zero-dependency abstraction
// for client-side service discovery.
//
// It answers one question for infrastructure clients (Redis, MySQL, MongoDB,
// Kafka, ...): "given a logical service name, which live host:port addresses
// can I connect to right now?". It deliberately says nothing about the
// provider-side registration of RPC frameworks — that stays bound to each
// framework (dubbo-go, kitex, ...).
//
// A company adapts its own naming service by implementing the single
// [Discovery] interface and registering it once via [Register]; every client
// starter then resolves names through the registered backend without any
// per-component adaptation.
package discovery

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Endpoint is a single connectable instance returned by a [Discovery] backend.
type Endpoint struct {
	// Addr is the connectable "host:port".
	Addr string

	// Weight is the load-balancing weight; 0 is treated as the default weight.
	Weight int

	// Healthy reports whether the discovery source considers this instance
	// healthy. Backends that do not track health should leave it false; callers
	// then treat all returned endpoints as eligible (see LiveDialer.Pick).
	Healthy bool

	// Metadata carries backend-specific attributes (zone, unit, version, ...),
	// passed through untouched for the caller to route on.
	Metadata map[string]string
}

// Discovery is the single interface a company adapts to its naming service.
//
// Implementations must be safe for concurrent use.
type Discovery interface {
	// Resolve returns the current snapshot of endpoints for name. It is called
	// once at cold start, before a client establishes its first connection.
	Resolve(ctx context.Context, name string) ([]Endpoint, error)

	// Watch subscribes to changes for name. Each time the instance set changes
	// the returned Watcher yields a fresh full snapshot. The caller is
	// responsible for calling Watcher.Stop when it no longer needs updates.
	Watch(ctx context.Context, name string) (Watcher, error)
}

// Watcher streams endpoint snapshots for a watched service name.
type Watcher interface {
	// Next blocks until the next snapshot is available and returns it. It
	// returns a non-nil error once the watcher has been stopped or the
	// underlying subscription fails; callers should then stop looping.
	Next() ([]Endpoint, error)

	// Stop releases the subscription. It is safe to call more than once.
	Stop() error
}

var (
	mu       sync.RWMutex
	registry = map[string]Discovery{}
)

// Register makes a Discovery backend available under name. It panics if name is
// already registered, mirroring the driver-registry idiom used elsewhere (e.g.
// starter-go-redis RegisterDriver), so duplicate wiring fails loudly at init.
func Register(name string, d Discovery) {
	if name == "" {
		panic("discovery: register with empty name")
	}
	if d == nil {
		panic("discovery: register nil backend for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		panic("discovery: backend already registered: " + name)
	}
	registry[name] = d
}

// Get returns the Discovery backend registered under name.
func Get(name string) (Discovery, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	return d, ok
}

// MustGet returns the Discovery backend registered under name, or an error that
// lists the available backends when none matches.
func MustGet(name string) (Discovery, error) {
	if d, ok := Get(name); ok {
		return d, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("discovery: no backend registered as %q (registered: %v)", name, names)
}
