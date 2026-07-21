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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"go-spring.org/log"
	"go-spring.org/spring/cloud/transaction/at"
	"go-spring.org/spring/gs"
	atgorm "go-spring.org/starter-transaction-at-gorm"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ----------------------------------------------------------------------------
// Two toy "databases" backed by separate sqlite handles: an account balance and
// a stock quantity. A single global transaction debits the account in one
// database and decrements stock in the other. Neither table's mutation is
// business-visible undo code — the AT gorm plugin captures a before-image around
// each DML statement and the coordinator restores it automatically on rollback.
// This is the AT selling point over Saga/TCC: no hand-written compensation.
// ----------------------------------------------------------------------------

// account is a row in the account database.
type account struct {
	ID      int64 `gorm:"primaryKey;column:id"`
	Balance int   `gorm:"column:balance"`
}

func (account) TableName() string { return "account" }

// stock is a row in the stock database.
type stock struct {
	ID    int64 `gorm:"primaryKey;column:id"`
	Count int   `gorm:"column:count"`
}

func (stock) TableName() string { return "stock" }

// BankService owns the two databases and the global transaction coordinator the
// starter contributes. The coordinator and global lock are autowired; the two
// databases are opened and enrolled in AT in the constructor.
type BankService struct {
	coord     at.Coordinator
	accountDB *gorm.DB
	stockDB   *gorm.DB
}

// newBankService opens the two in-memory databases, migrates the business and
// undo-log tables, installs the AT plugin on each (with a distinct resource id),
// and seeds one row per table. coord and lock are the beans the starter provides.
func newBankService(coord at.Coordinator, lock at.GlobalLock) (*BankService, error) {
	accountDB, err := openDB("account-db")
	if err != nil {
		return nil, err
	}
	stockDB, err := openDB("stock-db")
	if err != nil {
		return nil, err
	}
	if err := accountDB.AutoMigrate(&account{}); err != nil {
		return nil, err
	}
	if err := stockDB.AutoMigrate(&stock{}); err != nil {
		return nil, err
	}
	if err := atgorm.Migrate(accountDB); err != nil {
		return nil, err
	}
	if err := atgorm.Migrate(stockDB); err != nil {
		return nil, err
	}
	if err := accountDB.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil {
		return nil, err
	}
	if err := stockDB.Use(atgorm.NewPlugin("stock-db", coord, lock)); err != nil {
		return nil, err
	}
	if err := accountDB.Create(&account{ID: 1, Balance: 100}).Error; err != nil {
		return nil, err
	}
	if err := stockDB.Create(&stock{ID: 1, Count: 10}).Error; err != nil {
		return nil, err
	}
	return &BankService{coord: coord, accountDB: accountDB, stockDB: stockDB}, nil
}

// openDB opens a private shared-cache in-memory sqlite database. A distinct cache
// name per handle keeps the two "databases" isolated; MaxOpenConns=1 keeps each
// on a single connection so the in-memory data is not duplicated per connection.
func openDB(name string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	return db, nil
}

// purchase runs one global transaction: debit `cost` from the account and
// decrement `qty` from stock. When failStock is true the stock step returns an
// error, which fails the global transaction and drives an automatic rollback of
// both databases from their before-images. Each database write runs in its own
// local transaction so its undo log commits atomically with the business change.
func (s *BankService) purchase(ctx context.Context, cost, qty int, failStock bool) error {
	ctx, xid := s.coord.Begin(ctx)

	err := s.accountDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).
			Update("balance", gorm.Expr("balance - ?", cost)).Error
	})
	if err == nil {
		err = s.stockDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if failStock {
				return errors.New("stock service unavailable")
			}
			return tx.Model(&stock{}).Where("id = ?", 1).
				Update("count", gorm.Expr("count - ?", qty)).Error
		})
	}

	if err != nil {
		if rbErr := s.coord.Rollback(context.Background(), xid); rbErr != nil {
			return fmt.Errorf("business error %w; rollback error %v", err, rbErr)
		}
		return err
	}
	return s.coord.Commit(context.Background(), xid)
}

func (s *BankService) balance() int {
	var a account
	_ = s.accountDB.First(&a, 1).Error
	return a.Balance
}

