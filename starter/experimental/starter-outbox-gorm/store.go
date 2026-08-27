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

package StarterOutboxGorm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go-spring.org/cloud/experimental/outbox"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// gormStore implements [outbox.Store] on the outbox_message table.
type gormStore struct {
	db *gorm.DB

	// skipLocked reports whether the dialect understands FOR UPDATE SKIP
	// LOCKED (mysql 8+, postgres). SQLite and older MySQL run without the
	// lock clause: SQLite is a single writer so concurrent fetches cannot
	// interleave; on MySQL 5.7 run a single relay instance per table.
	skipLocked bool
}

// newStore builds a store over db, probing the dialect for SKIP LOCKED.
func newStore(db *gorm.DB) *gormStore {
	s := &gormStore{db: db}
	switch db.Dialector.Name() {
	case "mysql", "postgres":
		s.skipLocked = true
	}
	return s
}

// gormStore implements [outbox.Store].
var _ outbox.Store = (*gormStore)(nil)

// Fetch implements [outbox.Store.Fetch]. With SKIP LOCKED, concurrent relay
// instances never take the same pending row; without it (single-writer or
// single-instance deployments) the status filter plus ID ordering keep fetches
// deterministic.
func (s *gormStore) Fetch(ctx context.Context, now time.Time, limit int) ([]outbox.Record, error) {
	q := s.db.WithContext(ctx).
		Where("status = ? AND next_retry_at <= ?", "pending", now).
		Order("id").Limit(limit)
	if s.skipLocked {
		q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
	}
	var rows []outboxRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	recs := make([]outbox.Record, 0, len(rows))
	for i := range rows {
		recs = append(recs, toRecord(&rows[i]))
	}
	return recs, nil
}

// MarkSent implements [outbox.Store.MarkSent].
func (s *gormStore) MarkSent(ctx context.Context, id int64, sentAt time.Time) error {
	return s.db.WithContext(ctx).Model(&outboxRow{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{"status": "sent", "sent_at": sentAt}).Error
}

// MarkFailed implements [outbox.Store.MarkFailed].
func (s *gormStore) MarkFailed(ctx context.Context, id int64, err error, nextRetryAt time.Time) error {
	lastErr := ""
	if err != nil {
		lastErr = err.Error()
	}
	return s.db.WithContext(ctx).Model(&outboxRow{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{
			"attempts":      gorm.Expr("attempts + 1"),
			"last_error":    lastErr,
			"next_retry_at": nextRetryAt,
		}).Error
}

// MarkDead implements [outbox.Store.MarkDead].
func (s *gormStore) MarkDead(ctx context.Context, id int64, err error) error {
	lastErr := ""
	if err != nil {
		lastErr = err.Error()
	}
	res := s.db.WithContext(ctx).Model(&outboxRow{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{"status": "dead", "last_error": lastErr})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("outbox: record is no longer pending")
	}
	return nil
}

// toRecord projects a stored row to the neutral record.
func toRecord(row *outboxRow) outbox.Record {
	var headers map[string]string
	if row.Headers != "" {
		_ = json.Unmarshal([]byte(row.Headers), &headers)
	}
	return outbox.Record{
		ID:          row.ID,
		Destination: row.Destination,
		Key:         row.Key,
		Payload:     row.Payload,
		Headers:     headers,
		Attempts:    row.Attempts,
		LastError:   row.LastError,
		CreatedAt:   row.CreatedAt,
	}
}
