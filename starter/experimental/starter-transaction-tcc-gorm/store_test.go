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

package StarterTransactionTccGorm

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/experimental/transaction/tcc"
	"go-spring.org/stdlib/testing/assert"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestStore opens a fresh in-memory sqlite database and migrates the schema.
func newTestStore(t *testing.T) *gormStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.Error(t, err).Nil()
	assert.Error(t, db.AutoMigrate(&tccSnapshot{})).Nil()
	return &gormStore{db: db}
}

func TestGormStore_SaveLoadDeletePending(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// An in-flight transaction: trying, having tried a and b, with c in progress.
	trying := tcc.Snapshot{
		ID:         "run",
		Method:     "Svc.Do",
		Status:     tcc.StatusTrying,
		Tried:      []string{"a", "b"},
		InProgress: "c",
		TryResults: map[string]any{"a": "ra", "b": "rb"},
	}
	assert.Error(t, store.Save(ctx, "run", trying)).Nil()

	// A terminal transaction kept for inspection.
	assert.Error(t, store.Save(ctx, "gone", tcc.Snapshot{
		ID: "gone", Method: "Svc.Do", Status: tcc.StatusCancelled,
	})).Nil()

	// Load round-trips the fields (string results survive JSON cleanly).
	got, err := store.Load(ctx, "run")
	assert.Error(t, err).Nil()
	assert.That(t, got.ID).Equal("run")
	assert.That(t, got.Method).Equal("Svc.Do")
	assert.That(t, got.Status).Equal(tcc.StatusTrying)
	assert.That(t, got.InProgress).Equal("c")
	assert.Slice(t, got.Tried).Equal([]string{"a", "b"})
	assert.That(t, got.TryResults["a"]).Equal("ra")
	assert.That(t, got.TryResults["b"]).Equal("rb")

	// Pending returns only the non-terminal transaction.
	pending, err := store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(1)
	assert.That(t, pending[0].ID).Equal("run")

	// Delete removes it; Load then reports not found and Pending is empty.
	assert.Error(t, store.Delete(ctx, "run")).Nil()
	_, err = store.Load(ctx, "run")
	assert.Error(t, err).Is(tcc.ErrSnapshotNotFound)
	pending, err = store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(0)

	// Deleting an absent id is not an error.
	assert.Error(t, store.Delete(ctx, "run")).Nil()
}

func TestGormStore_MissingLoadReturnsNotFound(t *testing.T) {
	_, err := newTestStore(t).Load(context.Background(), "nope")
	assert.Error(t, err).Is(tcc.ErrSnapshotNotFound)
}

func TestGormStore_EndToEndExecuteThenRecover(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	coord := tcc.NewCoordinator(tcc.WithStore(store))

	// A healthy transaction commits: its log is deleted, so nothing is pending.
	res, err := coord.Execute(ctx, tcc.Transaction{
		ID:     "live",
		Method: "Svc.Do",
		Participants: []tcc.Participant{
			{Name: "a",
				Try:     func(context.Context) (any, error) { return "ra", nil },
				Confirm: func(context.Context, any) error { return nil },
				Cancel:  func(context.Context, any) error { return nil }},
		},
	})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(tcc.StatusCommitted)
	_, err = store.Load(ctx, "live")
	assert.Error(t, err).Is(tcc.ErrSnapshotNotFound)

	// A failing transaction cancels and keeps a terminal log.
	_, err = coord.Execute(ctx, tcc.Transaction{
		ID:     "rolled",
		Method: "Svc.Do",
		Participants: []tcc.Participant{
			{Name: "a",
				Try:     func(context.Context) (any, error) { return "ra", nil },
				Confirm: func(context.Context, any) error { return nil },
				Cancel:  func(context.Context, any) error { return nil }},
			{Name: "b",
				Try:     func(context.Context) (any, error) { return nil, errors.New("boom") },
				Confirm: func(context.Context, any) error { return nil },
				Cancel:  func(context.Context, any) error { return nil }},
		},
	})
	assert.Error(t, err).NotNil()
	snap, err := store.Load(ctx, "rolled")
	assert.Error(t, err).Nil()
	assert.That(t, snap.Status).Equal(tcc.StatusCancelled)

	// Simulate a crash mid-flight: a trying snapshot as it would be persisted
	// before the process died. A fresh coordinator over the same durable store
	// finds it pending, so recovery has something to resume.
	assert.Error(t, store.Save(ctx, "crashed", tcc.Snapshot{
		ID:         "crashed",
		Method:     "Svc.Do",
		Status:     tcc.StatusTrying,
		Tried:      []string{"a"},
		InProgress: "b",
		TryResults: map[string]any{"a": "ra"},
	})).Nil()

	pending, err := store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(1)
	assert.That(t, pending[0].ID).Equal("crashed")
	assert.That(t, pending[0].InProgress).Equal("b")
}
