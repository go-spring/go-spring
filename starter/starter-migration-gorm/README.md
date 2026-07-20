# starter-migration-gorm

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-migration-gorm` runs the schema-migration capability defined in
[`go-spring.org/spring/migration`](../../spring/migration) at application
startup, backed by a [gorm](https://gorm.io) `*gorm.DB` bean the application
already registered. It is the Go-Spring equivalent of putting **Flyway** or
**Liquibase** on the classpath — versioned, checksum-guarded, forward-only
migrations applied before the first request — reached with idiomatic Go rather
than by replicating their XML/DSL machinery.

It is a **client-form-variant integration** starter (see [DESIGN.md](../DESIGN.md)
§2.2): it consumes a named `*gorm.DB` bean rather than opening a connection of
its own, and it is multi-instance — bind several databases under
`spring.migration.<name>`. It exports a `gs.Runner`, not a server: a Runner runs
after every bean is wired but before any server begins serving, which is exactly
the ordering schema migration needs — the tables exist before the first request
reaches a repository or DAO. A migration failure aborts startup (fail-fast), so a
database left in an unknown schema state never serves traffic.

## Flyway parallels

| Flyway | starter-migration-gorm |
|---|---|
| `V<version>__<name>.sql` on the classpath | `V<version>__<name>.sql` in a `//go:embed` directory or a `source-dir` |
| `flyway_schema_history` table | `schema_migrations` table (configurable via `table`) |
| checksum validation on repeat | SHA-256 checksum — editing an applied script is a fail-fast error |
| `baselineVersion` | `baseline` — records versions `<=` it as applied without running them |
| out-of-order toggle | `allow-out-of-order` (default off) |
| forward-only (community) | forward-only; `Down` is reserved but never auto-run |

## Installation

```bash
go get go-spring.org/starter-migration-gorm
```

## Quick Start

The split is **the application owns the migrations, the starter owns the
runner** — the same split used by `starter-batch` and `starter-scheduler`.

### 1. Import the starter

```go
import _ "go-spring.org/starter-migration-gorm"
```

### 2. Supply migrations

Provide a `migration.Source` bean **named after the config entry** (the
recommended `go:embed` case, so migrations ship inside the single binary):

```go
//go:embed migrations
var migrationsFS embed.FS

gs.Provide(func() migration.Source {
    return migration.NewFSSource(migrationsFS, "migrations")
}).Name("app")
```

…or skip the bean and point `source-dir` at an on-disk directory instead.

### 3. Configure one entry per database

```properties
spring.migration.app.db-ref=app        # name of the *gorm.DB bean to migrate
spring.migration.app.source-dir=./sql  # optional; used when no Source bean is named "app"
```

On startup the Runner applies every pending migration in ascending version
order, records each in `schema_migrations`, and logs how many it applied. See
[example/example.go](example/example.go) for a runnable demo that proves startup
apply, second-run idempotency and checksum-drift fail-fast.

### 4. Run migrations programmatically (optional)

For a one-off admin command or a test, wrap a `*gorm.DB` as a `migration.Store`
directly:

```go
store, _ := migrationgorm.NewStore(db, "schema_migrations")
applied, err := migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)
```

## How it works

- **Version table** — `EnsureVersionTable` creates `schema_migrations(version,
  name, checksum, applied_at)` with column types portable across MySQL,
  PostgreSQL and SQLite. The table name is validated as a plain SQL identifier
  because it is interpolated into DDL (it cannot be a bound parameter).
- **Ordering** — enabled entries run name-sorted; within an entry, migrations run
  by ascending version. Version `0` and duplicate versions are rejected.
- **Idempotency** — an already-recorded version is skipped; if its recorded
  checksum differs from the source's, the run fails rather than silently re-run
  or ignore the edit.
- **Transaction** — each migration's `Up` and its version row are applied in one
  gorm transaction. On PostgreSQL and SQLite DDL is transactional, so a failed
  `Up` rolls back cleanly; on MySQL each DDL statement auto-commits (a limitation
  Flyway shares), but the version row is written only after `Up` succeeds, so a
  crash mid-migration leaves the row absent and the next run retries.

## Selecting the database and source

| Situation | Behaviour |
|---|---|
| `db-ref` set | migrates the `*gorm.DB` bean of that name; unknown name is a fail-fast error |
| `db-ref` empty, one `*gorm.DB` bean | uses that sole bean |
| `db-ref` empty, several beans | fail-fast — set `db-ref` to disambiguate |
| `migration.Source` bean named after the entry | preferred (the `go:embed` case) |
| no such bean, `source-dir` set | reads `V<version>__<name>.sql` files from disk |
| neither present | fail-fast |

## Caveats

- **Forward-only, fail-stop**, matching Flyway community edition: there is no
  automatic down-migration. The `Down` field on a `migration.Migration` is
  reserved for tooling and is never executed by the Runner.
- **Immutable history.** Once a version is applied, its script is frozen —
  editing it changes the checksum and fails the next startup. Add a new
  higher-versioned migration instead.

## Configuration

Bound under `${spring.migration.<name>}` — one entry per database.

| Key | Default | Description |
|---|---|---|
| `spring.migration.<name>.enabled` | `true` | Turn this entry on/off. |
| `spring.migration.<name>.db-ref` | `""` | Name of the `*gorm.DB` bean to migrate. |
| `spring.migration.<name>.source-dir` | `""` | On-disk directory of `.sql` files; fallback when no `Source` bean is named after the entry. |
| `spring.migration.<name>.baseline` | `0` | Record versions `<=` this as applied without running them. |
| `spring.migration.<name>.allow-out-of-order` | `false` | Permit applying a version below the highest already applied (a gap fill). |
| `spring.migration.<name>.table` | `schema_migrations` | Version-table name; must be a plain SQL identifier. |

## License

Apache 2.0. See [LICENSE](../../LICENSE).
