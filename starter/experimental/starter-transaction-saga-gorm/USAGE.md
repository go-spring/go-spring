# starter-transaction-saga-gorm Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavioral claim below is
verified against the starter source (`starter.go`, `config.go`, `store.go`) plus the
consumers of its bean: starter-transaction-saga (`starter.go`, `recovery.go`,
`config.go`) and the capability core `cloud/experimental/transaction/coordinator.go`.
**Saga pattern semantics (forward steps + reverse compensation, backward recovery) are
[distributed-transaction literature](https://seata.apache.org/docs/user/saga) — this page
covers only the go-spring increment.**

This starter contributes exactly one thing: a durable, gorm-backed `transaction.Store`
for the Saga coordinator, which is what turns on crash recovery. Scope note: the store
persists the saga *log* (snapshots), not business data; compensation of business effects
is still your `Compensate` functions. This module has **no example/** — the worked project
below is code-verified against `store_test.go` (including the end-to-end
execute-then-recover test), not smoke-verified against a running database.

---

## 1. Complete worked project

An order-placement saga over a MySQL business database: reserve stock, charge payment,
and a deliberately failing publish step that forces reverse compensation. The saga log —
including the terminal `Compensated` record — persists in `saga_snapshots` and survives a
restart. File tree:

```
demo/
├── go.mod
├── main.go
├── saga.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring                       v1.3.x
    go-spring.org/starter-gorm-mysql            latest   // provides the *gorm.DB
    go-spring.org/starter-transaction-saga      latest   // coordinator + registry + recovery Runner
    go-spring.org/starter-transaction-saga-gorm latest
    go-spring.org/starter-otel                  latest   // optional: real trace export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-transaction-saga"
    _ "go-spring.org/starter-transaction-saga-gorm"
)

func main() { gs.Run() }
```

**saga.go** — the entire application surface:

```go
package main

import (
    "context"
    "errors"

    "go-spring.org/cloud/experimental/transaction"
    "go-spring.org/spring/gs"
)

// Step registration MUST happen at wiring time (bean construction): the startup
// recovery Runner rebuilds each crashed saga from the StepRegistry keyed by the
// persisted method name, so steps registered later — e.g. from inside a custom
// Runner — may be absent when recovery runs (starter-transaction-saga/starter.go
// package comment, recovery.go:56-63).
func init() {
    gs.Provide(func(coord transaction.Coordinator, reg *transaction.StepRegistry) *OrderSaga {
        reg.Register("OrderService.Place",
            // Step 1: reserve stock (forward); release on compensation.
            transaction.Step{Name: "reserve-stock",
                Action:     func(ctx context.Context) (any, error) { return reserve(ctx, 2) },
                Compensate: func(ctx context.Context, _ any) error { return release(ctx, 2) }},
            // Step 2: charge payment.
            transaction.Step{Name: "charge-payment",
                Action:     func(ctx context.Context) (any, error) { return charge(ctx, 30) },
                Compensate: func(ctx context.Context, _ any) error { return refund(ctx, 30) }},
            // Step 3: publish — FAILS ON PURPOSE, driving reverse compensation
            // of charge-payment then reserve-stock, with every snapshot the
            // coordinator takes persisted to saga_snapshots.
            transaction.Step{Name: "publish-order",
                Action:     func(ctx context.Context) (any, error) { return nil, errors.New("broker unavailable") },
                Compensate: func(ctx context.Context, _ any) error { return nil }},
        )
        return &OrderSaga{coord: coord, reg: reg}
    }).Export(gs.As[gs.Rooter]())
}

type OrderSaga struct {
    coord transaction.Coordinator
    reg   *transaction.StepRegistry
}

func (o *OrderSaga) Run(ctx context.Context) error {
    place := transaction.GlobalTransactional(o.coord, o.reg)

    // The @GlobalTransactional equivalent: on error the coordinator has already
    // compensated every completed step in reverse and written the terminal log.
    return place(ctx, "OrderService.Place", func(ctx context.Context) error {
        return nil // steps run via the registry entry above
    })
}
```

(`reserve`/`charge`/etc. are your ordinary business calls — SQL via the autowired
`*gorm.DB` or RPC clients; Saga semantics are the linked literature.)

**conf/app.properties** — the complete, commented surface:

```properties
# --- datasource (starter-gorm-mysql; provides the autowired *gorm.DB) --------
spring.gorm.mysql.instances.dsn=app:pass@tcp(127.0.0.1:3306)/demo?parseTime=true

# --- Saga durable store (THIS starter's activation key) ----------------------
# Must be exactly "gorm" for this Store to register
# (OnProperty ... HavingValue("gorm"), NO MatchIfMissing — starter.go:49-51).
# With it set, the saga starter's in-memory default Store steps aside
# (OnMissingBean) and the coordinator + recovery Runner consume this one.
spring.transaction.saga.store=gorm

# --- Saga capability (parent starter; tracing/recovery toggles) ---------------
# Default true, shown for discoverability. One otel child span per step phase:
# saga.action <step> / saga.compensate <step>.
spring.transaction.saga.tracing=true
# Startup recovery Runner: scan Pending() and compensate crashed sagas.
spring.transaction.saga.recover-on-start=true

# --- observability (starter-otel, optional) -----------------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**Verify** (against the configured MySQL):

```bash
# after the failing saga run:
mysql> SELECT id, method, status, in_progress, completed FROM saga_snapshots;
# one row: the saga id, "OrderService.Place", status=2 (Compensated); see §4.1 for columns
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gorm-mysql + starter-transaction-saga + starter-transaction-saga-gorm
  ├─ saga starter: StepRegistry, in-memory Store [OnMissingBean],
  │   Coordinator, recovery Runner [cond: enabled + recover-on-start]
  └─ this starter: gs.Provide(newGormStore as transaction.Store)
        [cond: OnProperty("spring.transaction.saga.store").HavingValue("gorm")]
        │
gs.Run()
  ├─ config bind: ${spring.transaction.saga.gorm} → gormConfig (1 value tag)
  ├─ *gorm.DB autowired into newGormStore's second ctor arg
  ├─ store construction: db.AutoMigrate(&sagaSnapshot{}) — creates
  │  saga_snapshots, FAILS FAST (bean error) if it cannot        (starter.go:57-60)
  ├─ ordering: the durable Store wins the saga starter's OnMissingBean slot, so
  │  newCoordinator consumes it (the in-memory default is never built)
  ├─ recovery Runner.Run(): Store.Pending() → for each snapshot, rebuild steps
  │  from the registry by method name and coord.Recover()       (recovery.go:49-73)
  └─ your Rooter/Runner beans run after assembly
```

Construction log line: `gorm saga store created` (tag `AppDef`).

### 2.2 One failing saga — full walk, including persistence points

Cited from `cloud/experimental/transaction/coordinator.go` and `store.go`:

1. **Begin** — `GlobalTransactional(coord, reg)` looks up the method's steps and calls
   `coord.Execute`; the coordinator writes the initial `StatusRunning` snapshot via
   `persistRunning` — the saga is durable from its first step (coordinator.go:225-230).
2. **reserve-stock runs** — on success the coordinator upserts the running snapshot with
   `completed=["reserve-stock"]`, `step_results={"reserve-stock": ...}` via
   `Store.Save` (an `OnConflict UpdateAll` upsert, store.go:65-73). One row per saga id,
   constantly overwritten — `saga_snapshots` is a log of *current state*, not an
   append-only history.
3. **charge-payment runs** — same cadence: snapshot now `completed=[reserve-stock,
   charge-payment]`. Each `persistRunning` write is a save point a crash can resume from.
4. **publish-order FAILS** — `errors.New("broker unavailable")`. The coordinator
   compensates in reverse: `refund` (charge-payment), then `release` (reserve-stock).
   Each compensation is visible in traces (`saga.compensate <step>` spans) and updates
   the same snapshot row.
5. **Terminal record** — `finish`: a *committed* saga's row is deleted (work done, nothing
   to recover); a *compensated or failed* saga's row is **kept** with the terminal status
   for operator inspection (coordinator.go:235-244). After our run, the table holds one
   `Compensated` row — the persisted proof of compensation.
6. **Crash recovery (restart)** — the recovery Runner reads `Pending()` (every
   `StatusRunning` row — store.go:94-111), rebuilds the step list from the registry by
   the persisted method name, and `coord.Recover` compensates from the log's position:
   the in-progress step first (with a nil result — sidestepping the JSON round-trip
   caveat), then each completed step in reverse. Unreached steps are not compensated
   (verified by `TestGormStore_EndToEndExecuteThenRecover`: order `!b`, `!a`, never
   `!c`). A method with no registered steps is logged and skipped; an individual
   recovery error is logged and does not abort the remaining sagas, and the Runner never
   fails startup (recovery.go:45-73).

Note the failure-durability asymmetry: persistence errors in `persistRunning` are
*intentionally swallowed* — the saga already made progress and failing the operation on a
log write would be worse than a gap in the log (coordinator.go:218-224).

---

## 3. Per-key behavior reference

`grep -rhoE 'value:"[^"]+"' --include='*.go'` on this module yields **zero** tags;
the activation key below is a bean *condition* property (grep-invisible) and is included
because it is the only way to switch this Store on. This module now owns **no value-tag
keys at all** — the dead `spring.transaction.saga.gorm.db` key (bound but never read) was
removed; the `*gorm.DB` is always the container's default instance.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.transaction.saga.store` | string | (unset) | **Activation key** (condition, not a value tag). Must be exactly `gorm` — `OnProperty ... HavingValue("gorm")`, no `MatchIfMissing` (starter.go:49-51). | Any other value / unset → this Store never registers and the saga starter's in-memory default stays: no `saga_snapshots` table, **no crash recovery**, silently. |

Parent-starter keys that govern the machinery this Store feeds (documented in
[starter-transaction-saga](../starter-transaction-saga), listed here because they change
this Store's observable behavior): `spring.transaction.saga.enabled` (default true),
`spring.transaction.saga.tracing` (default true — `saga.action/compensate <step>` spans,
attributes `saga.id`/`saga.step`/`saga.phase`), `spring.transaction.saga.recover-on-start`
(default true — the Runner that makes this Store meaningful).

**Table schema** (created by the constructor's `AutoMigrate`, backend-agnostic — text and
int columns only, no dialect-specific types): `saga_snapshots(id PK, method, status int
indexed, in_progress, completed text, step_results text, updated_at)`
(store.go:35-46). `completed` / `step_results` are JSON-encoded
(`[]string` / `map[string]any`).

---

## 4. Verification & fault drills

### 4.1 Compensation records persisted

Run the worked project; then:

```sql
SELECT id, method, status, in_progress, completed, step_results FROM saga_snapshots;
-- terminal row: status = 2 (Compensated), completed carries the compensated steps,
-- step_results holds each Action's (JSON-typed) result.
```

Commit-path drill: fix step 3, rerun — after a committed saga the row is *deleted*
(coordinator.go:239-241), so an empty table after successes is correct, and `Pending()`
returns nothing.

### 4.2 Store persistence across restart (kill -9 mid-saga)

1. Make step 2 (`charge-payment`) sleep long enough to act within.
2. Start the app, trigger the saga, and `kill -9` the process while step 2 runs.
3. Inspect: `saga_snapshots` holds a `status = 0 (Running)` row with
   `completed=["reserve-stock"]`, `in_progress="charge-payment"`.
4. Restart the app. The recovery Runner logs
   `saga recovery: saga "<id>" recovered with status Compensated`, compensation of the
   in-progress step (nil result) then the completed ones runs, and the row's status
   flips to Compensated. This exact sequence is covered by
   `TestGormStore_EndToEndExecuteThenRecover` (store_test.go:93-149).
5. Failure sub-drill: if the method is not registered at wiring time, recovery logs
   `no steps registered for method ...; skipping` and the row stays Running — re-declare
   the saga definition and restart again.

### 4.3 Observing recovery in traces

With `tracing=true` + starter-otel, restart-time compensation emits
`saga.compensate <step>` spans tagged `saga.id` / `saga.step` / `saga.phase`, children of
whatever context the Runner passes — check your collector after the §4.2 restart.

### 4.4 AutoMigrate fail-fast drill

Point `spring.gorm.mysql.instances.dsn` at a database the user cannot DDL: the store bean fails at
construction (`auto-migrate saga_snapshots failed`, tag `AppDef`) and startup aborts —
misconfiguration surfaces at boot, not on the first saga.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No `saga_snapshots` table, nothing persists | `spring.transaction.saga.store` missing or not `gorm` — in-memory default Store active | Set it to exactly `gorm`; verify the `gorm saga store created` log line at boot. |
| Startup fails: `auto-migrate saga_snapshots failed` | The autowired `*gorm.DB` lacks DDL rights or the server is unreachable | Grant DDL / fix the driver starter's DSN (starter.go:57-60). |
| Crashed saga never recovers, log says `no steps registered for method` | Steps not registered at wiring time (registered from a Runner, or method name changed) | Register in bean construction under the *same* method name `GlobalTransactional` recorded (recovery.go:56-63). |
| Compensate gets `float64` instead of `int`, or `map[string]any` instead of a struct | JSON round-trip on recovery: results come back in their JSON form (store.go:52-57) | Keep Action results JSON-friendly (ids, tokens, scalars); the in-progress step always recovers with nil result. |
| Recovered saga compensates steps that never ran | — cannot happen: recovery is bounded by the log's `completed` + `in_progress` (verified by unit test) | If observed, it is a real bug — report it. |
| Multiple `*gorm.DB` instances, saga log lands in the wrong one | No instance-selection key exists (the former dead `db` key was removed); the container's default instance is always autowired | Restructure beans so the saga database is the default, or wrap it behind its own starter. |
| Table grows with terminal rows | By design: compensated/failed rows are kept for inspection; only committed sagas are deleted | Purge audited terminal rows on your own schedule; treat them as an audit trail. |
| Recovery swallows DB errors | `Pending()` failure only logs and returns nil (recovery.go:51-54); `persistRunning` errors are swallowed by design | Monitor the DB and the `AppDef` logs; do not treat silence as success. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 0 value tags + 1 activation condition key (the former dead `db` tag was removed) |
| Required | 1 (`spring.transaction.saga.store=gorm`) |
| Quickstart external deps | 1 (the database behind the gorm driver starter) |
| "Watch out" entries | 6 |

Design suspects (kept from the previous audit + new):

- Activation needs an extra property while every other starter in this family activates
  on import — the asymmetry buys explicit Store selection; defensible but worth stating.
- ~~`spring.transaction.saga.gorm.db` is bound but never used~~ — resolved: the dead key
  was dropped (multi-instance selection remains unimplemented; the prefix is kept for
  future store options).
- **No example/ and no integration smoke test wiring the full saga + gorm store path end
  to end** (only store unit tests) → add example/; this document's worked project is
  code-verified only.
- `persistRunning` swallows store errors by design (progress > log completeness) — a
  durable-store outage mid-saga leaves gaps that recovery cannot distinguish from
  "step never ran"; no metric exposes Save failures.
- JSON round-trip typing on recovered results is a silent footgun documented only in a
  code comment (store.go:52-57).
- Terminal rows are kept forever with no retention story; committed rows are deleted, so
  audit coverage is asymmetric by outcome.
