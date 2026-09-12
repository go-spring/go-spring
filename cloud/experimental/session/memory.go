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
	"maps"
	"sync"
	"time"
)

// Memory is a zero-dependency, concurrency-safe in-process [SessionStore] with
// per-entry idle expiry. It is the single-node / test backend. It stores a copy
// of each session's attributes, so mutations after Save do not leak into the
// store. Because it lives in one process it does not share sessions across
// replicas — use a distributed backend (Redis, ...) for that.
//
// An expired entry is dropped the next time [Memory.Load] touches it, which is
// enough for a request-driven store but leaves an entry that expires and is
// never read again in the map forever (a distributed backend gets this for free
// from its key TTL). [Memory.StartCleanup] adds a background sweep for the long
// running case.
type Memory struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
	sweep   *sweeper
}

type memoryEntry struct {
	data     sessionData
	expireAt time.Time // zero means no expiry
}

// sweeper is the background cleanup goroutine's handle. once makes [Memory.Stop]
// idempotent and safe to call concurrently.
type sweeper struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// NewMemory returns a ready-to-use in-process session store. Expired entries are
// reclaimed lazily; call [Memory.StartCleanup] to also sweep them in the
// background.
func NewMemory() *Memory { return &Memory{entries: map[string]memoryEntry{}} }

// StartCleanup starts a background goroutine that drops expired entries every
// interval, bounding the store's memory for sessions that are never read again.
// A non-positive interval is a no-op, as is a second call: at most one sweeper
// runs, and once [Memory.Stop] has ended it cleanup does not resume.
func (m *Memory) StartCleanup(interval time.Duration) {
	if interval <= 0 {
		return
	}
	m.mu.Lock()
	if m.sweep != nil {
		m.mu.Unlock()
		return
	}
	s := &sweeper{stop: make(chan struct{}), done: make(chan struct{})}
	m.sweep = s
	m.mu.Unlock()

	go func() {
		defer close(s.done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				m.dropExpired()
			case <-s.stop:
				return
			}
		}
	}()
}

// Stop ends the sweeper started by [Memory.StartCleanup] and waits for it to
// return. It is a no-op when no sweeper runs, and safe to call more than once.
// The store stays usable afterwards; only background cleanup ends.
func (m *Memory) Stop() {
	m.mu.RLock()
	s := m.sweep
	m.mu.RUnlock()
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
}

// dropExpired removes every entry whose idle deadline has passed. It runs on the
// sweeper goroutine; entries with no deadline (non-positive ttl on Save) are
// kept.
func (m *Memory) dropExpired() {
	now := time.Now()
	m.mu.Lock()
	for id, e := range m.entries {
		if !e.expireAt.IsZero() && now.After(e.expireAt) {
			delete(m.entries, id)
		}
	}
	m.mu.Unlock()
}

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
	maps.Copy(attrs, d.Attributes)
	return sessionData{Attributes: attrs, CreatedAt: d.CreatedAt}
}
