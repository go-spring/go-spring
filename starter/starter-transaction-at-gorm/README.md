# starter-transaction-at-gorm

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-transaction-at-gorm` contributes the **AT (Automatic Transaction)**
distributed-transaction capability defined in
[`go-spring.org/stdlib/transaction/at`](../../stdlib/transaction/at) to a
Go-Spring application, backed by [gorm](https://gorm.io). It is the Go-idiomatic
equivalent of **Seata AT**, reached without replicating Seata's TC/TM/RM roles.

AT's defining trait is **transparency**: you write no compensation code. A gorm
plugin intercepts each DML statement, captures a *before-image* (and, for
inserts, an *after-image*) into an `at_undo_log` row committed atomically with
your business data, and acquires a global row lock for write-write isolation. On
a global rollback the coordinator restores every changed row automatically from
that undo log.

It is a **Contributor**-archetype starter (see [DESIGN.md](../DESIGN.md) §2.3):
it opens no port and starts no server, only registers beans.

## Saga vs. TCC vs. AT — which one?

Go-Spring ships three distributed-transaction patterns; pick by how the undo is
written and how much isolation you need:

| | **Saga** | **TCC** | **AT** |
|---|---|---|---|
| Undo is written | by hand (a compensating function per step) | by hand (a `Cancel` per participant) | **derived automatically** from the captured before-image |
| Forward step | takes real effect immediately | **reserves** a resource (tentative) | takes real effect immediately (local commit) |
| Isolation | none | reservation invisible until Confirm | **global row lock** rejects a conflicting writer |
| Coupling | any resource (MQ, HTTP, cache) | any resource | a **SQL database via gorm** |
| Best for | long cross-service flows | short strongly-consistent flows | SQL data where you want automatic rollback with minimal code |

Reach for **AT** when the data lives in a SQL database and you want rollback for
free; for **TCC** when a resource must be *held* between try and commit; for
**Saga** when each step's effect is real and undone by explicit compensation.

## Installation

```bash
go get go-spring.org/starter-transaction-at-gorm
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-transaction-at-gorm"
```

The container now holds two beans:

- an `at.Coordinator` — the in-process orchestrator that commits (drops undo
  logs) or rolls back (restores from undo logs) every enrolled branch;
- an `at.GlobalLock` — the in-memory global row lock providing write-write
  isolation between concurrent global transactions.

### 2. Enrol each database in AT

For every `*gorm.DB` that should participate, migrate the undo-log table once and
install the plugin with a distinct **resource id** (recorded in undo logs and
lock keys). `coord` and `lock` are the beans above.

```go
if err := atgorm.Migrate(db); err != nil { ... }
if err := db.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil { ... }
```

### 3. Run a global transaction

Begin a global transaction, run each database write **inside its own local gorm
transaction** (so the undo log commits atomically with the business change), then
commit or roll back:

```go
ctx, xid := coord.Begin(ctx)
err := accountDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
    return tx.Model(&Account{}).Where("id = ?", 1).
        Update("balance", gorm.Expr("balance - ?", cost)).Error
})
if err != nil {
    _ = coord.Rollback(context.Background(), xid) // restores from before-images
    return err
}
return coord.Commit(context.Background(), xid)    // drops the undo logs
```

Each database self-registers a branch the first time it writes under `ctx`; a
database that writes several times is committed/rolled back exactly once. See
[example/example.go](example/example.go) for a runnable two-database demo
covering the commit path, the rollback path and the write-write conflict.

### 4. Or declare it as `@GlobalTransactional`

Wire `at.GlobalAT` into an aspect chain — the no-reflection equivalent of
`@GlobalTransactional(AT)`. It begins a global transaction, injects the xid, and
commits on success or rolls back on error:

```go
chain := aspect.NewChain(at.GlobalAT(coord))
```

Unlike Saga and TCC, AT needs **no method registry**: branches are discovered
from the SQL executed under the global transaction id on the context.

## How it works

- **Update / delete** — a *before* callback SELECTs the affected rows into a
  before-image and acquires the global lock on their primary keys before the
  write; an *after* callback records the undo log and registers the branch.
- **Insert** — an *after* callback reads the generated keys into an after-image,
  acquires the lock, and records the undo log so the row can be deleted on
  rollback.
- **Rollback** — undo logs for the transaction are replayed newest-first: an
  insert is deleted, a delete is re-inserted from its before-image, an update is
  set back to its before-image values; then the undo logs are dropped.
- **Recursion guard** — the plugin's own second-phase writes run on a suppressed
  context and the `at_undo_log` table is skipped, so restoration is never itself
  captured as new undo.

## Caveats

- **Run writes inside a gorm transaction.** The undo log is written on the same
  connection as the business statement; only a surrounding local transaction
  makes the two commit atomically.
- **Process-local.** The global lock and undo application are in-process — this
  is a single-process AT equivalent, not a distributed Seata TC deployment.
- **Image fidelity.** Before/after images are JSON-encoded, so numeric columns
  round-trip through `float64` (the same trade-off as
  `starter-transaction-saga-gorm`).

## Configuration

Bound under `${spring.transaction.at}` (shared with Saga and TCC under the
`spring.transaction` capability namespace, so several patterns can coexist).

| Key | Default | Description |
|---|---|---|
| `spring.transaction.at.enabled` | `true` | Turn the starter's beans on/off. |
| `spring.transaction.at.tracing` | `true` | Emit an otel child span per branch phase (commit / rollback) on the globals `starter-otel` installs. No-op without it. |

## License

Apache 2.0. See [LICENSE](../../LICENSE).
