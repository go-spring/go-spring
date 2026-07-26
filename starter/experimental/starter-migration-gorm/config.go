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

// Config binds a single ${spring.migration.<name>} entry — one database's
// migration plan. The prefix is per capability (schema migration), so a second
// backend implementation would reuse it; multiple databases are multiple map
// entries, not multiple prefixes.
type Config struct {
	// Enabled turns this entry on. It defaults to true so a configured entry
	// runs by default; set false to keep the config but skip the run.
	Enabled bool `value:"${enabled:=true}"`

	// DBRef names the *gorm.DB bean to migrate. When empty and exactly one
	// *gorm.DB bean exists, that one is used; when empty and several exist it is
	// a fail-fast error (naming avoids migrating the wrong database).
	DBRef string `value:"${db-ref:=}"`

	// SourceDir points at an on-disk directory of V<version>__<name>.sql files.
	// It is the fallback when no migration.Source bean named after this entry is
	// registered; the go:embed path (a Source bean) is preferred for a single
	// self-contained binary.
	SourceDir string `value:"${source-dir:=}"`

	// Baseline records every migration with version <= Baseline as applied
	// without running it — for adopting migrations on a database that already
	// carries schema. 0 disables.
	Baseline uint64 `value:"${baseline:=0}"`

	// AllowOutOfOrder permits applying a migration whose version is below the
	// highest already-applied version (a gap fill). Default false.
	AllowOutOfOrder bool `value:"${allow-out-of-order:=false}"`

	// Table is the version-table name. It must be a plain SQL identifier
	// (letters, digits, underscore; not starting with a digit).
	Table string `value:"${table:=schema_migrations}"`
}
