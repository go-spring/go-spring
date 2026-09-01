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

// Package StarterMigrationGoose runs [goose] schema migrations at startup over
// any gorm *gorm.DB. Goose owns the algorithm (versioning, checksums,
// transactions, dialect quirks); this starter only owns the wiring: config keys,
// the IoC lifecycle and the gorm-been -> *sql.DB bridge.
//
// It is enabled only when configured:
//
//	import _ "go-spring.org/starter-migration-goose"
//
//	spring.migration.app.db-ref=app        # name of the *gorm.DB bean
//	spring.migration.app.dir=./sql         # V<version>__<name>.sql files
//
// A [gs.Runner] executes after every bean is wired but before any server
// serves, so tables exist before the first request. A migration failure
// aborts startup (fail-fast) — a broken schema never serves traffic.
// Multiple databases are multiple entries under spring.migration.
//
// [goose]: https://github.com/pressly/goose
package StarterMigrationGoose

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Activated only when spring.migration.* is configured. The Runner binds
	// the per-database entries, collects every *gorm.DB bean by name, and
	// applies the pending goose migrations in Run. Exported as gs.Runner so
	// the container makes it a startup root and invokes it before servers.
	gs.Provide(&Runner{}).
		Name("migrationRunner").
		Condition(gs.OnProperty("spring.migration")).
		Export(gs.As[gs.Runner]())
}
