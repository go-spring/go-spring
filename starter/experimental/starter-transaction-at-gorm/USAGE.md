# starter-transaction-at-gorm Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavioral claim below is
verified against the starter source (`starter.go`, `config.go`, `plugin.go`, `branch.go`,
`undolog.go`) and the capability core `cloud/experimental/transaction/at`
(`at.go`, `coordinator.go`, `lock.go`, `global.go`). **AT pattern semantics (undo log /
before-image, two-phase commit/rollback, global-lock isolation — "what Seata AT does") are
[distributed-transaction literature](https://seata.apache.org/docs/user/at-mode)** — this page
covers only the go-spring increment.

**Scope**: the global lock and undo log are process-local. This is a single-process AT
equivalent, not a distributed Seata TC/TM/RM deployment (starter.go package comment).
**Activation**: a blank import is enough — both config keys default to on. There is **no
recovery Runner**: nothing replays undo logs after a crash (see §4.3, §6).

---

## 1. Complete worked project

Two toy databases (in-memory sqlite) — an account balance and a stock quantity — debited by
one global transaction. A deliberate SQL failure in the second database drives an automatic
rollback of both, restored from captured before-images. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring                    v1.3.x
    go-spring.org/starter-transaction-at-gorm latest
    go-spring.org/starter-otel               latest   // optional: real trace export
    gorm.io/gorm                             latest
    gorm.io/driver/sqlite                    latest   // or mysql/postgres driver starter
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-transaction-at-gorm"
)

func main() { gs.Run() }
```

**service.go** — the entire application surface:

```go
package main

import (
    "context"
    "errors"

    "go-spring.org/cloud/experimental/transaction/at"
    "go-spring.org/spring/gs"
    atgorm "go-spring.org/starter-transaction-at-gorm"
    "gorm.io/gorm"
)

type account struct {
    ID      int64 `gorm:"primaryKey;column:id"`
    Balance int   `gorm:"column:balance"`
}
func (account) TableName() string { return "account" }

type stock struct {
    ID    int64 `gorm:"primaryKey;column:id"`
    Count int   `gorm:"column:count"`
}
func (stock) TableName() string { return "stock" }

// The two starter beans are autowired into the constructor: the Coordinator
// (begin/commit/rollback) and the GlobalLock (write-write isolation).
type BankService struct {
    coord     at.Coordinator
    accountDB *gorm.DB
    stockDB   *gorm.DB
}

func newBankService(coord at.Coordinator, lock at.GlobalLock) (*BankService, error) {
    accountDB, err := openDB("account-db")
    if err != nil { return nil, err }
    stockDB, err := openDB("stock-db")
    if err != nil { return nil, err }

    if err := accountDB.AutoMigrate(&account{}); err != nil { return nil, err }
    if err := stockDB.AutoMigrate(&stock{}); err != nil { return nil, err }

    // Enrollment step 1: the at_undo_log table (fail-fast).
    if err := atgorm.Migrate(accountDB); err != nil { return nil, err }
    if err := atgorm.Migrate(stockDB); err != nil { return nil, err }

    // Enrollment step 2: the AT plugin, with a DISTINCT resource id per database —
    // it is the branch id in undo logs and lock keys, and the coordinator
    // deduplicates branches by it.
    if err := accountDB.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil { return nil, err }
    if err := stockDB.Use(atgorm.NewPlugin("stock-db", coord, lock)); err != nil { return nil, err }

    if err := accountDB.Create(&account{ID: 1, Balance: 100}).Error; err != nil { return nil, err }
    if err := stockDB.Create(&stock{ID: 1, Count: 10}).Error; err != nil { return nil, err }
    return &BankService{coord: coord, accountDB: accountDB, stockDB: stockDB}, nil
}

// purchase runs one global transaction. failStock=true fails the second database
// on purpose: the global transaction then rolls back and BOTH databases are
// restored from their before-images — no compensation code was ever written.
func (s *BankService) purchase(ctx context.Context, cost, qty int, failStock bool) error {
    ctx, xid := s.coord.Begin(ctx)

    err := s.accountDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return tx.Model(&account{}).Where("id = ?", 1).
            Update("balance", gorm.Expr("balance - ?", cost)).Error
    })
    if err == nil {
        err = s.stockDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
            if failStock {
                return errors.New("stock service unavailable") // deliberate failure
            }
            return tx.Model(&stock{}).Where("id = ?", 1).
                Update("count", gorm.Expr("count - ?", qty)).Error
        })
    }

    if err != nil {
        if rbErr := s.coord.Rollback(context.Background(), xid); rbErr != nil {
            return err // rollback error reported alongside the business error
        }
        return err
    }
    return s.coord.Commit(context.Background(), xid)
}
```

(`openDB` and the `gs.Provide(newBankService)` wiring are in
[example/example.go](example/example.go) — copy from there; each sqlite handle uses
`MaxOpenConns(1)` so the shared in-memory database is not duplicated per connection.)

**conf/app.properties** — the complete, commented surface (both keys are defaults, shown
for discoverability):

```properties
# --- AT distributed transaction ----------------------------------------------
# Blank import is enough; set false to import the module without contributing
# the coordinator/lock beans (OnProperty ... MatchIfMissing).
spring.transaction.at.enabled=true

