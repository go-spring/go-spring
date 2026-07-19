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

// Package migration is a framework-agnostic, zero-dependency abstraction for
// database schema versioning — the Go-idiomatic equivalent of Flyway /
// Liquibase, achieved with plain Go rather than an XML/DSL engine.
//
// It answers one question at startup: "which versioned changes has this
// database not seen yet, and how do I apply them exactly once, in order?". A
// [Runner] compares the ordered set of [Migration]s a [Source] yields against
// the applied rows a [Store] keeps in a version table (schema_migrations), and
// applies each pending migration in ascending version order, one transaction
// apiece where the backend supports transactional DDL.
//
// The design is forward-only and fail-stop, matching Flyway's community
// edition: there is no automatic down-migration orchestration. [Migration.Down]
// is reserved for a future explicit-rollback feature and the Runner never calls
// it. Two safety rails guard history:
//
//   - Checksum drift: if a migration already recorded as applied has a
//     different checksum than the one the Source now yields, someone edited a
//     historical script — the Runner fails fast rather than silently diverging.
//   - Out-of-order inserts: by default a pending migration whose version is
//     below the highest already-applied version is rejected (a gap was filled
//     after later versions ran). [Options.AllowOutOfOrder] relaxes this.
//
// The [Store] seam keeps the core free of any driver or ORM: a backend (e.g.
// gorm, via starter-migration-gorm) implements Store to create the version
// table, read applied records, and apply a migration atomically. The [Source]
// seam has two built-ins: [NewFSSource] over an fs.FS (go:embed a directory of
// .sql files) and [NewSource] over migrations registered in code.
package migration

import (
	"context"
	"time"
)

// Execer is the narrow seam a [Migration]'s Up uses to run SQL, decoupling
// migration definitions from any concrete driver or ORM. A [Store] passes an
// Execer bound to the migration's transaction.
type Execer interface {
	// ExecContext runs a single statement (or a driver-supported batch) and
	// discards any result set — migrations issue DDL/DML, not queries.
	ExecContext(ctx context.Context, query string, args ...any) error
}

// Migration is a single, versioned change unit. Versions are compared
// numerically, so 2 precedes 10; there is no lexical padding requirement.
type Migration struct {
	// Version is the strictly positive, unique ordering key. Two migrations
	// sharing a version is a configuration error the Runner rejects.
	Version uint64

	// Name is a human-readable label recorded in the version table. It carries
	// no semantics and may repeat across versions.
	Name string

	// Checksum is a stable hash of the migration's content. A [Source] sets it
	// so the Runner can detect a post-application edit to a historical script.
	// A code-registered migration may leave it empty to opt out of the check.
	Checksum string

	// Up applies the change. The Runner guarantees it runs at most once per
	// version across all processes sharing the version table (given a backend
	// whose Apply is atomic).
	Up func(ctx context.Context, exec Execer) error

	// Down is reserved for a future explicit-rollback feature; the Runner never
	// calls it. The framework is forward-only and fail-stop by design.
	Down func(ctx context.Context, exec Execer) error
}

// Record is one row of the version table: a migration already applied.
type Record struct {
	Version   uint64
	Name      string
	Checksum  string
	AppliedAt time.Time
}

// Source yields the ordered set of migrations to consider. The Runner sorts and
// validates whatever a Source returns, so a Source need not pre-sort. Two are
// built in: [NewFSSource] (filesystem / go:embed) and [NewSource] (code).
type Source interface {
	Migrations() ([]Migration, error)
}

// Store is the backend seam. A backend implements it to persist and read the
// version table and to apply a migration. Keeping this an interface (rather
// than a string-keyed driver registry) is deliberate: a store needs a live
// database handle, not a declarative policy — the same reasoning behind
// [go-spring.org/stdlib/lock]'s Locker seam.
type Store interface {
	// EnsureVersionTable creates the version table if it does not exist. It is
	// idempotent and safe to call on every startup.
	EnsureVersionTable(ctx context.Context) error

	// AppliedRecords returns every migration already recorded as applied. Order
	// is unspecified; the Runner indexes them by version.
	AppliedRecords(ctx context.Context) ([]Record, error)

	// Apply runs m.Up and records the version row. When the backend supports
	// transactional DDL both happen in one transaction, so a failed Up leaves no
	// half-applied row; on backends where DDL auto-commits (e.g. MySQL) the row
	// is still written only after Up succeeds.
	Apply(ctx context.Context, m Migration) error

	// MarkApplied records a migration as applied without running its Up. The
	// Runner uses it for baseline versions (adopting a database that already has
	// the schema).
	MarkApplied(ctx context.Context, m Migration) error
}