func (s *BankService) stockCount() int {
	var st stock
	_ = s.stockDB.First(&st, 1).Error
	return st.Count
}

// undoRows counts undo-log rows across both databases; zero means every branch's
// second-phase (commit or rollback) has cleaned up.
func (s *BankService) undoRows() int64 {
	var a, b int64
	s.accountDB.Table("at_undo_log").Count(&a)
	s.stockDB.Table("at_undo_log").Count(&b)
	return a + b
}

func main() {
	svcBean := gs.Provide(newBankService).Export(gs.As[gs.Rooter]())

	go func() {
		time.Sleep(time.Millisecond * 500)
		runTest(svcBean.Interface().(*BankService))
	}()

	gs.Run()
}

// runTest exercises the commit path, the rollback path and the write-write
// isolation guarantee, asserting the databases end where the AT protocol
// requires. Any deviation exits non-zero so check.sh fails.
func runTest(s *BankService) {
	ctx := context.Background()

	// Path 1 — commit: debit 30 and decrement 2. Both writes succeed, the global
	// transaction commits, both changes stick and every undo log is dropped.
	if err := s.purchase(ctx, 30, 2, false); err != nil {
		log.Errorf(ctx, log.TagAppDef, "commit path: unexpected error %v", err)
		os.Exit(1)
	}
	if b, c, u := s.balance(), s.stockCount(), s.undoRows(); b != 70 || c != 8 || u != 0 {
		log.Errorf(ctx, log.TagAppDef, "commit path: balance=%d stock=%d undo=%d, want 70/8/0", b, c, u)
		os.Exit(1)
	}
	fmt.Println("commit path OK: balance=70 stock=8 undo=0")

	// Path 2 — rollback: the account is debited but the stock step fails, so the
	// coordinator restores both databases from their before-images. The values are
	// exactly as they were after path 1 and no undo log is left behind.
	err := s.purchase(ctx, 50, 5, true)
	if err == nil {
		log.Errorf(ctx, log.TagAppDef, "rollback path: expected an error, got nil")
		os.Exit(1)
	}
	if b, c, u := s.balance(), s.stockCount(), s.undoRows(); b != 70 || c != 8 || u != 0 {
		log.Errorf(ctx, log.TagAppDef, "rollback path: balance=%d stock=%d undo=%d, want 70/8/0 (restored)", b, c, u)
		os.Exit(1)
	}
	fmt.Println("rollback path OK: balance=70 stock=8 undo=0 -", err.Error())

	// Path 3 — write-write isolation: one open global transaction holds the row
	// lock on account #1; a second global transaction touching the same row is
	// rejected with ErrLockConflict, so it stages nothing.
	gctx, xid := s.coord.Begin(ctx)
	if err := s.accountDB.WithContext(gctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).
			Update("balance", gorm.Expr("balance - ?", 1)).Error
	}); err != nil {
		log.Errorf(ctx, log.TagAppDef, "isolation path: first tx failed %v", err)
		os.Exit(1)
	}
	gctx2, xid2 := s.coord.Begin(ctx)
	err = s.accountDB.WithContext(gctx2).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&account{}).Where("id = ?", 1).
			Update("balance", gorm.Expr("balance - ?", 1)).Error
	})
	if !errors.Is(err, at.ErrLockConflict) {
		log.Errorf(ctx, log.TagAppDef, "isolation path: want ErrLockConflict, got %v", err)
		os.Exit(1)
	}
	_ = s.coord.Rollback(context.Background(), xid2)
	_ = s.coord.Rollback(context.Background(), xid)
	fmt.Println("isolation path OK: second global transaction rejected with ErrLockConflict")

	syscall.Kill(os.Getpid(), syscall.SIGTERM)
}

// ----------------------------------------------------------------------------
// Change working directory
// ----------------------------------------------------------------------------

// init sets the working directory of the application to the directory where this
// source file resides, so relative config paths resolve regardless of the launch
// path.
func init() {
	var execDir string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		execDir = filepath.Dir(filename)
	}
	if err := os.Chdir(execDir); err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(workDir)
}
