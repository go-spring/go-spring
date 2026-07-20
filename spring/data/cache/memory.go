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
	"time"
)

// Memory is a zero-dependency, concurrency-safe in-process [Cache] with
// per-entry expiry. It stores values as-is (no serialization), so cached
// results keep their concrete Go type — making it the ideal near level of a
// [MultiLevel] cache. Use a shared backend (Redis, memcached) for the far level
// so multiple replicas share state. The zero value is ready to use.
type Memory struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

type memoryEntry struct {
	val      any
	expireAt time.Time // zero means no expiry
}

// NewMemory returns a ready-to-use in-process cache. The zero value of [Memory]
// works too; this constructor exists for symmetry with the other backends.
func NewMemory() *Memory { return &Memory{} }

// Get implements [Cache]. An expired entry is treated as absent and lazily
// dropped. It never returns an error.
func (m *Memory) Get(_ context.Context, key string) (any, bool, error) {
	m.mu.RLock()
	e, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !e.expireAt.IsZero() && time.Now().After(e.expireAt) {
		m.mu.Lock()
		// Re-check under the write lock: a concurrent Set may have refreshed it.
		if cur, ok := m.entries[key]; ok && cur.expireAt.Equal(e.expireAt) {
			delete(m.entries, key)
		}
		m.mu.Unlock()
		return nil, false, nil
	}
	return e.val, true, nil
}

// Set implements [Cache]. A non-positive ttl stores the entry without expiry.
func (m *Memory) Set(_ context.Context, key string, val any, ttl time.Duration) error {
	e := memoryEntry{val: val}
	if ttl > 0 {
		e.expireAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	if m.entries == nil {
		m.entries = make(map[string]memoryEntry)
	}
	m.entries[key] = e
	m.mu.Unlock()
	return nil
}

// Delete implements [Cache]. Deleting an absent key is a no-op.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.entries, key)
	m.mu.Unlock()
	return nil
}
