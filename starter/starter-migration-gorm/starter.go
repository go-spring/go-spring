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

// Package StarterMigrationGorm runs [go-spring.org/stdlib/migration] schema
// migrations at application startup, backed by a gorm *gorm.DB the application
// already registered — the Go-Spring equivalent of putting Flyway/Liquibase on
// the classpath. Blank-import it and configure one entry per database:
//
//	import _ "go-spring.org/starter-migration-gorm"
//
//	spring.migration.app.db-ref=app          # name of the *gorm.DB bean
//	spring.migration.app.source-dir=./sql    # or supply a migration.Source bean named "app"
//
// The application supplies the migrations, the starter owns the runner — the
// same "app owns the work, starter owns the runner" split used by
// starter-batch and starter-scheduler. Migrations come from either a
// migration.Source bean named after the config entry (the go:embed case) or an
// on-disk directory named by source-dir.
//
// This is a client-form-variant integration starter (see starter/DESIGN.md
// §2.2): it consumes a named *gorm.DB bean rather than opening a connection of
// its own, and it is multi-instance — bind several databases under
// spring.migration.<name>. It exports a [gs.Runner] rather than a server: a
// Runner executes after all beans are wired but before any server begins
// serving, which is exactly the ordering schema migration needs — the tables
// exist before the first request reaches a repository or DAO. A migration
// failure returns an error from Run and aborts startup (fail-fast), so a broken
// schema never serves traffic.
//
// Forward-only and fail-stop, matching Flyway community edition: there is no
// automatic down-migration. See [go-spring.org/stdlib/migration] for the
// checksum-drift and out-of-order safety rails.
package StarterMigrationGorm

import (
	"go-spring.org/spring/gs"
)

func init() {
	// Activated only when spring.migration.* is configured. The Runner binds the
	// per-database entries, collects every *gorm.DB and migration.Source bean by
	// name, and applies the pending migrations in Run. Exported as gs.Runner so
	// the container makes it a startup root and invokes it before servers start.
	gs.Provide(&Runner{}).
		Name("migrationRunner").
		Condition(gs.OnProperty("spring.migration")).
		Export(gs.As[gs.Runner]())
}
