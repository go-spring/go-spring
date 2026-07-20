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

package transaction

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"
)

// ErrSnapshotNotFound is returned by [Store.Load] when no saga log exists for the
// requested id.
var ErrSnapshotNotFound = errors.New("transaction: saga snapshot not found")

// Snapshot is the persisted state of a running or finished [Saga] — the saga
// log. A [Store] writes one after each committed step and on the terminal
// outcome, so a recovering process can tell how far a saga got and which step
// results it must feed to compensation.
type Snapshot struct {
	// ID is the saga's idempotency key.
	ID string

	// Method is the logical method name the saga belongs to. Recovery uses it to
	// look the step definitions back up in a [StepRegistry], since the log stores
	// only progress data, not the (unpersistable) step functions.
	Method string

	// Status is the last known status. While steps are still running it is
	// StatusRunning; it becomes StatusCompensated or StatusCompensationFailed
	// only at the terminal write. A committed saga's log is deleted, so a stored
	// snapshot is never StatusCommitted.
	Status Status

	// Completed lists, in execution order, the names of steps whose Action
	// succeeded. Compensation replays these in reverse.
	Completed []string

	// InProgress is the name of the step whose Action had started but was not
	// confirmed complete when the snapshot was written (empty when none is in
	// flight). Because a crash may have interrupted it after a side effect,
	// recovery compensates it too — before the completed steps — with a nil
	// result, since its Action return value was never recorded.
	InProgress string

	// StepResults maps each completed step's name to its Action result, so
	// compensation can target exactly what was created.
	StepResults map[string]any

	// UpdatedAt is when this snapshot was written.
	UpdatedAt time.Time
}

// Store persists the saga log. It is the single pluggable seam of this package:
// the bundled [MemoryStore] keeps everything in process (enough for the common
// "single gateway process drives several downstreams" case and for tests), while
// a starter may register a gorm/redis/etcd-backed Store to survive a crash — the
// [Coordinator] is unchanged either way. A nil Store disables persistence, which
// is acceptable in development but not in production where a crash would strand a
// half-finished saga. Implementations must be safe for concurrent use.
type Store interface {
	// Save writes (or overwrites) the snapshot for id.
	Save(ctx context.Context, id string, snap Snapshot) error

	// Load returns the snapshot for id, or [ErrSnapshotNotFound] if absent.
	Load(ctx context.Context, id string) (Snapshot, error)

	// Delete removes the snapshot for id. Deleting an absent id is not an error,
	// so terminal cleanup is idempotent.
	Delete(ctx context.Context, id string) error

	// Pending returns every snapshot still in [StatusRunning] — the sagas a
	// process should resume after a crash. The order is unspecified.
	Pending(ctx context.Context) ([]Snapshot, error)
}

// MemoryStore is a zero-dependency, concurrency-safe in-process [Store]. State is
// lost when the process exits, so it does not provide crash recovery; back the
// Store with a durable implementation in production. The zero value is ready to
// use.
type MemoryStore struct {
	mu    sync.RWMutex
	snaps map[string]Snapshot
}

// Save implements [Store]. It stores a copy so later mutations of snap by the
// caller do not alias stored state.
func (m *MemoryStore) Save(_ context.Context, id string, snap Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.snaps == nil {
		m.snaps = make(map[string]Snapshot)
	}
	m.snaps[id] = cloneSnapshot(snap)
	return nil
}

// Load implements [Store]. It returns a copy, so the caller cannot mutate stored
// state through the returned snapshot.
func (m *MemoryStore) Load(_ context.Context, id string) (Snapshot, error) {
	m.mu.RLock()
	snap, ok := m.snaps[id]
	m.mu.RUnlock()
	if !ok {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return cloneSnapshot(snap), nil
}

// Delete implements [Store].
func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.snaps, id)
	m.mu.Unlock()
	return nil
}

// Pending implements [Store], returning copies of every StatusRunning snapshot.
func (m *MemoryStore) Pending(_ context.Context) ([]Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Snapshot
	for _, snap := range m.snaps {
		if snap.Status == StatusRunning {
			out = append(out, cloneSnapshot(snap))
		}
	}
	return out, nil
}

// cloneSnapshot deep-copies the slice/map fields so stored and returned
// snapshots do not share backing arrays with the caller.
func cloneSnapshot(snap Snapshot) Snapshot {
	if snap.Completed != nil {
		snap.Completed = append([]string(nil), snap.Completed...)
	}
	if snap.StepResults != nil {
		results := make(map[string]any, len(snap.StepResults))
		maps.Copy(results, snap.StepResults)
		snap.StepResults = results
	}
	return snap
}
