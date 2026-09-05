# starter-outbox-gorm Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `publish.go`, `store.go`, `model.go`,
`health/health.go`, `schema.json`), the relay core (`go-spring.org/cloud/experimental/outbox`)
and the self-asserting [example/](example/) (`example/check.sh`). The transactional-outbox
*pattern* is standard literature — everything below is go-spring's binding, wiring and
operational increment.

**Activation**: the module registers beans only when a `spring.outbox` property exists
(`gs.OnProperty("spring.outbox")`, starter.go:59; the check is a prefix check, so any
`spring.outbox.*` key activates it). There is no `enabled` key. Instances are multi-named:
one relay per entry under `spring.outbox.<name>.*`.

---

## 1. Complete worked project

A realistic order service that writes orders and publishes domain events atomically, with
health probes and a dead-letter drill. File tree:

```
demo/
├── go.mod
├── main.go
├── order.go
└── conf/
    ├── app.properties
    └── govern.yaml          # not needed by the outbox itself; shown for parity
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-outbox-gorm latest
    go-spring.org/starter-gorm-mysql  latest   // any gorm driver starter; provides the *gorm.DB bean
    go-spring.org/starter-kafka       latest   // any broker starter; registers the "kafka" driver
    go-spring.org/starter-actuator    latest   // optional: probes for the outbox:<name> indicator
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-kafka"
    _ "go-spring.org/starter-outbox-gorm"

    _ "demo/order"
)

func main() { gs.Run() }
```

**order.go** — the only outbox-aware code you write:

```go
package order

import (
    "go-spring.org/spring/gs"
    StarterOutboxGorm "go-spring.org/starter-outbox-gorm"
    "gorm.io/gorm"
)

func init() {
    gs.Provide(newService)
}

type Service struct{ db *gorm.DB }

func newService(db *gorm.DB) *Service { return &Service{db: db} }

// CreateOrder writes the order AND its event in ONE database transaction.
// That shared transaction is the whole point of the pattern: either both rows
// commit, or neither does (verified by example.go's rollback assertion).
func (s *Service) CreateOrder(order *Order, payload []byte) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(order).Error; err != nil {
            return err // rolls back the order AND the outbox row
        }
        return StarterOutboxGorm.Publish(tx, "orders.events", order.ID, payload, nil)
    })
}
```

**conf/app.properties** — the complete, commented outbox surface (timing values here are
production-ish; the example/ uses scaled-down values for a fast smoke run):

```properties
# --- database (driver starter's own keys) ------------------------------------
spring.gorm.db.dataSourceName=user:pass@tcp(127.0.0.1:3306)/demo

# --- outbox: one relay instance per entry under spring.outbox ----------------
# "main" is the instance name → bean name, health indicator "outbox:main".
spring.outbox.main.driver=kafka        # required: driver registered by the broker starter
spring.outbox.main.db=                 # empty → autowire the single *gorm.DB bean
spring.outbox.main.auto-migrate=true   # create outbox_message via gorm at startup
spring.outbox.main.poll-interval=1s    # clamped to >=100ms
spring.outbox.main.batch-size=100      # rows per poll
spring.outbox.main.max-attempts=8      # total attempts before dead-letter
spring.outbox.main.backoff-base=1s     # doubles per failure
spring.outbox.main.backoff-max=1m      # cap on one retry wait
spring.outbox.main.dlq-suffix=.dlq     # "orders.events" → "orders.events.dlq"; "" disables DLQ copy

# --- actuator (health indicator) ---------------------------------------------
spring.actuator.addr=:9370
```

**Verify** (out-of-the-box example — zero external deps, in-memory sqlite + a `mem` driver):

```bash
cd starter/experimental/starter-outbox-gorm/example && ./check.sh
# → "outbox example OK: atomicity, retry, dead-letter all passed"
#   "outbox example smoke test passed"
```

The example self-asserts: a committed transaction delivers exactly its message; a rolled-back
one delivers nothing; a flaky destination succeeds on the last permitted attempt; a poison
destination lands in `poison.dlq` with `x-dlq-retries: 3`; final table states are 2 sent /
1 dead / 0 pending (example.go:159-236).

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-outbox-gorm
  └─ gs.Module(gs.OnProperty("spring.outbox"))                 [starter.go:59]
        │   one pass per entry <name> under ${spring.outbox} (conf.BindEach)
        ├─ Provide(newRelay).Name(<name>).Init(Destroy wired)
        │     args: IndexArg(1, bound Config), IndexArg(2, TagArg(db) *gorm.DB),
        │           IndexArg(3, ValueArg(name))
        │     exported as gs.Rooter                                 [starter.go:61-66]
        └─ Provide(health.Indicator "outbox:<name>")               [starter.go:69-71]
              .Name("outbox:" + name) — mandatory: multi-instance health beans
              need distinct (Name,Type) keys or the container reports duplicates.

