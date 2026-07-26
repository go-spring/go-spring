# migration
[English](README.md) | [中文](README_CN.md)

`migration` is a zero-dependency abstraction for versioned database schema
migration — the Go-idiomatic equivalent of Flyway / Liquibase. A
[`Migration`](migration.go) is a numbered, named, checksummed unit of schema
change; a [`Source`](migration.go) yields the ordered set of them (from an
embedded directory or code); a [`Runner`](runner.go) applies the unapplied ones
in a version table, exactly once, forward-only.

The abstraction owns the *algorithm* (ordering, checksum guard, out-of-order
policy, baseline); the *storage* is a [`Store`](migration.go) seam a backend
implements over its own driver (see `starter-migration-gorm` for the gorm one).

## Features

- `Migration{Version, Name, Checksum, Up}` — a single migration; `Up` runs its
  statements through an [`Execer`](migration.go), so it is driver-agnostic.
- `NewFSSource(fsys, dir)` — reads Flyway-style `V<version>__<name>.sql` files
  from a `//go:embed` directory (or any `fs.FS`), SHA-256-hashing each.
- `NewSource(migs...)` — registers migrations written in Go code.
- `Runner.Migrate` — ensures the version table, then applies every unapplied
  migration in ascending version order, recording each.
- Checksum guard — editing an already-applied script is a fail-fast error, not a
  silent re-run.
- `Options{AllowOutOfOrder, Baseline}` — gap-fill policy and adopt-existing-schema
  baseline.
- Forward-only — `Down` is reserved on `Migration` but never executed.

## Usage

```go
package main

import (
    "context"
    "embed"

    "go-spring.org/spring/data/migration"
)

//go:embed migrations
var migrationsFS embed.FS

func run(ctx context.Context, store migration.Store) error {
    src := migration.NewFSSource(migrationsFS, "migrations")
    applied, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
    if err != nil {
        return err
    }
    _ = applied // the migrations applied on this run (empty on a no-op re-run)
    return nil
}
```

`store` is supplied by a backend — for gorm, `starter-migration-gorm` wraps a
`*gorm.DB` as a `migration.Store`. To register migrations in code instead of
`.sql` files:

```go
src := migration.NewSource(migration.Migration{
    Version: 1,
    Name:    "create users",
    Up: func(ctx context.Context, exec migration.Execer) error {
        return exec.ExecContext(ctx, "CREATE TABLE users (id BIGINT PRIMARY KEY)")
    },
})
```

## Semantics

- Migrations run by ascending version; version `0` and duplicate versions are
  rejected up front.
- An already-recorded version is skipped; if its recorded checksum differs from
  the source's, `Migrate` fails — migrations are immutable history.
- With `AllowOutOfOrder=false` (default) a version below the highest applied one
  is rejected; set it true to permit a gap fill.
- `Baseline=N` records every version `<= N` as applied without running it, for
  adopting a database that already carries schema.
- Forward-only, fail-stop: a failed migration aborts the run and no later
  migration is attempted.
