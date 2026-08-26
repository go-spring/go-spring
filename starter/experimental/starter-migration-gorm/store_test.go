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

package StarterMigrationGorm_test

import (
	"context"
	"embed"
	"testing"

	migration "go-spring.org/cloud/data/experimental/data/migration"
	StarterMigrationGorm "go-spring.org/starter-migration-gorm"
	"go-spring.org/stdlib/testing/assert"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed testdata/migrations
var migrationsFS embed.FS

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.That(t, err).Nil()
	sqlDB, err := db.DB()
	assert.That(t, err).Nil()
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func up(stmt string) func(context.Context, migration.Execer) error {
	return func(ctx context.Context, ex migration.Execer) error {
		return ex.ExecContext(ctx, stmt)
	}
}

// TestStore_MigrationChainWithSQLite runs a real chain: version table creation,
// two DDL migrations, idempotent second run, and rows the migrations created.
func TestStore_MigrationChainWithSQLite(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	store, err := StarterMigrationGorm.NewStore(db, "schema_migrations")
	assert.That(t, err).Nil()

	src := migration.NewSource(
		migration.Migration{Version: 1, Name: "init", Checksum: "c1", Up: up("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")},
		migration.Migration{Version: 2, Name: "seed", Checksum: "c2", Up: up("INSERT INTO users (name) VALUES ('alice')")},
	)

	done, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(2)

	// The migrated schema is real and queryable.
	var n int64
	assert.That(t, db.Raw("SELECT COUNT(*) FROM users").Scan(&n).Error).Nil()
	assert.That(t, n).Equal(int64(1))

	// Second run is a no-op: two applied records, nothing new executed.
	done, err = migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(0)

	recs, err := store.AppliedRecords(ctx)
	assert.That(t, err).Nil()
	assert.That(t, len(recs)).Equal(2)
	assert.That(t, recs[0].Version).Equal(uint64(1))
	assert.String(t, recs[0].Name).Equal("init")
}

// TestStore_FailedUpRollsBackRow verifies the transactional Apply contract: a
// failed Up leaves no version row, so the next run retries the migration.
func TestStore_FailedUpRollsBackRow(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	store, err := StarterMigrationGorm.NewStore(db, "")
	assert.That(t, err).Nil()

	bad := migration.NewSource(migration.Migration{
		Version: 1, Name: "bad", Checksum: "c1",
		Up: func(ctx context.Context, ex migration.Execer) error {
			if err := ex.ExecContext(ctx, "CREATE TABLE t1 (id INTEGER PRIMARY KEY)"); err != nil {
				return err
			}
			// Fails: no such table. The whole transaction (incl. the version
			// row) must roll back on SQLite.
			return ex.ExecContext(ctx, "INSERT INTO no_such_table VALUES (1)")
		},
	})
	_, err = migration.NewRunner(store, bad, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Matches("apply version 1")

	recs, err := store.AppliedRecords(ctx)
	assert.That(t, err).Nil()
	assert.That(t, len(recs)).Equal(0, "no version row after a failed Up")

	// And a retry with a fixed script succeeds from a clean slate.
	good := migration.NewSource(migration.Migration{
		Version: 1, Name: "bad", Checksum: "c1", Up: up("CREATE TABLE t1 (id INTEGER PRIMARY KEY)"),
	})
	done, err := migration.NewRunner(store, good, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(1)
}

// TestStore_ChecksumDriftDetected verifies editing an already-applied migration
// file is caught against the real version table.
func TestStore_ChecksumDriftDetected(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	store, _ := StarterMigrationGorm.NewStore(db, "schema_migrations")

	orig := migration.NewSource(migration.Migration{Version: 1, Name: "a", Checksum: "orig", Up: up("CREATE TABLE t (id INTEGER PRIMARY KEY)")})
	_, err := migration.NewRunner(store, orig, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()

	edited := migration.NewSource(migration.Migration{Version: 1, Name: "a", Checksum: "edited", Up: up("CREATE TABLE t (id INTEGER PRIMARY KEY)")})
	_, err = migration.NewRunner(store, edited, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Matches("checksum mismatch for version 1")
}

// TestStore_OutOfOrderRejected verifies the gap-fill guard against the real
// version table: a late version 2 below the applied version 3 fails fast.
func TestStore_OutOfOrderRejected(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	store, _ := StarterMigrationGorm.NewStore(db, "schema_migrations")

	first := migration.NewSource(
		migration.Migration{Version: 1, Name: "a", Checksum: "c1", Up: up("CREATE TABLE a (id INTEGER PRIMARY KEY)")},
		migration.Migration{Version: 3, Name: "c", Checksum: "c3", Up: up("CREATE TABLE c (id INTEGER PRIMARY KEY)")},
	)
	_, err := migration.NewRunner(store, first, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()

	gap := migration.NewSource(
		migration.Migration{Version: 1, Name: "a", Checksum: "c1", Up: up("CREATE TABLE a (id INTEGER PRIMARY KEY)")},
		migration.Migration{Version: 2, Name: "b", Checksum: "c2", Up: up("CREATE TABLE b (id INTEGER PRIMARY KEY)")},
		migration.Migration{Version: 3, Name: "c", Checksum: "c3", Up: up("CREATE TABLE c (id INTEGER PRIMARY KEY)")},
	)
	_, err = migration.NewRunner(store, gap, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Matches("out-of-order migration")

	// Opting in applies the gap fill.
	done, err := migration.NewRunner(store, gap, migration.Options{AllowOutOfOrder: true}).Migrate(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(1)
	assert.That(t, done[0].Version).Equal(uint64(2))
}

// TestStore_EmbeddedFSSourceChain runs the full go:embed path: V1__init.sql and
// V2__add_users_email.sql from testdata/migrations apply against SQLite and the
// second run is a no-op.
func TestStore_EmbeddedFSSourceChain(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	store, _ := StarterMigrationGorm.NewStore(db, "schema_migrations")
	src := migration.NewFSSource(migrationsFS, "testdata/migrations")

	done, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(2)
	assert.That(t, done[0].Version).Equal(uint64(1))
	assert.That(t, done[1].Version).Equal(uint64(2))
	assert.String(t, done[1].Name).Equal("add users email")

	// V2's ALTER TABLE really took effect.
	var n int64
	assert.That(t, db.Raw("SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'email'").Scan(&n).Error).Nil()
	assert.That(t, n).Equal(int64(1))

	// Idempotent second run.
	done, err = migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(done)).Equal(0)
}

// TestNewStore_RejectsBadTableName verifies the injection guard on the version
// table name.
func TestNewStore_RejectsBadTableName(t *testing.T) {
	db := newDB(t)
	_, err := StarterMigrationGorm.NewStore(db, "t; DROP TABLE x")
	assert.Error(t, err).Matches("invalid version-table name")
}
