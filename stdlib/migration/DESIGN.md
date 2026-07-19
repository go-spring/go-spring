# migration Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`migration` is the zero-dependency schema-migration abstraction in the stdlib
layer. It gives Go the *effect* of Flyway / Liquibase — versioned, checksum-guarded,
forward-only migrations applied exactly once — without copying their XML/DSL
machinery. A backend (gorm over MySQL/PostgreSQL/SQLite) is contributed as a
separate starter that supplies a `Store`.

## 1. Responsibilities and Boundaries

- Model a schema change as a numbered `Migration` whose `Up` issues statements
  through a driver-agnostic `Execer`.
- Collect migrations from a `Source` — an embedded `.sql` directory (the
  single-binary case) or code registration.
- Apply the unapplied migrations, in version order, recording each in a version
  table so a re-run is a no-op and an edited historical script is caught.
- Refuse to own the connection, the DDL dialect, or rollback. Opening the
  database and running SQL is the `Store`'s job; automatic down-migration is out
  of scope (forward-only, matching Flyway community edition).

## 2. Key Abstractions and Seams

- **`Store` interface as the backend seam.** There is no global driver registry.
  A backend needs a live handle (a `*gorm.DB`, a `*sql.DB`) to create the version
  table and run migrations transactionally, not a declarative policy — so the
  seam is the interface a starter implements, the same choice `stdlib/batch` and
  `stdlib/lock` make. The `Store` owns `EnsureVersionTable`, `AppliedRecords`,
  `Apply` (run `Up` + record in one transaction) and `MarkApplied` (baseline).
- **`Execer` decouples `Up` from the driver.** A migration's `Up` receives an
  `Execer` bound to the migration's transaction, so the same `Migration` runs on
  any backend and the `Store` decides transaction scope.
- **`Source` splits acquisition from execution.** `NewFSSource` parses
  Flyway-style names and SHA-256-hashes file bodies; `NewSource` takes
  code-defined migrations. The Runner consumes `[]Migration` and never knows
  where they came from.
- **Checksum is the tamper seam.** The source computes a checksum per migration
  (file hash for FS, caller-supplied for code); the Runner compares it against
  the recorded checksum and fails on drift. Both empty means "not tracked" and
  is allowed, so code migrations may opt out.

## 3. Constraints

- **Immutable history.** A version, once applied, is frozen; editing its script
  changes the checksum and fails the next run. The fix is a new higher version,
  never an edit — this is what makes a shared migration set safe across a team.
- **Version 0 and duplicates are rejected up front.** `loadSorted` validates the
  whole set before applying anything, so a malformed set fails before it mutates
  the database.
- **Ordering is strict by default.** A version below the highest applied one is
  rejected unless `AllowOutOfOrder` is set; the default protects against a
  late-merged low-numbered migration silently jumping the queue.
- **Forward-only, fail-stop.** `Down` is a reserved field the Runner never calls;
  a failed `Up` aborts the run and no later migration is attempted, so the
  database never lands in a half-known state from continuing past a failure.

## 4. Trade-offs and Alternatives Rejected

- **No driver-string registry.** Migration is not a config-time choice like
  `discovery` or `resilience`; you cannot express "the store I want" without a
  live database handle. Interface-as-seam beats registry indirection here.
- **No automatic rollback.** Down-migrations are unreliable in practice (a
  dropped column cannot be un-dropped with its data) and Flyway's community
  edition omits them too. `Down` stays a reserved field for external tooling
  rather than an executed path, so the framework never promises an undo it
  cannot honour.
- **Checksum guard over "just re-run".** Silently re-running an edited script
  would make the applied schema depend on run history; failing loud forces the
  immutable-history discipline that keeps environments identical.
- **Simple statement splitter, not a SQL parser.** `splitStatements` handles the
  DDL/DML a migration file carries (semicolons outside quotes/`--` comments);
  procedural bodies with inner semicolons belong in a single-statement file the
  driver executes whole, rather than justifying a full parser in stdlib.
