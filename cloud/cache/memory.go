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
	"context"
	"sync"
)

// Memory is a zero-dependency, concurrency-safe in-process [ByteCache]. It is
// the single-node and test backend: anything layered on this package — a codec,
// a decorator, an application read-through — can be exercised against it with
// no container and no network.
//
// Entries never expire: [Memory.SetBytes] ignores the ttl, so this store keeps
// no clock and needs no reclamation.
type Memory struct {
	mu sync.RWMutex
	m  map[string][]byte
}

// NewMemory returns an empty in-process ByteCache. Wrap it in [New] for the
// typed façade: cache.New(cache.NewMemory()).
func NewMemory() *Memory { return &Memory{m: make(map[string][]byte)} }

// GetBytes returns the raw bytes under key, or (nil, [ErrMiss]) when the key is
// absent.
func (m *Memory) GetBytes(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.m[key]
	if !ok {
		return nil, ErrMiss
	}
	return b, nil
}

// SetBytes stores the raw bytes under key for ttlSeconds, a whole number of
// seconds. Memory ignores it - entries never expire - so use a backend that
// honors the ttl when expiry is what you are testing.
func (m *Memory) SetBytes(_ context.Context, key string, val []byte, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = val
	return nil
}

// Delete removes key. Deleting an absent key is not an error.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
	return nil
}
