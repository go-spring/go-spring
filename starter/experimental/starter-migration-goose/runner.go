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

package StarterMigrationGoose

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	"go-spring.org/log"
	"gorm.io/gorm"
)

// Runner applies the configured goose migrations at startup. It is a
// [gs.Runner]: the container invokes Run after every bean is wired but before
// any server starts, so a DAO never queries a table a migration has not yet
// created. Its exported fields are populated by the IoC container.
type Runner struct {
	// Entries binds ${spring.migration} — one map entry per database.
	Entries map[string]Config `value:"${spring.migration}"`

	// DBs collects every *gorm.DB bean keyed by bean name, so an entry's
	// db-ref selects one. Optional so the missing case is a clear message, not
	// a wiring failure.
	DBs map[string]*gorm.DB `autowire:"?"`
}

// Run applies each enabled entry's migrations in a stable (name-sorted) order.
// Any failure aborts startup — a database left in an unknown schema state must
// not serve traffic.
func (r *Runner) Run(ctx context.Context) error {
	names := make([]string, 0, len(r.Entries))
	for name := range r.Entries {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cfg := r.Entries[name]
		if !cfg.Enabled {
			log.Infof(ctx, log.TagAppDef, "migration %q disabled, skipping", name)
			continue
		}
		db, err := r.resolveDB(name, cfg)
		if err != nil {
			return err
		}
		if err := migrateOne(ctx, db, cfg); err != nil {
			return fmt.Errorf("migration %q: %w", name, err)
		}
	}
	return nil
}

// resolveDB picks the *gorm.DB bean for an entry: db-ref when named, the only
// bean when exactly one exists, an error otherwise.
func (r *Runner) resolveDB(name string, cfg Config) (*gorm.DB, error) {
	if cfg.DBRef != "" {
		db, ok := r.DBs[cfg.DBRef]
		if !ok {
			return nil, fmt.Errorf("no *gorm.DB bean named %q (known: %v)", cfg.DBRef, keys(r.DBs))
		}
		return db, nil
	}
	switch len(r.DBs) {
	case 1:
		for _, db := range r.DBs {
			return db, nil
		}
	case 0:
		return nil, fmt.Errorf("entry %q: no *gorm.DB bean found; open a database starter first", name)
	default:
		return nil, fmt.Errorf("entry %q: several *gorm.DB beans exist (known: %v); set db-ref", name, keys(r.DBs))
	}
	return nil, nil
}

// migrateOne bridges the gorm connection to goose and applies every pending
// migration in cfg.Dir, forward-only.
func migrateOne(ctx context.Context, db *gorm.DB, cfg Config) error {
	if cfg.Dir == "" {
		return fmt.Errorf("dir is required (directory of V<version>__<name>.sql files)")
	}
	if _, err := os.Stat(cfg.Dir); err != nil {
		return fmt.Errorf("migration dir %q: %w", cfg.Dir, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("gorm connection has no *sql.DB: %w", err)
	}
	dialect, err := gooseDialect(db.Dialector.Name())
	if err != nil {
		return err
	}

	store, err := database.NewStore(database.Dialect(dialect), cfg.Table)
	if err != nil {
		return fmt.Errorf("goose store: %w", err)
	}
	// With a custom store goose derives the dialect from the store itself, so
	// the provider-level dialect must be empty.
	provider, err := goose.NewProvider("", sqlDB, os.DirFS(cfg.Dir),
		goose.WithStore(store),
		goose.WithAllowOutofOrder(cfg.AllowMissing),
	)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	res, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	if len(res) == 0 {
		log.Infof(ctx, log.TagAppDef, "goose: no pending migrations")
		return nil
	}
	for _, m := range res {
		log.Infof(ctx, log.TagAppDef, "goose: applied %s", m.Source.Path)
	}
	return nil
}

// gooseDialect maps a gorm dialector name onto goose's dialect enum.
func gooseDialect(name string) (goose.Dialect, error) {
	switch name {
	case "mysql":
		return goose.DialectMySQL, nil
	case "postgres":
		return goose.DialectPostgres, nil
	case "sqlite":
		return goose.DialectSQLite3, nil
	case "sqlserver":
		return goose.DialectMSSQL, nil
	case "clickhouse":
		return goose.DialectClickHouse, nil
	default:
		return "", fmt.Errorf("goose has no dialect for gorm dialector %q", name)
	}
}

func keys(m map[string]*gorm.DB) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
