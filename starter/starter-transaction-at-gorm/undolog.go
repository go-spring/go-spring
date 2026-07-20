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

package StarterTransactionATGorm

import (
	"encoding/json"
	"time"

	"go-spring.org/spring/cloud/transaction/at"
	"gorm.io/gorm"
)

// undoTableName is the table AT writes its undo logs to. Callbacks skip it so a
// captured statement's own undo-log write does not recurse into capture.
const undoTableName = "at_undo_log"

// undoRow is the persisted undo log for one intercepted DML statement. It is
// written in the same local transaction as the business data (so the two commit
// atomically) and read back on rollback to restore the affected rows.
//
// The before/after images are JSON-encoded into a single text column so the
// schema stays backend-agnostic — no array/JSON column type is required, exactly
// as starter-transaction-saga-gorm stores its snapshots.
type undoRow struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id"`
	XID       string    `gorm:"column:xid;index"`
	BranchID  string    `gorm:"column:branch_id"`
	Table     string    `gorm:"column:table_name"`
	SQLType   int       `gorm:"column:sql_type"`
	Context   string    `gorm:"column:context;type:text"` // JSON-encoded undoContext
	CreatedAt time.Time `gorm:"column:created_at"`
}

// TableName pins the table name regardless of gorm's pluralization rules.
func (undoRow) TableName() string { return undoTableName }

// undoContext is the JSON payload of undoRow.Context: the before-image (used to
// restore updates and re-insert deletes) and the after-image (used to delete
// inserts, and available for dirty-write inspection).
type undoContext struct {
	Before at.RecordImage `json:"before"`
	After  at.RecordImage `json:"after"`
}

// encodeContext marshals the before/after images for storage.
func encodeContext(before, after at.RecordImage) (string, error) {
	b, err := json.Marshal(undoContext{Before: before, After: after})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeContext unmarshals a stored undo context.
func decodeContext(s string) (undoContext, error) {
	var c undoContext
	if s == "" {
		return c, nil
	}
	err := json.Unmarshal([]byte(s), &c)
	return c, err
}

// Migrate creates the at_undo_log table on db if it does not exist. A backend
// starter or application calls it once at startup (fail-fast) before enabling AT
// on that database.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&undoRow{})
}
