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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB opens an in-memory sqlite database with the outbox table created.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, Migrate(db))
	return db
}

func TestPublish_CommitsWithTransaction(t *testing.T) {
	db := newTestDB(t)

	// Committed transaction leaves the outbox row.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return Publish(tx, "orders", "k1", []byte("committed"), map[string]string{"h": "v"})
	}))
	var n int64
	require.NoError(t, db.Model(&outboxRow{}).Count(&n).Error)
	assert.Equal(t, int64(1), n)

	// Rolled-back transaction leaves nothing — the atomicity guarantee.
	require.Error(t, db.Transaction(func(tx *gorm.DB) error {
		if err := Publish(tx, "orders", "k2", []byte("rolled-back"), nil); err != nil {
			return err
		}
		return errors.New("rollback")
	}))
	require.NoError(t, db.Model(&outboxRow{}).Count(&n).Error)
	assert.Equal(t, int64(1), n)
}

func TestStore_FetchOrderAndRetryFilter(t *testing.T) {
	db := newTestDB(t)
	store := newStore(db)
	ctx := context.Background()

	// Insert two rows: one due now, one due in the future.
	due := &outboxRow{Destination: "orders", Payload: []byte("m1"),
		Status: "pending", NextRetryAt: time.Now().Add(-time.Minute), CreatedAt: time.Now()}
	require.NoError(t, db.Create(due).Error)
	require.NoError(t, db.Create(&outboxRow{Destination: "orders", Payload: []byte("m2"),
		Status: "pending", NextRetryAt: time.Now().Add(time.Hour), CreatedAt: time.Now()}).Error)

	recs, err := store.Fetch(ctx, time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "m1", string(recs[0].Payload))
	assert.Equal(t, "orders", recs[0].Destination)

	// SQLite: no SKIP LOCKED clause, single-writer serializes fetches.
	assert.False(t, store.skipLocked)
}

func TestStore_MarkTransitions(t *testing.T) {
	db := newTestDB(t)
	store := newStore(db)
	ctx := context.Background()

	row := &outboxRow{Destination: "orders", Payload: []byte("m1"),
		Status: "pending", NextRetryAt: time.Now().Add(-time.Minute), CreatedAt: time.Now()}
	require.NoError(t, db.Create(row).Error)

	// MarkFailed bumps attempts and the retry time.
	require.NoError(t, store.MarkFailed(ctx, row.ID, errors.New("boom"), time.Now().Add(time.Minute)))
	var after outboxRow
	require.NoError(t, db.First(&after, row.ID).Error)
	assert.Equal(t, 1, after.Attempts)
	assert.Contains(t, after.LastError, "boom")
	assert.True(t, after.NextRetryAt.After(time.Now()))

	// Not due yet → fetch returns nothing.
	recs, err := store.Fetch(ctx, time.Now(), 10)
	require.NoError(t, err)
	assert.Empty(t, recs)

	// MarkSent flips status.
	require.NoError(t, store.MarkSent(ctx, row.ID, time.Now()))
	require.NoError(t, db.First(&after, row.ID).Error)
	assert.Equal(t, "sent", after.Status)
	assert.NotNil(t, after.SentAt)
}

func TestStore_MarkDeadTerminal(t *testing.T) {
	db := newTestDB(t)
	store := newStore(db)
	ctx := context.Background()

	row := &outboxRow{Destination: "orders", Payload: []byte("poison"),
		Status: "pending", NextRetryAt: time.Now().Add(-time.Minute), CreatedAt: time.Now()}
	require.NoError(t, db.Create(row).Error)

	require.NoError(t, store.MarkDead(ctx, row.ID, errors.New("exhausted")))
	var after outboxRow
	require.NoError(t, db.First(&after, row.ID).Error)
	assert.Equal(t, "dead", after.Status)

	// Dead rows are never fetched again.
	recs, err := store.Fetch(ctx, time.Now(), 10)
	require.NoError(t, err)
	assert.Empty(t, recs)

	// Marking a dead row again errors (no longer pending).
	require.Error(t, store.MarkDead(ctx, row.ID, nil))
}

func TestStore_HeadersRoundTrip(t *testing.T) {
	db := newTestDB(t)
	store := newStore(db)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return Publish(tx, "orders", "k1", []byte("m1"), map[string]string{"trace": "abc", "h2": "v2"})
	}))
	recs, err := store.Fetch(context.Background(), time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, map[string]string{"trace": "abc", "h2": "v2"}, recs[0].Headers)
	assert.Equal(t, "k1", recs[0].Key)
}
