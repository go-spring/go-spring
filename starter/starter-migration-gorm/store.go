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

package StarterMigrationGorm

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go-spring.org/spring/data/migration"
	"gorm.io/gorm"
)

// identRe guards the version-table name against SQL injection: the table name
// is interpolated into DDL/DML (it cannot be a bound parameter), so it must be a
// plain identifier.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// gormStore adapts a *gorm.DB to the [migration.Store] seam. It keeps the
// version table and applies migrations; the actual per-migration SQL runs
// through a [migration.Execer] bound to the transaction.
type gormStore struct {
	db    *gorm.DB
	table string
}

// NewStore adapts a *gorm.DB to the [migration.Store] seam, for callers that
// want to run migrations programmatically rather than through the startup
// Runner (for example a smoke test or a one-off admin command).
func NewStore(db *gorm.DB, table string) (migration.Store, error) {
	return newGormStore(db, table)
}

// newGormStore validates the table name and returns a Store over db.
func newGormStore(db *gorm.DB, table string) (*gormStore, error) {
	if table == "" {
		table = "schema_migrations"
	}
	if !identRe.MatchString(table) {
		return nil, fmt.Errorf("invalid version-table name %q (must be a plain SQL identifier)", table)
	}
	return &gormStore{db: db, table: table}, nil
}

// EnsureVersionTable creates the version table if absent. The column types are
// chosen to be portable across MySQL, PostgreSQL and SQLite.
func (s *gormStore) EnsureVersionTable(ctx context.Context) error {
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	version BIGINT PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	checksum VARCHAR(128) NOT NULL,
	applied_at TIMESTAMP NOT NULL
)`, s.table)
	return s.db.WithContext(ctx).Exec(ddl).Error
}

// verRow maps the version-table columns for scanning.
type verRow struct {
	Version   uint64    `gorm:"column:version"`
	Name      string    `gorm:"column:name"`
	Checksum  string    `gorm:"column:checksum"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

// AppliedRecords reads every recorded migration.
func (s *gormStore) AppliedRecords(ctx context.Context) ([]migration.Record, error) {
	var rows []verRow
	q := fmt.Sprintf("SELECT version, name, checksum, applied_at FROM %s ORDER BY version", s.table)
	if err := s.db.WithContext(ctx).Raw(q).Scan(&rows).Error; err != nil {
		return nil, err
	}
	recs := make([]migration.Record, len(rows))
	for i, r := range rows {
		recs[i] = migration.Record{Version: r.Version, Name: r.Name, Checksum: r.Checksum, AppliedAt: r.AppliedAt}
	}
	return recs, nil
}

// Apply runs the migration's Up and records the version row in one transaction.
// On PostgreSQL and SQLite the DDL is transactional, so a failed Up rolls back
// cleanly; on MySQL each DDL statement auto-commits (a MySQL limitation Flyway
// shares), but the version row is still written only after Up succeeds, so a
// crash mid-migration leaves the row absent and the next run retries.
func (s *gormStore) Apply(ctx context.Context, m migration.Migration) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if m.Up != nil {
			if err := m.Up(ctx, &gormExecer{tx: tx}); err != nil {
				return err
			}
		}
		return s.insertRecord(ctx, tx, m)
	})
}

// MarkApplied records a baseline migration without running its Up.
func (s *gormStore) MarkApplied(ctx context.Context, m migration.Migration) error {
	return s.insertRecord(ctx, s.db, m)
}

// insertRecord writes one version row using db (which may be a transaction).
func (s *gormStore) insertRecord(ctx context.Context, db *gorm.DB, m migration.Migration) error {
	q := fmt.Sprintf("INSERT INTO %s (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)", s.table)
	return db.WithContext(ctx).Exec(q, m.Version, m.Name, m.Checksum, time.Now().UTC()).Error
}

// gormExecer is the [migration.Execer] a migration's Up uses; it runs each
// statement on the migration's transaction.
type gormExecer struct{ tx *gorm.DB }

func (e *gormExecer) ExecContext(ctx context.Context, query string, args ...any) error {
	return e.tx.WithContext(ctx).Exec(query, args...).Error
}