# One otel child span per branch second-phase operation (at.commit <branch> /
# at.rollback <branch>) on the globals starter-otel installs. No-op — costs
# almost nothing — without starter-otel (the global tracer is then a no-op).
spring.transaction.at.tracing=true

# Startup crash recovery: scan every enrolled database's at_undo_log and roll
# back the undo logs a previous run's crash left behind. Set false only when
# the database is shared with other processes and recovery is external.
spring.transaction.at.recover-on-start=true

# --- observability (starter-otel, optional; from example-otel) ---------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
```

**Verify** (same shape as the smoke-tested example):

```bash
bash example/check.sh
# expect:
#   commit path OK: balance=70 stock=8 undo=0
#   rollback path OK: balance=70 stock=8 undo=0 - stock service unavailable
#   isolation path OK: second global transaction rejected with ErrLockConflict
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-transaction-at-gorm
  ├─ gs.Provide(MemoryGlobalLock as at.GlobalLock)   [cond: enabled + OnMissingBean]
  ├─ gs.Provide(newCoordinator as at.Coordinator)    [cond: enabled]
  └─ gs.Provide(newRecoveryRunner as gs.Runner)      [cond: enabled + recover-on-start]
        │
gs.Run()
  ├─ config bind: ${spring.transaction.at} → Config      (3 value tags)
  ├─ lock bean: in-memory MemoryGlobalLock; a durable-lock bean from the
  │  container makes it step aside (OnMissingBean)         (starter.go:77-79)
  ├─ coordinator bean: at.NewCoordinator(WithGlobalLock(lock)
  │  [+ WithObserver(AtObserver{}) when tracing=true])    (starter.go:91-98)
  ├─ your bean constructor runs: Migrate(db) + db.Use(NewPlugin(resource, coord, lock))
  │  per database — enrollment is CODE wiring, invisible in configuration; the
  │  plugin registers the handle for the recovery scan (plugin.go Initialize)
  └─ recovery Runner: scans each enrolled database's at_undo_log for orphans a
     previous run's crash left and rolls them back (§4.3); no server, no port
