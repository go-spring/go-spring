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

// Config binds a single ${spring.migration.<name>} entry — one database's
// migration plan. Multiple databases are multiple map entries, not multiple
// prefixes.
type Config struct {
	// Enabled turns this entry on. It defaults to true so a configured entry
	// runs by default; set false to keep the config but skip the run.
	Enabled bool `value:"${enabled:=true}"`

	// DBRef names the *gorm.DB bean to migrate. When empty and exactly one
	// *gorm.DB bean exists, that one is used; when empty and several exist it is
	// a fail-fast error (naming avoids migrating the wrong database).
	DBRef string `value:"${db-ref:=}"`

	// Dir points at a directory of goose SQL migrations
	// (V<version>__<name>.sql, forward-only via goose.Up). Use an absolute or
	// working-directory-relative path; for a self-contained binary prefer
	// copying the embedded migrations to a temp dir and pointing Dir there.
	Dir string `value:"${dir:=}"`

	// Table is goose's version-table name.
	Table string `value:"${table:=goose_db_version}"`

	// AllowMissing permits applying a migration whose version is below the
	// highest already-applied one (a gap fill, e.g. two branches both adding
	// migrations). Default false.
	AllowMissing bool `value:"${allow-missing:=false}"`
}
