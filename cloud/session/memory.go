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

package session

import (
	"context"
	"sync"
	"time"
)

// Memory is a zero-dependency, concurrency-safe in-process [SessionStore] with
// per-entry idle expiry. It is the single-node / test backend and is registered
// as "memory" out of the box. It stores a copy of each session's attributes, so
// mutations after Save do not leak into the store. Because it lives in one
// process it does not share sessions across replicas — use a distributed backend
// (Redis, ...) for that.
type Memory struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

type memoryEntry struct {
	data     sessionData
	expireAt time.Time // zero means no expiry
}

// NewMemory returns a ready-to-use in-process session store.
func NewMemory() *Memory { return &Memory{entries: map[string]memoryEntry{}} }

// Load implements [SessionStore]. An expired entry is treated as absent and
// lazily dropped. It never returns an error.
func (m *Memory) Load(_ context.Context, id string) (*Session, bool, error) {
	m.mu.RLock()
	e, ok := m.entries[id]
	m.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !e.expireAt.IsZero() && time.Now().After(e.expireAt) {
		m.mu.Lock()
		if cur, ok := m.entries[id]; ok && cur.expireAt.Equal(e.expireAt) {
			delete(m.entries, id)
		}
		m.mu.Unlock()
		return nil, false, nil
	}
	return fromData(id, copyData(e.data)), true, nil
}

// Save implements [SessionStore]. A non-positive ttl stores the session without
// expiry; a positive ttl refreshes the idle deadline (sliding renewal).
func (m *Memory) Save(_ context.Context, s *Session, ttl time.Duration) error {
	e := memoryEntry{data: s.snapshot()}
	if ttl > 0 {
		e.expireAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	if m.entries == nil {
		m.entries = make(map[string]memoryEntry)
	}
	m.entries[s.ID()] = e
	m.mu.Unlock()
	return nil
}

// Delete implements [SessionStore]. Deleting an absent id is a no-op.
func (m *Memory) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.entries, id)
	m.mu.Unlock()
	return nil
}

// copyData deep-copies the attribute map so a loaded session cannot alias the
// stored entry (a subsequent Set on the returned session must not mutate the
// store in place).
func copyData(d sessionData) sessionData {
	attrs := make(map[string]any, len(d.Attributes))
	for k, v := range d.Attributes {
		attrs[k] = v
	}
	return sessionData{Attributes: attrs, CreatedAt: d.CreatedAt}
}

// init registers the bundled Memory store as "memory" so the framework has a
// working session backend out of the box and in tests.
func init() {
	Register("memory", NewMemory())
}
