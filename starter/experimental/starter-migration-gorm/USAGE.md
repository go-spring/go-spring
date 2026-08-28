# starter-migration-gorm Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against this module (`starter.go`, `config.go`, `runner.go`, `store.go`), the engine it drives
(`cloud/data/migration`: `migration.go`, `runner.go`, `source_fs.go`), the gs startup sequence
(`spring/gs/internal/gs_app/app.go:272-330`), and the runnable, self-asserting
[example/](example/) (embedded migrations + `check.sh` smoke). **Flyway's own semantics are
[Flyway's documentation](https://documentation.red-gate.com/fd/quickstart-migrations-184127599.html)**
— this starter is the Go-Spring equivalent pattern, forward-only and fail-stop like Flyway
community edition (no automatic down-migration, `migration.go:31-33`).

**Activation**: the Runner bean registers only when `spring.migration.*` is configured
(`starter.go:57-60`, `gs.OnProperty("spring.migration")` — a prefix check, so any
`spring.migration.<name>...` key activates it). Blank-import the starter; do not import it
for side effects if you configure nothing.

---

## 1. Complete worked project

A service whose schema is migrated from embedded SQL before the first request can arrive:

```
demo/
├── go.mod
├── main.go
├── db.go               # opens + names the *gorm.DB bean
├── migrations/
│   ├── V1__create_widgets.sql
│   └── V2__seed_widgets.sql
└── conf/app.properties
```

**go.mod** (deps that matter):

```
require (
    gorm.io/driver/sqlite             latest   // or starter-gorm-mysql etc.
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-gorm-sqlite latest   // or your dialect starter
    go-spring.org/starter-migration-gorm latest
)
```

**migrations/V1__create_widgets.sql** / **V2__seed_widgets.sql** (verbatim from the example):

```sql
CREATE TABLE widgets (
    id   INTEGER PRIMARY KEY,
    name VARCHAR(64) NOT NULL
);
```
```sql
INSERT INTO widgets (id, name) VALUES (1, 'sprocket');
INSERT INTO widgets (id, name) VALUES (2, 'gizmo');
```

**db.go** — the application supplies the database **by name**:

```go
package main

import (
    "embed"

    "go-spring.org/cloud/data/migration"
    "go-spring.org/spring/gs"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

//go:embed migrations
var migrationsFS embed.FS

func openDB() (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open("file:demo?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    sqlDB, _ := db.DB()
    sqlDB.SetMaxOpenConns(1) // in-memory sqlite needs one pinned connection
    return db, nil
}

func init() {
    // Both beans are named "app" to match the spring.migration.app config entry:
    // the starter matches config entry, DB bean and Source bean by that shared name.
    gs.Provide(openDB).Name("app")
    gs.Provide(func() migration.Source {
        return migration.NewFSSource(migrationsFS, "migrations")
    }).Name("app")
}
```

**main.go**:

```go
package main

import (
    _ "go-spring.org/starter-migration-gorm"

    "go-spring.org/spring/gs"
)

func main() { gs.Run() }
```

**conf/app.properties** — the complete, commented surface:

```properties
# One migration entry named "app". Matches the *gorm.DB bean "app" and the
# migration.Source bean "app". db-ref is optional when only one *gorm.DB bean
# exists, but shown for discoverability.
spring.migration.app.db-ref=app
spring.migration.app.enabled=true
```

**Verify** (the example's own three-guarantee smoke, `example/example.go:105-156`):

```bash
cd demo && CGO_ENABLED=1 go run .      # or ./check.sh in the shipped example
# log: migration: entry "app" applied 2 migration(s)
# On a file-backed DB you can additionally inspect the result:
#   sqlite3 demo.db 'SELECT version, name FROM schema_migrations;'   -- 2 rows
#   sqlite3 demo.db 'SELECT COUNT(*) FROM widgets;'                  -- 2
```

A second start applies nothing: `applied 0 migration(s)` (version rows already present).

---

## 2. Assembly & timing — when migrations run

`gs.Run()` → `App.Start()` executes, in order (`gs_app/app.go:274-283`):

1. properties refresh, logging init
2. **IoC container refresh** — every bean is wired (Rooter `Init` methods run here); the
   migration Runner's fields are populated: `Entries` from `${spring.migration}`, `DBs` and
   `Sources` collect every `*gorm.DB` / `migration.Source` bean keyed by bean name
   (`runner.go:34-48`)
3. **Runners execute, synchronously and sequentially** (`app.go:311-316`) — migration happens
   here; any error aborts `Start` and the app exits before serving (`starter.go:39-41`:
   "a broken schema never serves traffic")
4. **servers start** in goroutines; readiness waits on every server's `ReadySignal`
   (`app.go:318-330`)

So migrations run **after all beans are wired, before any server accepts traffic** — a
repository or DAO never queries a table a migration has not yet created. ⚠ Ordering **among**
multiple Runners is the bean-collection order of `App.Runners`; there is no explicit
dependency edge between the migration Runner and your other Runners — if another Runner reads
the schema, verify it lands after migration (design suspect, §6).

Within `Run`, entries are processed in **name-sorted order** (`runner.go:54-59`); disabled
entries log at Info and skip. Each entry independently: `pickDB` (db-ref → named bean; absent
db-ref works only when exactly one `*gorm.DB` bean exists, `runner.go:90-110`), `pickSource`
(a `migration.Source` bean named after the entry wins; else `source-dir` from disk,
`runner.go:115-124`), then `Migrate`.

## 3. Per-key behavior reference

Keys under `spring.migration.<name>.*` (map bind via `${spring.migration}`, `runner.go:37`).
The same set is declared in `schema.json`.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `<name>` (entry itself) | map key | — | **Activation**: any `spring.migration.<name>.*` key registers the Runner. Entry name must match the `migration.Source` bean name (if you use one). | Name mismatch → falls through to `source-dir`; if that is empty too, fail-fast "neither a migration.Source bean ... nor a source-dir". |
| `enabled` | bool | true | Per-entry off switch; keeps config, skips the run with an Info log (`runner.go:62-65`). | — |
| `db-ref` | string | "" | Names the `*gorm.DB` bean. Empty + exactly one DB bean → that one; empty + 0 or ≥2 beans → fail-fast (`runner.go:90-110`). ⚠ with multiple databases, omitting it either fails (0/≥2) or risks migrating the wrong one. | "no *gorm.DB bean of that name is registered" / "set db-ref to disambiguate". |
| `source-dir` | string | "" | On-disk directory of `V<n>__<name>.sql` files, read via `os.DirFS` (`runner.go:120`). Ignored when a Source bean of the entry's name exists (embed wins). Relative paths resolve against the process CWD. | Wrong path → "read dir" error at startup; missing both source forms → fail-fast. |
| `baseline` | uint64 | 0 | Every migration with `Version <= baseline` is **recorded without running** (`runner.go:104-109`; `MarkApplied` writes only the version row, `store.go:111-113`). 0 disables. | Too high → newer migrations silently skipped-as-baselined; too low → applied to a DB that already has the objects → SQL "already exists" errors. |
| `allow-out-of-order` | bool | false | Permits a pending version **below** the highest applied (gap fill). Default false = history must extend strictly upward (`migration/runner.go:110-114`). | Left false after adding a backfill V3 when V5 is applied → startup fails "out-of-order migration; set allow-out-of-order". |
| `table` | string | schema_migrations | Version-table name. Must be a plain SQL identifier `^[A-Za-z_][A-Za-z0-9_]*$` — validated at startup (`store.go:32,54-57`); it is interpolated into DDL, it cannot be a bound parameter. | Invalid name → fail-fast before any DDL; changing it later orphans prior history (the old table's rows are never read). |

Migration-file naming (engine semantics, `source_fs.go:37-44`): `V<version>__<name>.sql` —
leading V optional and case-insensitive, double-underscore separator, version a non-negative
integer compared **numerically** (V2 < V10). Content is SHA-256-hashed into the recorded
checksum. Non-`.sql` files ignored; subdirectories not descended. A malformed name is a hard
error, not a silent skip. Files are split into statements on semicolons (quote- and
`--`-comment-aware) — procedures/PL-pgSQL blocks with inner semicolons must be issued as
single-statement files (`source_fs.go:130-135`).

## 4. Verification & fault drills

All drills mirror the example's guarantees (`example/example.go:105-156`); use a file-backed
sqlite so state survives restarts.

1. **Startup apply + idempotency**: start once → `schema_migrations` has 2 rows, `widgets` 2
   rows; start again → `applied 0 migration(s)` in the log.
2. **Checksum drift (edited history)**: edit `V1__create_widgets.sql` after it was applied →
   restart fails with `checksum mismatch for version 1 ... a migration already applied was
   edited; migrations are immutable history` (`migration/runner.go:95-99`). Revert the edit —
   or, intentionally, accept the new reality by hand-editing the stored checksum.
3. **Dirty-state recovery (crash mid-migration)**: the Up SQL and the version row run in one
   gorm transaction (`store.go:99-108`). On PostgreSQL/SQLite DDL is transactional → a failed
   Up rolls back cleanly and the next start retries. ⚠ On MySQL each DDL statement
   auto-commits (a MySQL limitation Flyway shares, `store.go:96-98`): a crash can leave
   applied DDL with **no** version row — the next start retries the file and dies on
   "table already exists"; fix by hand (drop/complement the objects, or insert the version
   row) then restart.
4. **Baseline on an existing schema**: point the entry at a database that already carries the
   schema of V1; set `spring.migration.app.baseline=1` → V1 is recorded without running, V2
   applies. Drill: check `schema_migrations` now contains V1 with `applied_at` set and V1's
   objects untouched.
5. **Out-of-order gap fill**: ship files V1, V2, V3 and V5 (V4 deliberately absent), start —
   all four apply. Later add `V4__gap.sql` and restart → startup fails with "out-of-order
   migration; set allow-out-of-order" (V4 is below the highest applied V5). Set
   `spring.migration.app.allow-out-of-order=true`, restart → V4 applies below V5. Remove the
   flag again once the gap is filled.
6. **Programmatic run (no starter)**: `store, _ := migrationgorm.NewStore(db, "schema_migrations")`
   then `migration.NewRunner(store, src, migration.Options{}).Migrate(ctx)` — the exported
   seam behind admin commands and smoke tests (`store.go:45-47`).

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Starter never runs, no migration logs | No `spring.migration.*` key configured | Add at least one entry — `OnProperty("spring.migration")` is the activation switch |
| `references db-ref %q but no *gorm.DB bean of that name` | db-ref typo, or bean registered under another name | Name the `gs.Provide(openDB).Name("app")` bean exactly as db-ref |
| `no db-ref but N *gorm.DB beans exist` | Multiple databases, no explicit selection | Set `db-ref` per entry |
| `neither a migration.Source bean ... nor a source-dir` | Entry name ≠ Source bean name and no source-dir | Align names, or set `source-dir` |
| `checksum mismatch for version N` | An already-applied .sql file was edited | Revert the edit (history is immutable); or consciously update the stored checksum |
| `out-of-order migration; set allow-out-of-order` | New file with version below the highest applied | Add the gap fill as a new highest version, or set `allow-out-of-order=true` |
| `table already exists` on retry (MySQL) | DDL auto-committed mid-migration, version row absent (§4.3) | Reconcile schema by hand, restart; the retry then succeeds |
| `invalid version-table name` | `table` key fails the identifier regex | Plain identifier only — no quoting, no dots |
| Migration applies but app still fails on missing table | Another Runner read the schema before migration ran (Runner order is collection order) | Audit Runner ordering; see design suspect §6 |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 per entry (+ the `spring.migration` map bind) |
| Required | 0 syntactically; in practice `db-ref` (multi-DB) and one of Source-bean / `source-dir` |
| Quickstart external deps | 1 (the database; 0 with in-memory sqlite) |
| "Watch out" entries | 4 |

Design suspects: Runner ordering vs other Runners is implicit — relies on runner-before-server
with no explicit dependency edge; `schema.json` types `baseline` as `object` (should be
`integer` — metadata bug, config binding itself is a uint64 and works); failure
mid-migration is not surfaced as a distinct error class (the dirty state must be inferred
from "table already exists" on retry); `source-dir` relative-path resolution depends on CWD
(consider documenting/absolutizing).
