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
	"time"

	"gorm.io/gorm"
)

// outboxTableName is the table the write side inserts into and the relay
// drains. Publishing and business writes commit atomically because they share
// one database transaction.
const outboxTableName = "outbox_message"

// outboxRow is one pending message. It is written by [Publish] inside the
// business transaction and resolved by the relay's gormStore.
type outboxRow struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Destination string    `gorm:"column:destination;index:idx_outbox_dispatch,priority:1"`
	Key         string    `gorm:"column:msg_key"`
	Payload     []byte    `gorm:"column:payload"`
	Headers     string    `gorm:"column:headers;type:text"` // JSON map[string]string
	Status      string    `gorm:"column:status;index:idx_outbox_dispatch,priority:2"`
	Attempts    int       `gorm:"column:attempts"`
	NextRetryAt time.Time `gorm:"column:next_retry_at"`
	LastError   string    `gorm:"column:last_error;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	SentAt      *time.Time `gorm:"column:sent_at"`
}

// TableName pins the table name regardless of gorm's pluralization rules.
func (outboxRow) TableName() string { return outboxTableName }

// Migrate creates the outbox_message table on db if it does not exist. It runs
// when the starter's auto-migrate is enabled; applications that manage their
// schema with a migration tool should create the table from the documented DDL
// instead and leave auto-migrate off.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&outboxRow{})
}
