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
	"time"

	"go-spring.org/cloud/experimental/transaction/at"
	"go-spring.org/stdlib/testing/assert"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newRecoveryTestDB opens a fresh in-memory sqlite database with the undo-log and
// business tables migrated but NO plugin installed — recovery must work against
// whatever survived the "crash", not against a live AT session.
func newRecoveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.Error(t, err).Nil()
	sqlDB, err := db.DB()
	assert.Error(t, err).Nil()
	sqlDB.SetMaxOpenConns(1)
	assert.Error(t, Migrate(db)).Nil()
	assert.Error(t, db.AutoMigrate(&account{})).Nil()
	return db
}

// crashMidTransaction stages the classic orphan scenario: a global transaction
// updates, inserts and deletes rows (phase one commits business data plus undo
// logs) and then "crashes" — the coordinator is simply dropped without a phase
// two. It returns the xid whose undo logs are now orphaned.
func crashMidTransaction(t *testing.T, db *gorm.DB) string {
	t.Helper()
	coord := at.NewCoordinator()
	assert.Error(t, db.Create(&account{ID: 1, Balance: 100}).Error).Nil()
	assert.Error(t, db.Create(&account{ID: 2, Balance: 50}).Error).Nil()
	pluginDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	assert.Error(t, err).Nil()
	sqlDB, _ := pluginDB.DB()
	sqlDB.SetMaxOpenConns(1)
	err = pluginDB.Use(NewPlugin("acct", coord, &at.MemoryGlobalLock{}))
	assert.Error(t, err).Nil()

	gctx, xid := coord.Begin(context.Background())
	assert.Error(t, pluginDB.WithContext(gctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&account{}).Where("id = ?", 1).Update("balance", 40).Error; err != nil {
			return err
		}
		if err := tx.Create(&account{ID: 7, Balance: 500}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", 2).Delete(&account{}).Error
	})).Nil()
	return xid
}

func TestATRecovery_ReplaysOrphanedUndoLogs(t *testing.T) {
	db := newRecoveryTestDB(t)
	xid := crashMidTransaction(t, db)

	// The crashed state: business writes visible, undo logs orphaned.
	var total int64
	assert.Error(t, db.Model(&account{}).Count(&total).Error).Nil()
	assert.That(t, total).Equal(int64(2))
	assert.That(t, undoCount(t, db, xid)).Equal(int64(3))

	// Boot recovery rolls the whole branch back from its before-images.
	assert.Error(t, recoverDatabase(context.Background(), "acct", db)).Nil()

	var acc account
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(100) // update undone
	var n int64
	assert.Error(t, db.Model(&account{}).Where("id = ?", 7).Count(&n).Error).Nil()
	assert.That(t, n).Equal(int64(0)) // insert undone
	assert.Error(t, db.Model(&account{}).Where("id = ?", 2).Count(&n).Error).Nil()
	assert.That(t, n).Equal(int64(1))                     // delete re-inserted
	assert.That(t, undoCount(t, db, xid)).Equal(int64(0)) // logs dropped
}

func TestATRecovery_NoOrphansIsNoOp(t *testing.T) {
	db := newRecoveryTestDB(t)
	assert.Error(t, db.Create(&account{ID: 1, Balance: 100}).Error).Nil()

	assert.Error(t, recoverDatabase(context.Background(), "acct", db)).Nil()
	var acc account
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(100)
}

func TestATRecovery_MultipleXidsRecoveredIndependently(t *testing.T) {
	db := newRecoveryTestDB(t)
	xid1 := crashMidTransaction(t, db)

	// A second orphaned xid: an update staged manually (as a crash between the
	// phases would have left it), with a hand-written undo row.
	assert.Error(t, db.Model(&account{}).Where("id = ?", 1).Update("balance", 60).Error).Nil()
	ctxJSON, err := encodeContext(
		at.RecordImage{Table: "account", Rows: []at.RowImage{{
			PrimaryKeys: map[string]any{"id": int64(1)},
			Values:      map[string]any{"id": int64(1), "balance": int64(100)},
		}}},
		at.RecordImage{},
	)
	assert.Error(t, err).Nil()
	assert.Error(t, db.Create(&undoRow{
		XID: "deadbeef", BranchID: "acct", Table: "account",
		SQLType: int(at.SQLUpdate), Context: ctxJSON, CreatedAt: time.Now(),
	}).Error).Nil()

	assert.Error(t, recoverDatabase(context.Background(), "acct", db)).Nil()

	var acc account
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(100) // both xids' updates undone
	assert.That(t, undoCount(t, db, xid1)).Equal(int64(0))
	assert.That(t, undoCount(t, db, "deadbeef")).Equal(int64(0))
}

func TestATRecovery_RunnerScansRegisteredDatabases(t *testing.T) {
	db := newRecoveryTestDB(t)
	xid := crashMidTransaction(t, db)

	recoverMu.Lock()
	saved := recoverDBs
	recoverDBs = map[string]*gorm.DB{"acct": db}
	recoverMu.Unlock()
	t.Cleanup(func() {
		recoverMu.Lock()
		recoverDBs = saved
		recoverMu.Unlock()
	})

	r := newRecoveryRunner()
	assert.Error(t, r.Run(context.Background())).Nil()
	assert.That(t, undoCount(t, db, xid)).Equal(int64(0))

	var acc account
	assert.Error(t, db.First(&acc, 1).Error).Nil()
	assert.That(t, acc.Balance).Equal(100)
}

func TestATRecovery_PluginRegistersItsDatabase(t *testing.T) {
	coord := at.NewCoordinator()
	newTestDB(t, coord, &at.MemoryGlobalLock{})

	recoverMu.Lock()
	_, ok := recoverDBs["acct"]
	recoverMu.Unlock()
	assert.That(t, ok).True()
}
