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
	"encoding/json"
	"errors"
	"time"

	"go-spring.org/spring/cloud/transaction"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sagaSnapshot is the persisted row for one saga log, in table saga_snapshots.
// The slice/map fields of a [transaction.Snapshot] are stored JSON-encoded in
// text columns so the schema stays backend-agnostic (no array/JSON column
// types required). Status is stored as its int value and indexed so Pending can
// scan for in-flight sagas cheaply.
type sagaSnapshot struct {
	ID          string    `gorm:"primaryKey;column:id"`
	Method      string    `gorm:"column:method"`
	Status      int       `gorm:"column:status;index"`
	InProgress  string    `gorm:"column:in_progress"`
	Completed   string    `gorm:"column:completed;type:text"`    // JSON-encoded []string
	StepResults string    `gorm:"column:step_results;type:text"` // JSON-encoded map[string]any
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

// TableName pins the table name regardless of gorm's pluralization rules.
func (sagaSnapshot) TableName() string { return "saga_snapshots" }

// gormStore is a durable [transaction.Store] backed by a *gorm.DB. It lets a
// crashed application recover in-flight sagas after restart: the coordinator
// writes progress here as it runs, and the recovery scan reads it back.
//
// JSON round-trip caveat: step Action results are stored as JSON, so on recovery
// a value comes back in its JSON form — a number becomes float64, a struct
// becomes map[string]any, and so on, not its original Go type. Sagas that must
// survive a crash should keep Action results JSON-friendly (ids, tokens and
// other scalars) and not rely on rich Go types in Compensate. The in-progress
// step is always recovered with a nil result, so it sidesteps this entirely.
type gormStore struct {
	db *gorm.DB
}

var _ transaction.Store = (*gormStore)(nil)

// Save upserts the snapshot for id, overwriting any previous row.
func (s *gormStore) Save(ctx context.Context, id string, snap transaction.Snapshot) error {
	row, err := toRow(id, snap)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(&row).Error
}

// Load returns the snapshot for id, or [transaction.ErrSnapshotNotFound].
func (s *gormStore) Load(ctx context.Context, id string) (transaction.Snapshot, error) {
	var row sagaSnapshot
	err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return transaction.Snapshot{}, transaction.ErrSnapshotNotFound
		}
		return transaction.Snapshot{}, err
	}
	return fromRow(row)
}

// Delete removes the snapshot for id; deleting an absent id is not an error.
func (s *gormStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&sagaSnapshot{}, "id = ?", id).Error
}

// Pending returns every snapshot still in StatusRunning — the sagas to resume.
func (s *gormStore) Pending(ctx context.Context) ([]transaction.Snapshot, error) {
	var rows []sagaSnapshot
	err := s.db.WithContext(ctx).
		Where("status = ?", int(transaction.StatusRunning)).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]transaction.Snapshot, 0, len(rows))
	for _, row := range rows {
		snap, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, nil
}

// toRow encodes a Snapshot into its persisted row form.
func toRow(id string, snap transaction.Snapshot) (sagaSnapshot, error) {
	completed, err := encodeJSON(snap.Completed)
	if err != nil {
		return sagaSnapshot{}, err
	}
	results, err := encodeJSON(snap.StepResults)
	if err != nil {
		return sagaSnapshot{}, err
	}
	updated := snap.UpdatedAt
	if updated.IsZero() {
		updated = time.Now()
	}
	return sagaSnapshot{
		ID:          id,
		Method:      snap.Method,
		Status:      int(snap.Status),
		InProgress:  snap.InProgress,
		Completed:   completed,
		StepResults: results,
		UpdatedAt:   updated,
	}, nil
}

// fromRow decodes a persisted row back into a Snapshot.
func fromRow(row sagaSnapshot) (transaction.Snapshot, error) {
	var completed []string
	if err := decodeJSON(row.Completed, &completed); err != nil {
		return transaction.Snapshot{}, err
	}
	var results map[string]any
	if err := decodeJSON(row.StepResults, &results); err != nil {
		return transaction.Snapshot{}, err
	}
	return transaction.Snapshot{
		ID:          row.ID,
		Method:      row.Method,
		Status:      transaction.Status(row.Status),
		Completed:   completed,
		InProgress:  row.InProgress,
		StepResults: results,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// encodeJSON marshals v, returning "" for nil so an empty column round-trips to
// a nil slice/map rather than a spurious empty value.
func encodeJSON(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeJSON unmarshals s into dst, treating "" as "leave dst at its zero value".
func decodeJSON(s string, dst any) error {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), dst)
}