```

Log line to expect at construction: `at coordinator created tracing=true` (tag `AppDef`).

### 2.2 One global transaction with a failure — full walk

`purchase(ctx, 50, 5, failStock=true)`, rationale cited from source:

1. **Begin** — `coord.Begin(ctx)` mints a random 16-byte hex XID, maps `xid → nil` in the
   coordinator's `active` table and returns `WithXID(ctx, xid)`
   (coordinator.go:73-79). The XID riding the context is the *only* signal that
   distinguishes a global-transaction write from an ordinary one.
2. **Branch enroll (already done at construction)** — `db.Use(NewPlugin(...))` registered
   gorm callbacks: before/after `gorm:update`, before/after `gorm:delete`, after
   `gorm:create` (plugin.go:70-89). `Migrate` created `at_undo_log` beforehand — the
   plugin's own writes to that table are skipped by a table-name guard
   (plugin.go:99-100) so capture never recurses.
3. **SQL executes with undo-log capture** — `accountDB.Update("balance", ...)`:
   - `before_update` fires: `active()` checks suppressed-flag / undo-log table / XID
     (plugin.go:94-107), SELECTs the before-image with the statement's *own WHERE
     clause* (`whereExpr`, so image == rows the write will touch), and acquires the
     global lock on those rows all-or-nothing — a lock conflict fails the *statement*
     so the local transaction rolls back rather than staging a conflicting change
     (plugin.go:113-131, lock.go:47-64).
   - the UPDATE runs; `after_update` fires: re-reads the after-image by primary keys,
     JSON-encodes `{before, after}` into one undo row and INSERTs it **on the same
     connection/transaction as the business data** so the two commit atomically
     (plugin.go:186-190), then registers the branch
     (`coord.Register`; deduplicated by resource id — one branch per database per
     XID, coordinator.go:81-95).
   - the local gorm transaction commits: business change + undo log are now durable
     **together**. This is AT phase one.
4. **Failure** — the stock step returns `errors.New("stock service unavailable")` before
   any SQL: `stock-db` never registers a branch for this XID. The error propagates to
   `purchase`, which calls `coord.Rollback(ctx, xid)`.
5. **Rollback via undo log** — the coordinator takes the branch list (single-shot: a
   second resolution of the same XID reports `ErrUnknownTransaction`,
   coordinator.go:134-143) and unwinds branches in **reverse registration order**
   (coordinator.go:120-128). The branch's `Rollback` reads undo rows `ORDER BY id DESC`
   — newest statement undone first (branch.go:53-68) — and `restore` reverses each by
   SQL type: INSERT → delete the row by its after-image PKs, DELETE → re-insert from the
   before-image, UPDATE → write the before-image values back (branch.go:73-107). All
   restoration runs on a **suppressed context** so it is not re-captured as new undo
   logs (plugin.go:34-39). Finally the undo logs themselves are deleted.
6. **Lock release** — the coordinator releases every lock key held by the XID; a release
   error is deliberately swallowed since the transaction is already resolved
   (coordinator.go:149-153).

On the success path, step 5 becomes: every branch *deletes its undo logs* (phase-two
commit is cheap — the business data already committed locally, branch.go:42-46), then lock
release.

---

## 3. Per-key behavior reference

Prefix `${spring.transaction.at}` — the shared `spring.transaction` capability namespace,
so Saga (`spring.transaction.saga.*`) and TCC can coexist. Wrapper `value` tags are
top-level absolute keys under that prefix. Verified against
`grep -rhoE 'value:"[^"]+"' --include='*.go'` — exactly these three:

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.transaction.at.enabled` | bool | `true` | `OnProperty(...).HavingValue("true").MatchIfMissing()` (starter.go:70). `false` contributes neither bean — import without assembly. | `false` → autowiring `at.Coordinator`/`at.GlobalLock` fails at container assembly (no such bean); runtime writes run without capture, silently non-AT. |
| `spring.transaction.at.tracing` | bool | `true` | Appends `at.WithObserver(transaction.AtObserver{})` at coordinator construction (starter.go:93-94). Span per branch phase: `at.commit <branch>` / `at.rollback <branch>`, attributes `at.xid` / `at.branch` / `at.phase` (cloud/experimental/transaction/observe.go). | `false` (or forgetting starter-otel with `true`): silent — no spans, no warning. |
| `spring.transaction.at.recover-on-start` | bool | `true` | Registers the crash-recovery gs.Runner (starter.go:95-99, recovery.go). At startup it scans every AT-enrolled database's `at_undo_log` and rolls back each orphaned XID (see §4.3). | `false` → orphaned undo logs from a crash are NOT replayed: `at_undo_log` keeps growing and phase-one business changes stay applied until reconciled manually. |

No enum keys; no coupled keys. Note the *enrollment* knobs (resource id, which databases
join) are **code, not configuration** — see §6.

---

## 4. Verification & fault drills

### 4.1 Rollback on SQL error — inspect at_undo_log before/after

Run the example (`bash example/check.sh` or `go run . -manual` in example/). The
assertions inside `runTest` are exactly this drill:

- After the failing `purchase(50, 5, failStock=true)`: `balance=70 stock=8 undo=0` — the
  account debit of 50 was **restored to 70** from its before-image even though its local
  transaction had already committed, and the undo log is gone.
- Against a real database, the equivalent manual check is
  `SELECT COUNT(*) FROM at_undo_log` → 0 after every resolved global transaction; a
  non-zero count that persists means a branch's phase two failed (see §5,
  StatusCommitFailed/RollbackFailed).

### 4.2 Write-write isolation drill

Path 3 of the example: hold one global transaction open on account row 1, then begin a
second touching the same row — the second statement fails with
`at: global row lock conflict` (`at.ErrLockConflict`), so the second transaction stages
nothing. Read it in traces as an errored `at.*` span, or assert `errors.Is(err,
at.ErrLockConflict)` as the example does.

### 4.3 Crash recovery drill — orphan undo logs after kill -9 — **fixed: startup Runner**

The recovery Runner (recovery.go; `spring.transaction.at.recover-on-start`, default on)
resolves what this section previously recorded as a suspect:

- **kill -9 between phase one and phase two**: the business change and its undo log are
  already committed locally. The coordinator's `active` map dies with the process, so
  the global transaction can never commit — on the next boot the Runner scans each
  enrolled database's `at_undo_log` (any row at boot is an orphan: nothing of this
  process can be in flight before Runners execute), logs a loud ERROR with the count and
  the oldest entry, then rolls back each XID by replaying its undo (`gormBranch.Rollback`,
  newest log first). Verified by `TestATRecovery_ReplaysOrphanedUndoLogs`.
- **kill -9 during phase-two rollback**: partially restored rows. Replay is safe:
  `Branch.Rollback` is idempotent (branch.go:52-53) — undo logs are only deleted after a
  full replay, so the next boot finishes the job.
