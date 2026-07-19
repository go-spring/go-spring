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

package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// memStore is an in-memory [Store] for exercising the Runner without a database.
// It records applied migrations and every statement each migration's Up issued,
// so tests can assert both the version-table state and the exact execution.
type memStore struct {
	table       []Record
	execLog     []string
	failVersion uint64 // when >0, Apply of this version returns an error
	ensured     int
}

type logExecer struct{ store *memStore }

func (e logExecer) ExecContext(_ context.Context, query string, _ ...any) error {
	e.store.execLog = append(e.store.execLog, query)
	return nil
}

func (m *memStore) EnsureVersionTable(context.Context) error { m.ensured++; return nil }

func (m *memStore) AppliedRecords(context.Context) ([]Record, error) {
	return append([]Record(nil), m.table...), nil
}

func (m *memStore) Apply(ctx context.Context, mig Migration) error {
	if m.failVersion != 0 && mig.Version == m.failVersion {
		return errors.New("boom")
	}
	if mig.Up != nil {
		if err := mig.Up(ctx, logExecer{m}); err != nil {
			return err
		}
	}
	m.table = append(m.table, Record{mig.Version, mig.Name, mig.Checksum, time.Now()})
	return nil
}

func (m *memStore) MarkApplied(_ context.Context, mig Migration) error {
	m.table = append(m.table, Record{mig.Version, mig.Name, mig.Checksum, time.Now()})
	return nil
}

// upStmt returns an Up that issues one identifiable statement.
func upStmt(stmt string) func(context.Context, Execer) error {
	return func(ctx context.Context, ex Execer) error { return ex.ExecContext(ctx, stmt) }
}

func TestMigrate_AppliesInOrderThenIdempotent(t *testing.T) {
	store := &memStore{}
	src := NewSource(
		Migration{Version: 2, Name: "b", Checksum: "c2", Up: upStmt("s2")},
		Migration{Version: 1, Name: "a", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 10, Name: "c", Checksum: "c10", Up: upStmt("s10")},
	)
	r := NewRunner(store, src, Options{})

	// First run applies all three in ascending version order.
	done, err := r.Migrate(context.Background())
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(3)
	assert.That(t, store.execLog).Equal([]string{"s1", "s2", "s10"})
	assert.That(t, len(store.table)).Equal(3)

	// Second run is a no-op: nothing applied, no new statements executed.
	store.execLog = nil
	done, err = r.Migrate(context.Background())
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(0)
	assert.That(t, len(store.execLog)).Equal(0)
	// The version table is ensured on every run (idempotent create).
	assert.That(t, store.ensured).Equal(2)
}

func TestMigrate_ChecksumDriftFailsFast(t *testing.T) {
	store := &memStore{}
	src := NewSource(Migration{Version: 1, Name: "a", Checksum: "orig", Up: upStmt("s1")})
	_, err := NewRunner(store, src, Options{}).Migrate(context.Background())
	assert.Error(t, err).Nil()

	// The same version now yields a different checksum — a historical edit.
	edited := NewSource(Migration{Version: 1, Name: "a", Checksum: "edited", Up: upStmt("s1b")})
	_, err = NewRunner(store, edited, Options{}).Migrate(context.Background())
	assert.Error(t, err).Matches("checksum mismatch for version 1")
}

func TestMigrate_OutOfOrderRejectedByDefault(t *testing.T) {
	store := &memStore{}
	// Apply versions 1 and 3 first.
	first := NewSource(
		Migration{Version: 1, Name: "a", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 3, Name: "c", Checksum: "c3", Up: upStmt("s3")},
	)
	_, err := NewRunner(store, first, Options{}).Migrate(context.Background())
	assert.Error(t, err).Nil()

	// A late-arriving version 2 sits below the highest applied (3): rejected.
	gap := NewSource(
		Migration{Version: 1, Name: "a", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 2, Name: "b", Checksum: "c2", Up: upStmt("s2")},
		Migration{Version: 3, Name: "c", Checksum: "c3", Up: upStmt("s3")},
	)
	_, err = NewRunner(store, gap, Options{}).Migrate(context.Background())
	assert.Error(t, err).Matches("out-of-order migration")
}

func TestMigrate_OutOfOrderAllowed(t *testing.T) {
	store := &memStore{}
	first := NewSource(
		Migration{Version: 1, Name: "a", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 3, Name: "c", Checksum: "c3", Up: upStmt("s3")},
	)
	_, err := NewRunner(store, first, Options{}).Migrate(context.Background())
	assert.Error(t, err).Nil()

	gap := NewSource(
		Migration{Version: 1, Name: "a", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 2, Name: "b", Checksum: "c2", Up: upStmt("s2")},
		Migration{Version: 3, Name: "c", Checksum: "c3", Up: upStmt("s3")},
	)
	done, err := NewRunner(store, gap, Options{AllowOutOfOrder: true}).Migrate(context.Background())
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(1)
	assert.That(t, done[0].Version).Equal(uint64(2))
}

func TestMigrate_BaselineMarksWithoutRunning(t *testing.T) {
	store := &memStore{}
	src := NewSource(
		Migration{Version: 1, Name: "legacy", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 2, Name: "new", Checksum: "c2", Up: upStmt("s2")},
	)
	done, err := NewRunner(store, src, Options{Baseline: 1}).Migrate(context.Background())
	assert.Error(t, err).Nil()
	// v1 recorded as baseline (not executed); only v2 actually ran.
	assert.That(t, len(done)).Equal(1)
	assert.That(t, done[0].Version).Equal(uint64(2))
	assert.That(t, store.execLog).Equal([]string{"s2"})
	assert.That(t, len(store.table)).Equal(2)
}

func TestMigrate_ApplyFailureAbortsAndStopsFurtherWork(t *testing.T) {
	store := &memStore{failVersion: 2}
	src := NewSource(
		Migration{Version: 1, Name: "a", Checksum: "c1", Up: upStmt("s1")},
		Migration{Version: 2, Name: "b", Checksum: "c2", Up: upStmt("s2")},
		Migration{Version: 3, Name: "c", Checksum: "c3", Up: upStmt("s3")},
	)
	done, err := NewRunner(store, src, Options{}).Migrate(context.Background())
	assert.Error(t, err).Matches("apply version 2")
	// v1 committed; v2 failed; v3 never attempted.
	assert.That(t, len(done)).Equal(1)
	assert.That(t, store.execLog).Equal([]string{"s1"})
}

func TestMigrate_RejectsZeroAndDuplicateVersions(t *testing.T) {
	store := &memStore{}
	_, err := NewRunner(store, NewSource(Migration{Version: 0, Name: "z"}), Options{}).Migrate(context.Background())
	assert.Error(t, err).Matches("version must be > 0")

	dup := NewSource(
		Migration{Version: 1, Name: "a", Checksum: "c1"},
		Migration{Version: 1, Name: "b", Checksum: "c2"},
	)
	_, err = NewRunner(store, dup, Options{}).Migrate(context.Background())
	assert.Error(t, err).Matches("duplicate version 1")
}
