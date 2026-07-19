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
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/transaction/at"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// account is a toy business table this test drives AT over.
type account struct {
	ID      int64 `gorm:"primaryKey;autoIncrement;column:id"`
	Balance int   `gorm:"column:balance"`
}

func (account) TableName() string { return "account" }

// newTestDB opens a fresh in-memory sqlite database, migrates the undo-log and
// business tables and installs the AT plugin bound to coord/lock. MaxOpenConns=1
// keeps the whole test on one connection so the shared :memory: database is not
// silently duplicated per connection.
func newTestDB(t *testing.T, coord at.Coordinator, lock at.GlobalLock) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.Error(t, err).Nil()
	sqlDB, err := db.DB()
	assert.Error(t, err).Nil()
	sqlDB.SetMaxOpenConns(1)
	assert.Error(t, Migrate(db)).Nil()
	assert.Error(t, db.AutoMigrate(&account{})).Nil()
	assert.Error(t, db.Use(NewPlugin("acct", coord, lock))).Nil()
	return db
}

// undoCount reports how many undo-log rows exist for xid.
func undoCount(t *testing.T, db *gorm.DB, xid string) int64 {
	t.Helper()
	var n int64
	assert.Error(t, db.Model(&undoRow{}).Where("xid = ?", xid).Count(&n).Error).Nil()
	return n
}

func TestAT_UpdateRollbackRestoresBeforeImage(t *testing.T) {
	ctx := context.Background()
	coord := at.NewCoordinator(at.WithGlobalLock(&at.MemoryGlobalLock{}))
	db := newTestDB(t, coord, &at.MemoryGlobalLock{})

	assert.Error(t, db.Create(&account{ID: 1, Balance: 100}).Error).Nil()

	// A global transaction that updates the balance, then rolls back.
	gctx, xid := coord.Begin(ctx)
	err := db.WithContext(gctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).Update("balance", 40).Error
	})
	assert.Error(t, err).Nil()
	assert.That(t, undoCount(t, db, xid)).Equal(int64(1))

	// Mid-transaction the new value is visible.
	var acc account
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(40)

	// Rollback restores the before-image and drops the undo log.
	assert.Error(t, coord.Rollback(ctx, xid)).Nil()
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(100)
	assert.That(t, undoCount(t, db, xid)).Equal(int64(0))
}

func TestAT_UpdateCommitClearsUndoLog(t *testing.T) {
	ctx := context.Background()
	coord := at.NewCoordinator(at.WithGlobalLock(&at.MemoryGlobalLock{}))
	db := newTestDB(t, coord, &at.MemoryGlobalLock{})

	assert.Error(t, db.Create(&account{ID: 1, Balance: 100}).Error).Nil()

	gctx, xid := coord.Begin(ctx)
	err := db.WithContext(gctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).Update("balance", 40).Error
	})
	assert.Error(t, err).Nil()

	// Commit keeps the business change and drops the undo log.
	assert.Error(t, coord.Commit(ctx, xid)).Nil()
	var acc account
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(40)
	assert.That(t, undoCount(t, db, xid)).Equal(int64(0))
}

func TestAT_InsertRollbackDeletesRow(t *testing.T) {
	ctx := context.Background()
	coord := at.NewCoordinator(at.WithGlobalLock(&at.MemoryGlobalLock{}))
	db := newTestDB(t, coord, &at.MemoryGlobalLock{})

	gctx, xid := coord.Begin(ctx)
	err := db.WithContext(gctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&account{ID: 7, Balance: 500}).Error
	})
	assert.Error(t, err).Nil()

	var n int64
	assert.Error(t, db.Model(&account{}).Where("id = ?", 7).Count(&n).Error).Nil()
	assert.That(t, n).Equal(int64(1))

	// Rollback removes the inserted row.
	assert.Error(t, coord.Rollback(ctx, xid)).Nil()
	assert.Error(t, db.Model(&account{}).Where("id = ?", 7).Count(&n).Error).Nil()
	assert.That(t, n).Equal(int64(0))
	assert.That(t, undoCount(t, db, xid)).Equal(int64(0))
}

func TestAT_DeleteRollbackReinsertsRow(t *testing.T) {
	ctx := context.Background()
	coord := at.NewCoordinator(at.WithGlobalLock(&at.MemoryGlobalLock{}))
	db := newTestDB(t, coord, &at.MemoryGlobalLock{})

	assert.Error(t, db.Create(&account{ID: 3, Balance: 250}).Error).Nil()

	gctx, xid := coord.Begin(ctx)
	err := db.WithContext(gctx).Transaction(func(tx *gorm.DB) error {
		return tx.Where("id = ?", 3).Delete(&account{}).Error
	})
	assert.Error(t, err).Nil()

	var n int64
	assert.Error(t, db.Model(&account{}).Where("id = ?", 3).Count(&n).Error).Nil()
	assert.That(t, n).Equal(int64(0))

	// Rollback re-inserts the deleted row from its before-image.
	assert.Error(t, coord.Rollback(ctx, xid)).Nil()
	var acc account
	assert.Error(t, db.First(&acc, 3).Error).Nil()
	assert.That(t, acc.Balance).Equal(250)
	assert.That(t, undoCount(t, db, xid)).Equal(int64(0))
}

func TestAT_WriteWriteConflictIsRejected(t *testing.T) {
	ctx := context.Background()
	lock := &at.MemoryGlobalLock{}
	coord := at.NewCoordinator(at.WithGlobalLock(lock))
	db := newTestDB(t, coord, lock)

	assert.Error(t, db.Create(&account{ID: 1, Balance: 100}).Error).Nil()

	// First global transaction locks row 1 and stays open.
	gctx1, _ := coord.Begin(ctx)
	err := db.WithContext(gctx1).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).Update("balance", 40).Error
	})
	assert.Error(t, err).Nil()

	// A second global transaction touching the same row is rejected by the global
	// lock, so its local transaction fails and stages nothing.
	gctx2, _ := coord.Begin(ctx)
	err = db.WithContext(gctx2).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).Update("balance", 10).Error
	})
	assert.Error(t, err).Is(at.ErrLockConflict)
}