- A failed replay is logged ERROR and its undo logs are kept (nothing is deleted), so
  the situation degrades to the old behavior — visible, with data for manual recovery.
- The Runner never fails startup, and scans each enrolled database independently.

Two limits to know:

- **Wiring-time enrollment**: the Runner discovers databases through the plugin's
  `Initialize` registration (plugin.go), so the plugin MUST be installed during bean
  construction (`db.Use` in a constructor), not from a later Runner — otherwise the scan
  misses that database.
- **Single-process contract**: sharing the database between processes breaks the
  "any boot-time row is an orphan" assumption (another instance's in-flight transaction
  would be rolled back). Such deployments must set `recover-on-start=false` and
  reconcile manually; cross-database branch ordering is also not reconstructed
  (each database replays its own logs in reverse id order).

Manual drill on a file-backed database: start the example against sqlite on disk, kill -9
mid-transaction (add a sleep between the two branch transactions), restart — expect the
`at recovery: found N orphaned undo-log entries ...` ERROR followed by one
`at recovery: global transaction "<xid>" rolled back` INFO per XID, and
`SELECT COUNT(*) FROM at_undo_log` → 0.

### 4.4 Tracing drill

`example-otel/` (docker-compose: Jaeger on :4317/:16686) drives the same three paths and
then verifies traces appear for service `transaction-at-gorm-otel-example`. Expect one
`at.commit account-db` + one `at.commit stock-db` span per committed transaction and
`at.rollback account-db` on the failure path, each tagged with the XID.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `at: unknown global transaction` on Register | Commit/Rollback already resolved this XID (single-shot), or Begin was skipped | One resolution per XID; re-Begin for a new attempt (coordinator.go:134-143). |
| `at: global row lock conflict` on a write | Another in-flight global transaction holds the row | Retry after the other resolves; this is the isolation guarantee working, not a bug. |
| Writes run but no undo log, no rollback | Database never enrolled: `Migrate`/`db.Use(NewPlugin(...))` missing, or writes run on a context without XID | Enroll per database; run writes under `coord.Begin`'s context (or `at.GlobalAT(coord)`). |
| `at_undo_log` table missing / INSERT fails | `Migrate(db)` not called on that database before first captured write | Call `Migrate` fail-fast at startup, next to `AutoMigrate`. |
| Two databases interfere / branch double-freed | Same `resource` id passed to `NewPlugin` for both | Distinct id per database — it is the branch id and part of lock keys. |
| Rollback error `branch %q rollback: ...` surfaced | Restoration SQL failed (schema drift, row gone, connection) | Inspect `at_undo_log.context` JSON for the dead XID; restore manually; see StatusRollbackFailed semantics (at.go:180-182). |
| Undo-log rows accumulate and never clear | `recover-on-start=false`, or a replay failed (its ERROR names the XID and the logs are kept) or StatusCommitFailed branches | Monitor `SELECT COUNT(*) FROM at_undo_log`; read `at_undo_log.context` JSON for the dead XID and restore manually (safe: their transactions can never resolve again). |
| Duplicate plugin error from gorm | `db.Use` called twice with same name (`at:<resource>`) | Enroll once per handle. |
| Crash orphans NOT rolled back after restart | Plugin installed after wiring (from a Runner / inside `gs.Run`), so the recovery scan never saw the database | Install `db.Use(NewPlugin(...))` during bean construction, as the example does. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 3 |
| Required | 0 |
| Quickstart external deps | 0 (1 Jaeger only for example-otel tracing) |
| "Watch out" entries | 6 |

Design suspects (kept from the previous audit + new):

- ~~**No crash-recovery Runner for AT**~~ — resolved: a startup recovery Runner now
  replays orphaned undo logs (§4.3), closing the asymmetry with Saga/TCC. Remaining
  caveats: single-process contract only (disable via `recover-on-start=false` when the
  database is shared) and no cross-database branch-order reconstruction.
- `Migrate`/`db.Use` are manual per-database steps, invisible in configuration —
  acceptable (gorm plugins are code-wired), but nothing fails fast if a DB is forgotten;
  it simply participates as a non-AT database.
- Enrollment order is conventional, not enforced: `Migrate` before first captured write
  is a runtime contract; a plugin installed before `Migrate` only fails on the first
  intercepted DML.
- Phase-two failure semantics are asymmetric: commit failure = cleanup failure (data is
  consistent), rollback failure = possible inconsistency needing alerting
  (at.go:176-182) — but the starter exposes no metric/alarm hook to tell them apart
  beyond span error status.
- `RetryPolicy` exists on the core coordinator (`at.WithRetry`) but the starter wires no
  configuration for it — a second-phase retry is unreachable via properties.
- Undo-log growth monitoring is left entirely to the user (no metric on undo-log count).
