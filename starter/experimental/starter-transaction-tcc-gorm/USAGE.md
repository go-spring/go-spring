# starter-transaction-tcc-gorm Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavioral claim below is
verified against the starter source (`starter.go`, `config.go`, `store.go`) plus the
consumers of its bean: starter-transaction-tcc (`starter.go`, `recovery.go`, `config.go`)
and the capability core `cloud/experimental/transaction/tcc/coordinator.go`. **TCC pattern
semantics (Try / Confirm / Cancel) are
[distributed-transaction literature](https://seata.apache.org/docs/user-mode/tcc-mode) —
this page covers only the go-spring increment.**

This starter contributes exactly one thing: a durable, gorm-backed `tcc.Store` for the TCC
coordinator, which is what turns on crash recovery. Scope note: the store persists the TCC
*log* (snapshots), not business data; confirming/cancelling business effects is still your
`Participant` functions. This module has **no example/** — the worked project below is
code-verified against `store_test.go` (including the end-to-end execute-then-recover test),
not smoke-verified against a running database.

---

## 1. Complete worked project

An order-placement TCC over a MySQL business database: reserve stock, charge payment, and a
deliberately failing Try that forces cancellation of the tried participants. The TCC log —
including the terminal `Cancelled` record — persists in `tcc_snapshots` and survives a
restart. File tree:

```
demo/
├── go.mod
├── main.go
├── order.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring                      v1.3.x
    go-spring.org/starter-gorm-mysql           latest   // provides the *gorm.DB
    go-spring.org/starter-transaction-tcc      latest   // coordinator + registry + recovery Runner
    go-spring.org/starter-transaction-tcc-gorm latest
    go-spring.org/starter-otel                 latest   // optional: real trace export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-transaction-tcc"
    _ "go-spring.org/starter-transaction-tcc-gorm"
)

func main() { gs.Run() }
```

**order.go** — the entire application surface:

```go
package main

import (
    "context"
    "errors"

    "go-spring.org/cloud/experimental/transaction/tcc"
    "go-spring.org/spring/gs"
)

// Participant registration MUST happen at wiring time (bean construction): the
// startup recovery Runner rebuilds each crashed transaction from the
// ParticipantRegistry keyed by the persisted method name, so participants
// registered later — e.g. from inside a custom Runner — may be absent when
// recovery runs (starter-transaction-tcc/starter.go package comment, recovery.go).
func init() {
    gs.Provide(func(coord tcc.Coordinator, reg *tcc.ParticipantRegistry) *OrderTcc {
        reg.Register("OrderService.Place", []tcc.Participant{
            // Participant 1: reserve stock (Try); confirm spends it, cancel releases it.
            {Name: "reserve-stock",
                Try:     func(ctx context.Context) (any, error) { return reserve(ctx, 2) },
                Confirm: func(ctx context.Context, _ any) error { return spend(ctx, 2) },
                Cancel:  func(ctx context.Context, _ any) error { return release(ctx, 2) }},
            // Participant 2: charge payment.
            {Name: "charge-payment",
                Try:     func(ctx context.Context) (any, error) { return charge(ctx, 30) },
                Confirm: func(ctx context.Context, _ any) error { return commitCharge(ctx, 30) },
                Cancel:  func(ctx context.Context, _ any) error { return refund(ctx, 30) }},
            // Participant 3: publish — FAILS ON PURPOSE in Try, driving cancellation
            // of charge-payment then reserve-stock, with every snapshot the
            // coordinator takes persisted to tcc_snapshots.
            {Name: "publish-order",
                Try:     func(ctx context.Context) (any, error) { return nil, errors.New("broker unavailable") },
                Confirm: func(ctx context.Context, _ any) error { return nil },
                Cancel:  func(ctx context.Context, _ any) error { return nil }},
        })
        return &OrderTcc{coord: coord}
    }).Export(gs.As[gs.Rooter]())
}

type OrderTcc struct {
    coord tcc.Coordinator
}

func (o *OrderTcc) Run(ctx context.Context) error {
    // The @GlobalTransactional equivalent: on a Try error the coordinator has
    // already cancelled every tried participant in reverse and written the
    // terminal log.
    return tcc.GlobalTCC(o.coord, "OrderService.Place", func(ctx context.Context) error {
        return nil // participants run via the registry entry above
    })
}
```

(`reserve`/`charge`/etc. are your ordinary business calls — SQL via the autowired `*gorm.DB`
or RPC clients; TCC semantics are the linked literature.)

**conf/app.properties** — the complete, commented surface:

```properties
# --- datasource (starter-gorm-mysql; provides the autowired *gorm.DB) --------
spring.gorm.mysql.instances.dsn=app:pass@tcp(127.0.0.1:3306)/demo?parseTime=true

# --- TCC durable store (THIS starter's activation key) -----------------------
# Must be exactly "gorm" for this Store to register
# (OnProperty ... HavingValue("gorm"), NO MatchIfMissing — starter.go).
# With it set, the tcc starter's in-memory default Store steps aside
# (OnMissingBean) and the coordinator + recovery Runner consume this one.
spring.transaction.tcc.store=gorm

# --- TCC capability (parent starter; tracing/recovery toggles) ---------------
spring.transaction.tcc.enabled=true
# Default true, shown for discoverability. One otel child span per phase:
# tcc.try <participant> / tcc.confirm <participant> / tcc.cancel <participant>.
spring.transaction.tcc.tracing=true
# Startup recovery Runner: scan Pending() and cancel crashed transactions.
spring.transaction.tcc.recover-on-start=true

# --- observability (starter-otel, optional) -----------------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**Verify** (against the configured MySQL):

```bash
# after the failing transaction run:
mysql> SELECT id, method, status, in_progress, tried FROM tcc_snapshots;
# one row: the transaction id, "OrderService.Place", status=4 (Cancelled); see §4.1
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gorm-mysql + starter-transaction-tcc + starter-transaction-tcc-gorm
  ├─ tcc starter: ParticipantRegistry, in-memory Store [OnMissingBean],
  │   Coordinator, recovery Runner [cond: enabled + recover-on-start]
  └─ this starter: gs.Provide(newGormStore as tcc.Store)
        [cond: OnProperty("spring.transaction.tcc.store").HavingValue("gorm")]
        │
gs.Run()
  ├─ config bind: ${spring.transaction.tcc.gorm} → gormConfig (0 value tags)
  ├─ *gorm.DB autowired into newGormStore's second ctor arg
  ├─ store construction: db.AutoMigrate(&tccSnapshot{}) — creates
  │  tcc_snapshots, FAILS FAST (bean error) if it cannot        (starter.go)
  ├─ ordering: the durable Store wins the tcc starter's OnMissingBean slot, so
  │  newCoordinator consumes it (the in-memory default is never built)
  ├─ recovery Runner.Run(): Store.Pending() → for each snapshot, rebuild
  │  participants from the registry by method name and coord.Recover()
  └─ your Rooter/Runner beans run after assembly
```

Construction log line: `create gorm tcc store success` (tag `AppDef`).

### 2.2 One failing transaction — full walk, including persistence points

Cited from `cloud/experimental/transaction/tcc/coordinator.go` and `store.go`:

1. **Begin** — `GlobalTCC(coord, method, fn)` looks up the method's participants and calls
   `coord.Execute`; before each Try the coordinator records intent with a `StatusTrying`
   snapshot naming the participant about to run, so a crash mid-Try is known to need cancel
   (`persist`, coordinator.go:118).
2. **reserve-stock tries** — on success the coordinator upserts the trying snapshot with
   `tried=["reserve-stock"]`, `try_results={"reserve-stock": ...}` via `Store.Save` (an
   `OnConflict UpdateAll` upsert, store.go). One row per transaction id, constantly
   overwritten — `tcc_snapshots` is a log of *current state*, not an append-only history.
3. **charge-payment tries** — same cadence: snapshot now `tried=[reserve-stock,
   charge-payment]`. Each `persist` write is a save point a crash can resume from.
4. **publish-order FAILS** — `errors.New("broker unavailable")`. The coordinator cancels in
   reverse: `refund` (charge-payment), then `release` (reserve-stock). Each cancel is
   visible in traces and updates the same snapshot row.
5. **Terminal record** — `finish`: a *committed* transaction's row is deleted (work done,
   nothing to recover); a *cancelled or failed* transaction's row is **kept** with the
   terminal status for operator inspection (`StatusCancelled` = 4, coordinator.go:304-308).
6. **Crash recovery (restart)** — the recovery Runner reads `Pending()` (every non-terminal
   row — store.go), rebuilds the participant list from the registry by the persisted method
   name, and `coord.Recover` cancels from the log's position: the in-progress participant
   first (with a nil result — sidestepping the JSON round-trip caveat), then each tried
   participant in reverse. Unreached participants are not cancelled. A method with no
   registered participants is logged and skipped; an individual recovery error is logged
   and does not abort the remaining transactions.

Note the failure-durability asymmetry: persistence errors in `persist` are *intentionally
swallowed* — the transaction already made progress and failing the operation on a log write
would be worse than a gap in the log (coordinator.go `persist`).

---

## 3. Per-key behavior reference

`grep -rhoE 'value:"[^"]+"' --include='*.go'` on this module yields **zero** tags; the
activation key below is a bean *condition* property (grep-invisible) and is included because
it is the only way to switch this Store on. This module owns **no value-tag keys at all** —
the `*gorm.DB` is always the container's default instance.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.transaction.tcc.store` | string | (unset) | **Activation key** (condition, not a value tag). Must be exactly `gorm` — `OnProperty ... HavingValue("gorm")`, no `MatchIfMissing` (starter.go). | Any other value / unset → this Store never registers and the tcc starter's in-memory default stays: no `tcc_snapshots` table, **no crash recovery**, silently. |

Parent-starter keys that govern the machinery this Store feeds (documented in
[starter-transaction-tcc](../starter-transaction-tcc), listed here because they change this
Store's observable behavior): `spring.transaction.tcc.enabled` (default true),
`spring.transaction.tcc.tracing` (default true — per-phase child spans),
`spring.transaction.tcc.recover-on-start` (default true — the Runner that makes this Store
meaningful).

**Table schema** (created by the constructor's `AutoMigrate`, backend-agnostic — text and
int columns only, no dialect-specific types): `tcc_snapshots(id PK, method, status int
indexed, tried text, in_progress, try_results text, updated_at)`. `tried` / `try_results`
are JSON-encoded (`[]string` / `map[string]any`).

---

## 4. Verification & fault drills

### 4.1 Cancellation records persisted

Run the worked project; then:

```sql
SELECT id, method, status, in_progress, tried, try_results FROM tcc_snapshots;
-- terminal row: status = 4 (Cancelled), tried carries the cancelled participants,
-- try_results holds each Try's (JSON-typed) result.
```

Commit-path drill: fix the failing participant, rerun — after a committed transaction the
row is *deleted* (coordinator.go), so an empty table after successes is correct, and
`Pending()` returns nothing.

### 4.2 Store persistence across restart (kill -9 mid-transaction)

1. Make the payment participant's Try sleep long enough to act within.
2. Start the app, trigger the transaction, and `kill -9` the process while it runs.
3. Inspect: `tcc_snapshots` holds a `status = 0 (Trying)` row with `tried=["reserve-stock"]`,
   `in_progress="charge-payment"`.
4. Restart the app. The recovery Runner logs `tcc recovery: transaction recovered`, cancel of
   the in-progress participant (nil result) then the tried ones runs, and the row's status
   flips to Cancelled. This sequence is covered by `TestGormStore_EndToEndExecuteThenRecover`
   (store_test.go).
5. Failure sub-drill: if the method is not registered at wiring time, recovery logs
   `tcc recovery: skip method with no registered participants` and the row stays Trying —
   re-declare the transaction and restart again.

### 4.3 Observing recovery in traces

With `tracing=true` + starter-otel, restart-time cancellation emits per-phase child spans
tagged with the transaction id / participant / phase — check your collector after the §4.2
restart.

### 4.4 AutoMigrate fail-fast drill

Point `spring.gorm.mysql.instances.dsn` at a database the user cannot DDL: the store bean
fails at construction (`auto-migrate tcc_snapshots failed`, tag `AppDef`) and startup aborts
— misconfiguration surfaces at boot, not on the first transaction.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No `tcc_snapshots` table, nothing persists | `spring.transaction.tcc.store` missing or not `gorm` — in-memory default Store active | Set it to exactly `gorm`; verify the `create gorm tcc store success` log line at boot. |
| Startup fails: `auto-migrate tcc_snapshots failed` | The autowired `*gorm.DB` lacks DDL rights or the server is unreachable | Grant DDL / fix the driver starter's DSN (starter.go). |
| Crashed transaction never recovers, log says `no registered participants` | Participants not registered at wiring time (registered from a Runner, or method name changed) | Register in bean construction under the *same* method name `GlobalTCC` recorded (recovery.go). |
| Cancel gets `float64` instead of `int`, or `map[string]any` instead of a struct | JSON round-trip on recovery: results come back in their JSON form (store.go) | Keep Try results JSON-friendly (ids, tokens, scalars); the in-progress participant always recovers with nil result. |
| Recovered transaction cancels participants that never tried | — cannot happen: recovery is bounded by the log's `tried` + `in_progress` | If observed, it is a real bug — report it. |
| Multiple `*gorm.DB` instances, TCC log lands in the wrong one | No instance-selection key exists; the container's default instance is always autowired | Restructure beans so the TCC database is the default, or wrap it behind its own starter. |
| Table grows with terminal rows | By design: cancelled/failed rows are kept for inspection; only committed transactions are deleted | Purge audited terminal rows on your own schedule; treat them as an audit trail. |
| Recovery swallows DB errors | A `Pending()` failure only logs (recovery.go); `persist` errors are swallowed by design | Monitor the DB and the `AppDef` logs; do not treat silence as success. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 0 value tags + 1 activation condition key |
| Required | 1 (`spring.transaction.tcc.store=gorm`) |
| Quickstart external deps | 1 (the database behind the gorm driver starter) |
| "Watch out" entries | 8 |

Design suspects:

- Activation needs an extra property while the tcc capability itself activates on import —
  the asymmetry buys explicit Store selection; defensible but worth stating.
- **No example/ and no integration smoke test** wiring the full tcc + gorm store path end to
  end (only store unit tests) → add example/; this document's worked project is
  code-verified only.
- `persist` swallows store errors by design (progress > log completeness) — a durable-store
  outage mid-transaction leaves gaps that recovery cannot distinguish from "participant
  never tried"; no metric exposes Save failures.
- JSON round-trip typing on recovered results is a silent footgun documented only in a code
  comment (store.go).
- Terminal rows are kept forever with no retention story; committed rows are deleted, so
  audit coverage is asymmetric by outcome.