gs.Run()
  ├─ config bind: ${spring.outbox.<name>} → Config (value tags; expr: driver != '')
  ├─ bean wiring: *gorm.DB resolved from TagArg(c.DB) — empty db key autowires
  │               the single *gorm.DB bean; named key picks that bean
  ├─ Relay.Init()  [starter.go:97]
  │     1. messaging.GetDriver(driver) — resolved at run time, not construction,
  │        so broker starters registering their driver later still work
  │     2. optional Migrate(db) when auto-migrate=true
  │     3. outbox.NewRelay(gormStore, driver, cfg, logObserver) and the loop
  │        starts on a background goroutine (Init returns immediately)
  ├─ readiness: unaffected — the relay is a worker, not a gs.Server; blocking
  │        on it would defeat the readiness signal (starter.go:55-58 comment)
  └─ on SIGTERM: Relay.Destroy() — see §4.4 for the drain contract.
```

Init failure modes are fatal: unknown driver name or failed auto-migrate log an ERROR with
the instance name and abort startup (starter.go:98-107).

### 2.2 One message, end to end

Write side (`publish.go:42-61`):

1. Your code opens `db.Transaction(...)`; business write + `Publish(tx, dest, key, payload, headers)`
   insert into the SAME transaction. `Publish` marshals headers to JSON and inserts an
   `outbox_row` with `status="pending"`, `next_retry_at=now`, `created_at=now`.
2. Commit — the row becomes visible to the relay only now. Rollback removes it silently.

Relay loop (`cloud/experimental/outbox/relay.go`, driven by starter.go:109-116):

3. **Fetch** — every `poll-interval`, `gormStore.Fetch` selects up to `batch-size` rows with
   `status='pending' AND next_retry_at <= now`, ordered by `id` ascending; on mysql/postgres
   it adds `FOR UPDATE SKIP LOCKED` so concurrent relay instances never share a row
   (store.go:58-74).
4. **Deliver** — for each record (ID order within the batch), the relay lazily opens a
   `messaging.Publisher` per destination (cached for the process lifetime) and publishes
   Key/Payload/Headers. A failing record never aborts the batch — one poisoned message
   must not starve the rest (relay.go:111-127).
5. **Mark** — success: `MarkSent` flips the row to `status='sent'` with `sent_at` (guarded by
   `status='pending'` in the WHERE, so a racing mark can't double-apply). Failure: see §2.3.

If a fetch itself fails, the relay logs one ERROR per poll and keeps polling — a dead
database must not be silent, nor crash the process (relay.go:113-119).

### 2.3 Retry and dead-letter — exact semantics

On a failed attempt (relay.go:164-190):

- `attempts` (already failed count) + 1 < `max-attempts` → `MarkFailed`: `attempts+1`,
  `last_error`, `next_retry_at = now + backoff(attempts)` where backoff = `backoff-base`
  doubled per failure, capped at `backoff-max` (outbox.go:157-166). Observer `OnRetry` fires.
- Attempts exhausted (`>= max-attempts`):
  - `dlq-suffix` non-empty → the relay first publishes a **copy** to `destination + dlq-suffix`
    carrying the original headers plus `x-dlq-error`, `x-dlq-retries` (the attempt count) and
    `x-dlq-key`; only if that copy succeeds does it `MarkDead` (`status='dead'`, `last_error`).
  - `dlq-suffix=""` → straight to `MarkDead`, no copy (an unwatched data path — see §6).
  - If the DLQ publish itself fails, the record is **not** lost: `MarkFailed` keeps it pending
    so the next run retries the whole dead-letter path (relay.go:181-186 — losing a dead
    letter is worse than redelivering).
- Edge: if `MarkSent` fails after a successful broker publish (store down), the record stays
  pending and will be delivered again — this is the at-least-once source; consumers must be
  idempotent or dedupe by Key (relay.go:133-141, outbox.go:22-24).

Lifecycle of a row: `pending → sent | dead`. `sent` rows are never pruned by the starter —
table growth (and archival) is an operator concern.

---

## 3. Per-key behavior reference

All keys live under `spring.outbox.<name>.` — one instance per entry. Defaults from
config.go:29-63; normalization (clamps) from outbox.go:132-153.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `driver` | string | — | **Required** (`expr:"driver != ''"`, config.go:36). Driver name registered by a broker starter (`kafka`, `nats`, …) or your own `messaging.RegisterDriver`. Resolved in Init, so late-registered drivers work. | Missing/unknown → startup fails with "outbox %q: …" ERROR. |
| `db` | string | "" | Names the `*gorm.DB` bean backing the outbox table. Empty autowires the single `*gorm.DB` bean (whatever driver starter provided it). | Name with no such bean → wiring failure at startup. |
| `auto-migrate` | bool | false | `true` runs `db.AutoMigrate(&outboxRow{})` in Init — creates `outbox_message` plus the `(status, next_retry_at)` dispatch index (model.go:40-42,55-57). Off by default: manage the table from the README DDL when schema is migration-tooled. ⚠ auto-migrate does not upgrade a pre-existing table created from an older DDL. | false with no table → every fetch ERRORs (logged each poll, relay keeps running, nothing delivers). |
| `poll-interval` | duration | 1s | Wait between polls when drained. `<=0` → 1s; `<100ms` clamped to 100ms (outbox.go:133-137). | Too low → busy polling against a dead store spams fetch ERRORs; too high → latency after idle. |
| `batch-size` | int | 100 | Max rows per fetch. `<=0` → 100. | Large batches hold row locks (SKIP LOCKED) longer and stretch a drain cycle. |
| `max-attempts` | int | 8 | Total delivery attempts before dead-letter. `0` → 8; negative → 1 (immediate dead-letter on first failure). | 1 with a flaky broker dead-letters instantly. |
| `backoff-base` | duration | 1s | Wait after the first failure; doubled per further failure. `<=0` → 1s. | Together with `backoff-max` sets total retry window: 8 attempts at 1s..1m ≈ 3.5 min before DLQ. |
| `backoff-max` | duration | 1m | Cap on a single backoff. `<=0` → 1m. | — |
| `dlq-suffix` | string | ".dlq" | Appended to destination to derive the dead-letter destination. ⚠ empty string **disables** the DLQ copy — exhausted records go straight to `dead` with no copy anywhere (config.go:59-62). | Empty suffix intended "no suffix", actually silences dead-lettering. |

Coupling notes:

- `db` must point at the **same database** your business transactions write to — the atomicity
  guarantee is "same transaction", which only exists within one database.
- Ordering across batches/restarts is not guaranteed; if the broker is keyed (kafka
  partitions), set `Publish`'s `key` so same-key messages stay ordered (relay.go:34-37).
- These keys are instance-scoped (`spring.outbox.<name>.*`); the starter has no
  top-level wrapper keys (no `${observability:=}`-style absolute keys exist here).

---

## 4. Verification & fault drills

### 4.1 Atomicity

With the example running (or your own service): create one order in a transaction that
commits, and one that the handler rolls back after `Publish`. Observe the broker: only the
committed message arrives; `SELECT status, count(*) FROM outbox_message GROUP BY status`
shows no phantom rows. The example asserts exactly this (example.go:159-172).

### 4.2 Retry with backoff

Point a destination at a briefly unavailable broker (or use the example's `orders.flaky`,
which fails twice then succeeds). Watch the WARN log lines:

```
WARN ... _app_outbox ... outbox: record 3 to "orders.flaky" failed (...), retry at 2026-08-28T10:00:02Z
```

(starter.go:147-150). The `retry at` timestamp walks `backoff-base` doubling. After the
broker recovers, delivery succeeds and the row flips to `sent`.

### 4.3 Dead-letter drill

Publish to a permanently failing destination (example: `poison`). After `max-attempts`
attempts, a copy lands in `poison.dlq` stamped with `x-dlq-error`, `x-dlq-retries`,
`x-dlq-key`; the row becomes `dead`; one ERROR line fires (starter.go:153-156). Verify:

```sql
SELECT id, destination, attempts, last_error FROM outbox_message WHERE status = 'dead';
```

Flip drill: set `max-attempts=1` in config before starting to see immediate dead-lettering.

### 4.4 Shutdown / drain contract

On SIGTERM the container calls `Relay.Destroy()` (starter.go:125-136): it cancels the loop's
context and waits, **bounded by a hardcoded 5s** (`DrainTimeout`, starter.go:122). The loop
stops fetching new batches and finishes delivering the record currently in flight — or marks
it failed for the next run — then returns (relay.go:91-94). On timeout the loop is abandoned
(rows stay pending and resume after restart; a delivery may repeat — at-least-once).

Note: this drain is the relay's own cancellation semantics. Load-balance instance
"Weight=0" drain does **not** apply here — the outbox relay is not registered in any
registry/lb pool; its shutdown path is only `Destroy` as above.

### 4.5 Observability surface

- Log tag `_app_outbox` (starter.go:52). Tune independently:

```properties
logger.outbox.type=Logger
logger.outbox.level=WARN
logger.outbox.tag=_app_outbox
```

- Retry → WARN, dead → ERROR, successful publishes are silent by design — the broker-side
  access log already records them (starter.go:138-156).
- The `outbox.Observer` seam (outbox.go:168-181) receives OnPublished/OnRetry/OnDead
  synchronously. The starter wires a log-only observer; no metrics/otel adapter ships yet —
  bring your own by wrapping `outbox.Relay` directly if needed.

### 4.6 Health

The starter registers an indicator bean named `outbox:<name>` (starter.go:69-71) — the
`.Name()` is mandatory, otherwise two instances collide as duplicate `(Name,Type)` beans.
The probe is a plain `SELECT 1` through the same `*gorm.DB` the relay drains
(health/health.go:29-33): it catches an unreachable database, but does **not** reflect relay
lag, dead-letter depth or pending backlog.

```bash
curl -s :9370/health | jq '.components["outbox:main"]'
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails: `outbox "main": driver "kafka" not found` | broker starter not imported, or wrong name | Import the broker starter or `messaging.RegisterDriver("kafka", …)` before Init runs. |
| Startup fails after auto-migrate error | DB unreachable / insufficient DDL rights | Fix connectivity or pre-create the table from the README DDL and set `auto-migrate=false`. |
| Continuous `fetch failed (keep polling)` ERRORs | table missing (auto-migrate off, DDL not applied) or db down | Apply the DDL; the relay survives but delivers nothing while fetch fails. |
| Messages delivered twice | crash/redelivery between publish and MarkSent — at-least-once by design | Make consumers idempotent or dedupe by `Key`. |
| Rows pile up `pending` | broker down: retries back off up to `backoff-max` | Restore the broker; watch `retry at` log lines. Monitor `SELECT count(*) … WHERE status='pending'`. |
| Rows go `dead` with no DLQ copy | `dlq-suffix=""` (DLQ disabled) | Set a suffix; re-publish dead rows manually if needed. |
| Ordering violated across messages | per-batch ID order only; no cross-batch guarantee | Set a partitioning `Key` at `Publish` and rely on broker key ordering. |
| MySQL 5.7: duplicate deliveries under concurrent relays | 5.7 lacks `FOR UPDATE SKIP LOCKED` (store.go:36-38) | Run a single relay instance per table, or upgrade to mysql 8+. |
| Shutdown log `drain timed out` | in-flight batch exceeded the hardcoded 5s | Benign — rows stay pending and resume after restart. |
| Indicator says UP but nothing delivers | indicator only probes DB connectivity | Check relay logs (`_app_outbox`) and pending-row count instead. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 9 |
| Required | 1 (`driver`) |
| Quickstart external deps | 2 (DB + broker; bundled example uses 0 via sqlite + mem driver) |
| "Watch out" entries | 7 (atomicity requires same-tx; at-least-once; per-batch ordering only; MySQL 5.7 single relay; DLQ-off unwatched; 5s drain cap; sent-row growth) |

Design suspects (for the audit ledger):

1. ~~Index mismatch~~ — FIXED 2026-08-27: gorm model now builds `(status, next_retry_at)`,
   matching the README DDL and `Fetch`'s predicate.
2. ~~Silent `Store.Fetch` errors~~ — FIXED 2026-08-27: fetch failures log an ERROR each poll
   (relay keeps polling, no crash).
3. Corrupt `headers` JSON is silently dropped in `toRecord` (store.go:117-121).
4. `DrainTimeout=5s` hardcoded, not configurable.
5. DLQ disabled (`dlq-suffix=""`) silently routes exhausted rows to `dead` with no copy — an
   unwatched data path.
6. ~~Dead `Relay.pubOnce` field~~ — FIXED 2026-08-27 (removed).
7. Health indicator name suggests outbox health but only probes DB connectivity.
8. The log-only Observer leaves the `outbox.Observer` seam without a shipped metrics/otel
   adapter (observe-messaging bridge is the natural home).
