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

package outbox

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemoryStore is an in-memory [Store] for tests and demos. It is NOT durable:
// records vanish with the process, so it provides no real transactional
// guarantee — the pattern's point is surviving crashes, which this store
// cannot. Use it to exercise Relay wiring without a database.
type MemoryStore struct {
	mu     sync.Mutex
	nextID int64
	rows   []*memRow
}

// memRow is one stored record plus its scheduling state.
type memRow struct {
	rec        Record
	status     string
	nextRetry  time.Time
	sentAt     time.Time
	leased     bool // fetched and not yet resolved → hidden from other Fetches
	leasedTill time.Time
}

// MemoryStore implements [Store].
var _ Store = (*MemoryStore)(nil)

// Add inserts a pending record and returns its assigned ID. nextRetry
// defaults to now; zero time means now.
func (s *MemoryStore) Add(rec Record, nextRetry time.Time) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	rec.ID = s.nextID
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	if nextRetry.IsZero() {
		nextRetry = time.Now()
	}
	s.rows = append(s.rows, &memRow{rec: rec, status: StatusPending, nextRetry: nextRetry})
	return rec.ID
}

// Fetch implements [Store.Fetch]. Leased rows are hidden until their lease
// expires (crudely standing in for SKIP LOCKED's row-lock timeout).
func (s *MemoryStore) Fetch(ctx context.Context, now time.Time, limit int) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	for _, r := range s.rows {
		if len(out) >= limit {
			break
		}
		if r.status != StatusPending || r.nextRetry.After(now) {
			continue
		}
		if r.leased && now.Before(r.leasedTill) {
			continue
		}
		r.leased, r.leasedTill = true, now.Add(30*time.Second)
		out = append(out, r.rec)
	}
	return out, nil
}

// MarkSent implements [Store.MarkSent].
func (s *MemoryStore) MarkSent(ctx context.Context, id int64, sentAt time.Time) error {
	return s.transition(id, StatusPending, func(r *memRow) { r.status = StatusSent; r.sentAt = sentAt })
}

// MarkFailed implements [Store.MarkFailed].
func (s *MemoryStore) MarkFailed(ctx context.Context, id int64, err error, nextRetryAt time.Time) error {
	return s.transition(id, StatusPending, func(r *memRow) {
		r.rec.Attempts++
		if err != nil {
			r.rec.LastError = err.Error()
		}
		r.nextRetry = nextRetryAt
		r.leased = false
	})
}

// MarkDead implements [Store.MarkDead].
func (s *MemoryStore) MarkDead(ctx context.Context, id int64, err error) error {
	return s.transition(id, StatusPending, func(r *memRow) {
		r.status = StatusDead
		if err != nil {
			r.rec.LastError = err.Error()
		}
		r.leased = false
	})
}

// transition applies fn to the pending row with id, or returns an error.
func (s *MemoryStore) transition(id int64, want string, fn func(*memRow)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.rec.ID != id {
			continue
		}
		if r.status != want {
			return errors.New("outbox: MemStore transition on non-pending record")
		}
		fn(r)
		return nil
	}
	return errors.New("outbox: MemStore record not found")
}

// Snapshot returns a copy of all rows by status, for assertions.
func (s *MemoryStore) Snapshot(status string) []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	for _, r := range s.rows {
		if r.status == status {
			out = append(out, r.rec)
		}
	}
	return out
}
