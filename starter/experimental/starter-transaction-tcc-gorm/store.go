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
	"encoding/json"
	"errors"
	"time"

	"go-spring.org/cloud/experimental/transaction/tcc"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// tccSnapshot is the persisted row for one TCC log, in table tcc_snapshots. The
// slice/map fields of a [tcc.Snapshot] are stored JSON-encoded in text columns
// so the schema stays backend-agnostic (no array/JSON column types required).
// Status is stored as its int value and indexed so Pending can scan for
// in-flight transactions cheaply.
type tccSnapshot struct {
	ID         string    `gorm:"primaryKey;column:id"`
	Method     string    `gorm:"column:method"`
	Status     int       `gorm:"column:status;index"`
	Tried      string    `gorm:"column:tried;type:text"` // JSON-encoded []string
	InProgress string    `gorm:"column:in_progress"`
	TryResults string    `gorm:"column:try_results;type:text"` // JSON-encoded map[string]any
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

// TableName pins the table name regardless of gorm's pluralization rules.
func (tccSnapshot) TableName() string { return "tcc_snapshots" }

// gormStore is a durable [tcc.Store] backed by a *gorm.DB. It lets a crashed
// application recover in-flight transactions after restart: the coordinator
// writes progress here as it runs, and the recovery scan reads it back.
//
// JSON round-trip caveat: Try results are stored as JSON, so on recovery a value
// comes back in its JSON form — a number becomes float64, a struct becomes
// map[string]any, and so on, not its original Go type. Transactions that must
// survive a crash should keep Try results JSON-friendly (ids, tokens and other
// scalars) and not rely on rich Go types in Confirm/Cancel. The in-progress
// participant is always recovered with a nil result, so it sidesteps this
// entirely.
type gormStore struct {
	db *gorm.DB
}

var _ tcc.Store = (*gormStore)(nil)

// nonTerminalStatuses is the set Pending scans for — every status a transaction
// can be in before it reaches StatusCommitted or StatusCancelled.
var nonTerminalStatuses = []int{
	int(tcc.StatusTrying),
	int(tcc.StatusConfirming),
	int(tcc.StatusCancelling),
}

// Save upserts the snapshot for id, overwriting any previous row.
func (s *gormStore) Save(ctx context.Context, id string, snap tcc.Snapshot) error {
	row, err := toRow(id, snap)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(&row).Error
}

// Load returns the snapshot for id, or [tcc.ErrSnapshotNotFound].
func (s *gormStore) Load(ctx context.Context, id string) (tcc.Snapshot, error) {
	var row tccSnapshot
	err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tcc.Snapshot{}, tcc.ErrSnapshotNotFound
		}
		return tcc.Snapshot{}, err
	}
	return fromRow(row)
}

// Delete removes the snapshot for id; deleting an absent id is not an error.
func (s *gormStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&tccSnapshot{}, "id = ?", id).Error
}

// Pending returns every snapshot not yet in a terminal state — the transactions
// a process should resume after a crash.
func (s *gormStore) Pending(ctx context.Context) ([]tcc.Snapshot, error) {
	var rows []tccSnapshot
	err := s.db.WithContext(ctx).
		Where("status IN ?", nonTerminalStatuses).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]tcc.Snapshot, 0, len(rows))
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
func toRow(id string, snap tcc.Snapshot) (tccSnapshot, error) {
	tried, err := encodeJSON(snap.Tried)
	if err != nil {
		return tccSnapshot{}, err
	}
	results, err := encodeJSON(snap.TryResults)
	if err != nil {
		return tccSnapshot{}, err
	}
	updated := snap.UpdatedAt
	if updated.IsZero() {
		updated = time.Now()
	}
	return tccSnapshot{
		ID:         id,
		Method:     snap.Method,
		Status:     int(snap.Status),
		Tried:      tried,
		InProgress: snap.InProgress,
		TryResults: results,
		UpdatedAt:  updated,
	}, nil
}

// fromRow decodes a persisted row back into a Snapshot.
func fromRow(row tccSnapshot) (tcc.Snapshot, error) {
	var tried []string
	if err := decodeJSON(row.Tried, &tried); err != nil {
		return tcc.Snapshot{}, err
	}
	var results map[string]any
	if err := decodeJSON(row.TryResults, &results); err != nil {
		return tcc.Snapshot{}, err
	}
	return tcc.Snapshot{
		ID:         row.ID,
		Method:     row.Method,
		Status:     tcc.Status(row.Status),
		Tried:      tried,
		InProgress: row.InProgress,
		TryResults: results,
		UpdatedAt:  row.UpdatedAt,
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
