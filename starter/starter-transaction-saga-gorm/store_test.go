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

package StarterTransactionSagaGorm

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/spring/cloud/transaction"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestStore opens a fresh in-memory sqlite database and migrates the schema.
func newTestStore(t *testing.T) *gormStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.Error(t, err).Nil()
	assert.Error(t, db.AutoMigrate(&sagaSnapshot{})).Nil()
	return &gormStore{db: db}
}

func TestGormStore_SaveLoadDeletePending(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// A running saga: interrupted while running c, having completed a and b.
	running := transaction.Snapshot{
		ID:          "run",
		Method:      "Svc.Do",
		Status:      transaction.StatusRunning,
		Completed:   []string{"a", "b"},
		InProgress:  "c",
		StepResults: map[string]any{"a": "ra", "b": "rb"},
	}
	assert.Error(t, store.Save(ctx, "run", running)).Nil()

	// A terminal saga kept for inspection.
	assert.Error(t, store.Save(ctx, "gone", transaction.Snapshot{
		ID: "gone", Method: "Svc.Do", Status: transaction.StatusCompensated,
	})).Nil()

	// Load round-trips the fields (string results survive JSON cleanly).
	got, err := store.Load(ctx, "run")
	assert.Error(t, err).Nil()
	assert.That(t, got.ID).Equal("run")
	assert.That(t, got.Method).Equal("Svc.Do")
	assert.That(t, got.Status).Equal(transaction.StatusRunning)
	assert.That(t, got.InProgress).Equal("c")
	assert.Slice(t, got.Completed).Equal([]string{"a", "b"})
	assert.That(t, got.StepResults["a"]).Equal("ra")
	assert.That(t, got.StepResults["b"]).Equal("rb")

	// Pending returns only the running saga.
	pending, err := store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(1)
	assert.That(t, pending[0].ID).Equal("run")

	// Delete removes it; Load then reports not found and Pending is empty.
	assert.Error(t, store.Delete(ctx, "run")).Nil()
	_, err = store.Load(ctx, "run")
	assert.Error(t, err).Is(transaction.ErrSnapshotNotFound)
	pending, err = store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(0)

	// Deleting an absent id is not an error.
	assert.Error(t, store.Delete(ctx, "run")).Nil()
}

func TestGormStore_MissingLoadReturnsNotFound(t *testing.T) {
	_, err := newTestStore(t).Load(context.Background(), "nope")
	assert.Error(t, err).Is(transaction.ErrSnapshotNotFound)
}

func TestGormStore_EndToEndExecuteThenRecover(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// A live execution that fails on its last step: the coordinator compensates
	// and keeps a terminal Compensated log.
	coord := transaction.NewCoordinator(transaction.WithStore(store))
	_, err := coord.Execute(ctx, transaction.Saga{
		ID:     "live",
		Method: "Svc.Do",
		Steps: []transaction.Step{
			{Name: "a", Action: func(context.Context) (any, error) { return "ra", nil },
				Compensate: func(context.Context, any) error { return nil }},
			{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
		},
	})
	assert.Error(t, err).NotNil()
	snap, err := store.Load(ctx, "live")
	assert.Error(t, err).Nil()
	assert.That(t, snap.Status).Equal(transaction.StatusCompensated)

	// Simulate a crash mid-flight: a running snapshot with a completed then an
	// in-progress step, as it would have been persisted before the process died.
	assert.Error(t, store.Save(ctx, "crashed", transaction.Snapshot{
		ID:          "crashed",
		Method:      "Svc.Do",
		Status:      transaction.StatusRunning,
		Completed:   []string{"a"},
		InProgress:  "b",
		StepResults: map[string]any{"a": "ra"},
	})).Nil()

	// A fresh coordinator over the same durable store recovers it: backward
	// recovery compensates the in-flight b first, then the completed a.
	var order []string
	coord2 := transaction.NewCoordinator(transaction.WithStore(store))
	res, err := coord2.Recover(ctx, transaction.Saga{
		ID:     "crashed",
		Method: "Svc.Do",
		Steps: []transaction.Step{
			{Name: "a", Action: func(context.Context) (any, error) { return "ra", nil },
				Compensate: func(context.Context, any) error { order = append(order, "!a"); return nil }},
			{Name: "b", Action: func(context.Context) (any, error) { return "rb", nil },
				Compensate: func(context.Context, any) error { order = append(order, "!b"); return nil }},
			{Name: "c", Action: func(context.Context) (any, error) { return "rc", nil },
				Compensate: func(context.Context, any) error { order = append(order, "!c"); return nil }},
		},
	})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(transaction.StatusCompensated)
	// c was never reached by the log, so it is not compensated; b then a are.
	assert.Slice(t, order).Equal([]string{"!b", "!a"})

	got, err := store.Load(ctx, "crashed")
	assert.Error(t, err).Nil()
	assert.That(t, got.Status).Equal(transaction.StatusCompensated)
}
