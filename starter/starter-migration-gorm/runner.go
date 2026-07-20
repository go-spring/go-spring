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
	"os"
	"sort"

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/spring/migration"
	"gorm.io/gorm"
)

// Runner applies the configured migrations at startup. It is a [gs.Runner]:
// the container invokes Run after every bean is wired but before any server
// starts, so a repository or DAO never queries a table a migration has not yet
// created. Its exported fields are populated by the IoC container.
type Runner struct {
	// Entries binds ${spring.migration} — one map entry per database. The map
	// key is the instance name that also matches a migration.Source bean.
	Entries map[string]Config `value:"${spring.migration}"`

	// DBs collects every *gorm.DB bean keyed by bean name, so an entry's db-ref
	// selects one. Optional so the missing case is a clear message, not a wiring
	// failure.
	DBs map[string]*gorm.DB `autowire:"?"`

	// Sources collects every migration.Source bean keyed by bean name, so an
	// entry named "app" is served by a Source bean named "app" (the go:embed
	// path). When absent for an entry, source-dir is used instead.
	Sources map[string]migration.Source `autowire:"?"`
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
		e := r.Entries[name]
		if !e.Enabled {
			log.Infof(ctx, log.TagAppDef, "migration: entry %q disabled, skipping", name)
			continue
		}
		db, err := r.pickDB(name, e)
		if err != nil {
			return err
		}
		src, err := r.pickSource(name, e)
		if err != nil {
			return err
		}
		store, err := newGormStore(db, e.Table)
		if err != nil {
			return errutil.Explain(err, "migration: entry %q", name)
		}
		opts := migration.Options{AllowOutOfOrder: e.AllowOutOfOrder, Baseline: e.Baseline}
		applied, err := migration.NewRunner(store, src, opts).Migrate(ctx)
		if err != nil {
			return errutil.Explain(err, "migration: entry %q", name)
		}
		log.Infof(ctx, log.TagAppDef, "migration: entry %q applied %d migration(s)", name, len(applied))
	}
	return nil
}

// pickDB resolves the *gorm.DB for an entry: by db-ref when set, else the sole
// bean when exactly one exists. Anything else is a fail-fast error.
func (r *Runner) pickDB(name string, e Config) (*gorm.DB, error) {
	if e.DBRef != "" {
		db, ok := r.DBs[e.DBRef]
		if !ok {
			return nil, errutil.Explain(nil,
				"migration: entry %q references db-ref %q but no *gorm.DB bean of that name is registered", name, e.DBRef)
		}
		return db, nil
	}
	switch len(r.DBs) {
	case 1:
		for _, db := range r.DBs {
			return db, nil
		}
	case 0:
		return nil, errutil.Explain(nil,
			"migration: entry %q has no db-ref and no *gorm.DB bean is registered", name)
	}
	return nil, errutil.Explain(nil,
		"migration: entry %q has no db-ref but %d *gorm.DB beans exist; set db-ref to disambiguate", name, len(r.DBs))
}

// pickSource resolves the migration source for an entry: a migration.Source
// bean named after the entry (go:embed) takes precedence; otherwise source-dir
// is read from disk. Neither present is a fail-fast error.
func (r *Runner) pickSource(name string, e Config) (migration.Source, error) {
	if src, ok := r.Sources[name]; ok {
		return src, nil
	}
	if e.SourceDir != "" {
		return migration.NewFSSource(os.DirFS(e.SourceDir), "."), nil
	}
	return nil, errutil.Explain(nil,
		"migration: entry %q has neither a migration.Source bean named %q nor a source-dir", name, name)
}
