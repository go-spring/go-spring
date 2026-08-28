# starter-transaction-tcc Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `recovery.go`, `starter_test.go`), the
runnable [example/](example/) (`bash example/check.sh`) and [example-otel/](example-otel/)
(Jaeger via docker-compose), and the wrapped coordinator in
[`go-spring.org/cloud/experimental/transaction/tcc`](../../../cloud/experimental/transaction/tcc)
(`tcc.go`, `coordinator.go`, `global.go`, `store.go`); tracing via
[`cloud/observe/transaction/observer.go`](../../../cloud/observe/transaction/observer.go).
**TCC pattern semantics (Try/Confirm/Cancel, and the participant obligations — idempotence,
empty rollback, anti-hanging) are distributed-transaction literature** — see
[Seata TCC mode](https://seata.apache.org/docs/user/anchor?version=1.7.0) and the README's
"Participant obligations"; this page covers only the go-spring increment.

**Activation**: a blank import is enough — every key defaults on (`MatchIfMissing`). No
activation key; `spring.transaction.tcc.enabled=false` is the opt-OUT. Contributor archetype:
no port, no server, only beans. The `spring.transaction` namespace is shared with
`starter-transaction-saga`, so Saga and TCC can run side by side without collision.

---

## 1. Complete worked project

A one-process "order service" reserving stock and freezing balance as ONE TCC transaction.
The second participant's Try deliberately fails (amount too large), so the first participant's
reservation is cancelled — the exact scenario [example/example.go](example/example.go) asserts.
File tree:

```
demo/
├── go.mod
├── main.go
├── order/
│   └── order.go        # ledgers + OrderService + self-test
└── conf/
    └── app.properties
```

**go.mod** (deps that matter — mirrors [example/go.mod](example/go.mod)):

```
require (
    go-spring.org/spring                v1.3.x
    go-spring.org/cloud                 latest  // tcc coordinator lives here
    go-spring.org/starter-transaction-tcc latest
    go-spring.org/starter-otel          latest  // optional: real trace export
)
```

**main.go**:

```go
package main

import (
    _ "demo/order"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"          // tracing globals (optional)
    _ "go-spring.org/starter-transaction-tcc"
)

func main() { gs.Run() }
```

**order/order.go** — the entire TCC surface (abridged from the shipped example; the ledger
implementation there is the reference):

```go
package order

import (
    "context"
    "errors"
    "sync"

    "go-spring.org/cloud/experimental/transaction/tcc"
    "go-spring.org/spring/gs"
)

// A TCC resource: an `available` pool plus a per-txID `frozen` reservation.
// Try moves value into frozen (reserved, NOT business-visible as spent);
// Confirm drops the frozen amount (spend is final); Cancel returns it.
type ledger struct {
    mu        sync.Mutex
    available int
    frozen    map[string]int // keyed by tx id -> Cancel idempotent, Try anti-hanging
}

func (l *ledger) try(txID string, n int) error {
    l.mu.Lock(); defer l.mu.Unlock()
    if _, done := l.frozen[txID]; done { return nil } // replay = no double reserve
    if l.available < n { return errors.New("insufficient") }
    l.available -= n
    l.frozen[txID] = n
    return nil
}

func (l *ledger) confirm(txID string) error { // idempotent: missing = already confirmed
    l.mu.Lock(); defer l.mu.Unlock()
    delete(l.frozen, txID)
    return nil
}

func (l *ledger) cancel(txID string) error { // idempotent + empty-rollback safe
    l.mu.Lock(); defer l.mu.Unlock()
    if n, ok := l.frozen[txID]; ok {
        l.available += n
        delete(l.frozen, txID)
    }
    return nil
}

type OrderService struct {
    Coord tcc.Coordinator           `autowire:""`  // the starter's bean
    Reg   *tcc.ParticipantRegistry  `autowire:""`  // the starter's registry bean

    stock, balance *ledger
}

func init() {
    gs.Provide(func() *OrderService {
        s := &OrderService{stock: newLedger(10), balance: newLedger(100)}

        // Register participants at WIRING TIME (bean construction) under a
        // logical method name — recovery looks them up by exactly this key
        // (recovery.go). All three phases are REQUIRED: a nil one is rejected
        // by the coordinator BEFORE any side effect (validate).
        // Phases read the tx id from the ctx the coordinator hands them — the
        // same ctx place() built with WithTransactionID (helper `txid` below).
        s.Reg.Register("OrderService.Place",
            tcc.Participant{
                Name: "ReserveStock",
                Try:     func(ctx context.Context) (any, error) { return nil, s.stock.try(txid(ctx), 2) },
                Confirm: func(ctx context.Context, _ any) error { return s.stock.confirm(txid(ctx)) },
                Cancel:  func(ctx context.Context, _ any) error { return s.stock.cancel(txid(ctx)) },
            },
            tcc.Participant{
                Name:    "FreezeBalance",
                Try:     func(ctx context.Context) (any, error) { return nil, s.balance.try(txid(ctx), 999) }, // DELIBERATE failure
                Confirm: func(ctx context.Context, _ any) error { return s.balance.confirm(txid(ctx)) },
                Cancel:  func(ctx context.Context, _ any) error { return s.balance.cancel(txid(ctx)) },
            },
        )
        return s
    }).Export(gs.As[gs.Rooter]())
}

func txid(ctx context.Context) string {
    id, _ := tcc.TransactionIDFromContext(ctx)
    return id
}
```

(In the shipped example `place` instead builds the participants inline per invocation, closing
over the `txID` parameter — both shapes are valid. The registry + `tcc.GlobalTCC(coord, reg)`
shape shown here is what makes the method recoverable by name; note that during RECOVERY the
Runner's ctx does not carry the tx id, so production Confirm/Cancel should key off the Try
result or their own reservation table rather than the context.)

Run path (self-test, mirrors `runTest` in the example; amounts are parameters here, while the
registry block above fixes the drill's 2/999 — pick one shape and use it for both paths):

```go
func (s *OrderService) place(ctx context.Context, txID string, qty, cost int) (tcc.Result, error) {
    ctx = tcc.WithTransactionID(ctx, txID) // idempotency key from the edge
    return s.Coord.Execute(ctx, tcc.Transaction{
        ID:     txID,
        Method: "OrderService.Place", // recovery key — must match the registration
        // Participants as registered above, with qty/cost closed over instead
        // of the drill's fixed 2/999 (shipped example's inline shape):
        //   ReserveStock  -> s.stock.try(txID, qty)   / confirm / cancel
        //   FreezeBalance -> s.balance.try(txID, cost) / confirm / cancel
        Participants: inlineParticipants(s, txID, qty, cost),
    })
}

// Path 1 — commit: place(ctx, "order-commit", 3, 60) -> StatusCommitted,
//   stock 10->7, balance 100->40, nothing frozen.
// Path 2 — rollback: place(ctx, "order-rollback", 2, 999) -> FreezeBalance's
//   Try fails with "insufficient"; the coordinator cancels the TRIED
//   participants in reverse — ReserveStock's reservation is released; both
//   ledgers back at 7/40 with 0 frozen.
```

**conf/app.properties** — the complete, commented surface (copy of
[example/conf/app.properties](example/conf/app.properties) plus tracing):

```properties
# --- tcc ---------------------------------------------------------------------
# All three keys default on; a blank import is enough. Shown for discoverability.
spring.transaction.tcc.enabled=true
spring.transaction.tcc.tracing=true

# Startup recovery scan. ⚠ No-op with the in-memory default Store (a restart
# loses the log); real only once a durable tcc.Store bean exists. NOTE: unlike
# saga (starter-transaction-saga-gorm), NO durable-Store starter ships for TCC
# today — contribute your own tcc.Store bean to make this key meaningful.
spring.transaction.tcc.recover-on-start=true

# --- observability (starter-otel) — full working copy: example-otel/conf ------
spring.observability.enable=true
spring.observability.service-name=tcc-demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

**Expected output** (failure drill — `place(ctx, "order-rollback", 2, 999)`):

```
... tcc coordinator created tracing=true
commit path OK: Committed                    (path 1 ran first in the self-test)
rollback path OK: Cancelled - tcc: participant FreezeBalance Try: insufficient
```

`place` returns `err = "insufficient"`, `res.Status = tcc.StatusCancelled`,
`res.Errors = [{Participant: FreezeBalance, Phase: Try, Err: insufficient}]`, and both ledgers
are exactly as they were before path 2 (stock 7/0, balance 40/0) — the ReserveStock reservation
was released by Cancel.

**Verify**:

```bash
bash example/check.sh          # shipped smoke: runs both paths, self-asserts, exit 0
# trace variant:
docker compose -f example-otel/docker-compose.yml up -d   # Jaeger :4317/:16686
cd example-otel && go run .    # asserts spans reach Jaeger, self-exits
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-transaction-tcc
  ├─ gs.Provide(tcc.NewParticipantRegistry)             [condition: enabled]
  ├─ gs.Provide(MemoryStore as tcc.Store)               [enabled + OnMissingBean[Store]]
  ├─ gs.Provide(newCoordinator)                         [enabled; exported as Coordinator]
  └─ gs.Provide(newRecoveryRunner)                      [enabled + recover-on-start; exported as gs.Runner]
        │
gs.Run()
  ├─ config bind: ${spring.transaction.tcc} → Config (3 value tags, all := defaults)
  ├─ bean wiring: Store autowired into the coordinator; registry + Store +
  │   Coordinator autowired into the recovery Runner
  ├─ Rooter/bean construction: YOUR code calls reg.Register(...) here — wiring-time
  │   registration is a hard requirement (recovery rebuilds by method name)
  ├─ log: "tcc coordinator created tracing=<v>"
  ├─ Runners execute (recovery Runner is a no-op unless a durable Store holds
  │   non-terminal snapshots) — never fails startup
  └─ on SIGTERM: standard container shutdown; no tcc-specific drain
```

A contributed durable `tcc.Store` bean (the only `tcc.Store` in the container) takes over both
the coordinator's log and the recovery scan — same seam as saga's gorm store, but today you
must write it yourself.

### 2.2 One transaction, step by step — including the failure path

`place(ctx, "order-rollback", 2, 999)` with participants [ReserveStock, FreezeBalance(fails)]:

1. **validate first**: the coordinator rejects a participant with a missing Name, a duplicate
   Name, or a nil Try/Confirm/Cancel BEFORE any side effect (coordinator.go `validate` — "a
   missing one is a programming error caught before any side effect"). Result status
   `CancelFailed`, error returned.
2. **Try ReserveStock**: the coordinator persists intent FIRST — snapshot `{Status: Trying,
   InProgress: "ReserveStock"}` (a crashed Try may have partially reserved, so recovery must
   know to cancel this participant) — then runs Try. On success the snapshot is re-saved with
   the participant folded into `Tried` and `InProgress` cleared.
3. **Try FreezeBalance fails** ("insufficient"): the failing error is appended to
   `Result.Errors` first; then `cancel` runs the TRIED participants in REVERSE try order —
   ReserveStock.Cancel(txID) releases the reservation. Cancel receives Try's recorded value
   (nil here); participants must tolerate nil (empty rollback).
4. **finish**: a committed transaction's log is DELETED; a cancelled/failed one is KEPT with the
   terminal status for inspection.
5. `Execute` returns `(Result{Status: Cancelled, Errors:[…]}, theTryError)`.

The success path differs at the decision point: after every Try succeeds, the coordinator
durably records `StatusConfirming` — the commit decision — and ONLY THEN confirms every tried
participant in forward order. A Confirm failure does NOT return an error from `Execute`
("a confirm failure is not a try error"); it surfaces as `StatusConfirmFailed` plus
`Result.Errors` — the signal for manual intervention, since the TCC contract expects
Confirm/Cancel to eventually succeed (give them a non-zero `Participant.Retry`).

Design rationale worth knowing: persistence errors during the in-flight log writes are
intentionally swallowed ("failing the whole operation on a log write would be worse than a gap
in the log", coordinator.go); retries reuse `resilience.Policy` via a "default" executor so TCC
phase retries and outbound resilience share one knob set.

---

## 3. Per-key behavior reference

Exactly three keys (verified: `grep -rhoE 'value:"[^"]+"' … | sort -u` → 3 tags). All are
TOP-LEVEL absolute keys under `spring.transaction.tcc` — one `Config` instance, no multi-instance
group.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.transaction.tcc.enabled` | bool | `true` | `OnProperty(...).HavingValue("true").MatchIfMissing()` — absent means ON. `false` imports the module without contributing any bean. | `false` by accident → no Coordinator bean → autowire of `tcc.Coordinator` fails at wiring (visible). |
| `spring.transaction.tcc.tracing` | bool | `true` | Attaches `transactionobserve.TccObserver{}` at construction → one otel child span per participant phase (`tcc.try/confirm/cancel <name>`) on the starter-otel globals. No-op without starter-otel. | Expecting traces without starter-otel → nothing, and no warning anywhere. |
| `spring.transaction.tcc.recover-on-start` | bool | `true` | Registers the recovery `gs.Runner`: scans `Store.Pending()` for NON-terminal snapshots (`Trying`/`Confirming`/`Cancelling`) and drives each to its decided outcome — forward Confirm for `Confirming`, backward Cancel otherwise. ⚠ **Coupled to the store seam AND currently dead**: no durable TCC Store starter ships; with the in-memory store the scan is always empty after a restart. Contribute your own `tcc.Store` bean to activate it. | `false` with a durable store → crash-stranded transactions never resolved (silently inconsistent); `true` without one → no-op, no warning. |

Related keys owned by OTHER modules: `spring.observability.*` (starter-otel). Note there is NO
`spring.transaction.tcc.store` key in code — the README previously claimed one; selection is by
contributing a `tcc.Store` bean, not by a property (see §6).

---

## 4. Verification & fault drills

### 4.1 Commit and cancel drills (shipped)

```bash
bash example/check.sh
```

The example self-asserts both paths (commit: ledgers drop, nothing frozen; rollback: failing
Try + reverse Cancel, ledgers unchanged) and exits non-zero on any deviation; grep the run for
`commit path OK|rollback path OK` to see both outcomes.

### 4.2 Cancel-on-confirm-failure drill

Make the second participant's `Confirm` return an error and run a fully-tryable transaction
(e.g. cost 60): every Try succeeds, the commit decision is recorded (`StatusConfirming`), the
failing Confirm downgrades the result to `StatusConfirmFailed`, the FIRST participant stays
confirmed (Confirm runs forward and does not unwind — there is no "cancel after confirm"),
and `Execute` returns a NIL error with the failure in `Result.Errors`. This asymmetry is the
TCC contract: Confirm must eventually succeed (retry it, alert on it), because Cancel cannot
undo a confirmed participant.

### 4.3 Durability boundary — read this before relying on recovery

**TCC as shipped here is in-memory and non-durable.** The default `tcc.Store` is a
process-local map: a crash wipes the TCC log together with the coordinator state, so
the recovery Runner (`spring.transaction.tcc.recover-on-start`, default on) is a no-op
after every restart — there is nothing to scan. The Confirm/Cancel decisions of a
crashed transaction are simply gone, and half-tried participants are left for the
business side to reconcile.

If you need crash-recoverable distributed transactions today, use **Saga** with
[starter-transaction-saga-gorm](../starter-transaction-saga-gorm)
(`spring.transaction.saga.store=gorm`), whose durable saga log drives the same-style
startup recovery out of the box. Making TCC itself durable requires contributing your
own `tcc.Store` bean (next section) — no durable TCC store ships in this repo.

### 4.4 Recovery-on-restart drill (requires your own durable store)

1. Contribute a durable `tcc.Store` (mirror
   [starter-transaction-saga-gorm](../starter-transaction-saga-gorm): a bean exported as
   `tcc.Store`, AutoMigrating a `tcc_snapshots`-style table). The in-memory default steps aside
   via `OnMissingBean`.
2. Kill -9 mid-transaction. Three crash points recover differently:
   - crashed in **Try** (`Trying`, `InProgress` set): recovery cancels backward; the in-flight
     participant is cancelled FIRST with a nil Try result (empty rollback), then the tried ones
     in reverse;
   - crashed after the **commit decision** (`Confirming`): recovery confirms FORWARD — the
     durable decision must not be second-guessed; Confirm's required idempotence makes the
     replay safe (pinned by `TestRecoveryRunner_ConfirmsPendingCommit`);
   - crashed during **Cancel** (`Cancelling`): recovery resumes cancelling backward.
3. Watch: `tcc recovery: transaction "tx-N" recovered with status Committed|Cancelled`.
   Committed transactions DELETE their log; a method no longer registered logs
   `no participants registered for method ... skipping`.
4. The Runner never fails startup; per-transaction errors are logged and skipped.

### 4.5 Observing

- Traces (needs starter-otel): spans `tcc.try <participant>` / `tcc.confirm <…>` /
  `tcc.cancel <…>` per phase, attributes `tcc.id`, `tcc.participant`, `tcc.phase`; failures
  recorded with error status. [example-otel](example-otel/) proves the chain end-to-end against
  Jaeger (`docker compose up -d`, then `go run .` — it queries Jaeger's API and asserts traces
  for service `transaction-tcc-otel-example`).
- Logs: tag `AppDef`; lines `tcc coordinator created tracing=…` and the `tcc recovery: …`
  family.
- No metrics, no health indicator (see §6).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Autowire error on `tcc.Coordinator` | `enabled=false`, or the condition excluded the bean | Remove the disable; blank import is default-on. |
| `tcc: participant "X" must define Try, Confirm and Cancel` (before anything ran) | A nil phase — the coordinator's `validate` rejects it pre-side-effect | Define all three; unlike a Saga step, TCC has no optional phase. |
| Cancel didn't run after a crash | In-memory default store — the restart lost the log | Contribute a durable `tcc.Store` bean; `recover-on-start` alone does nothing. |
| `tcc recovery: no participants registered for method "X"` | Participants registered from a Runner / after startup, or method-name drift | Register at bean-construction time under the exact persisted `Method` string. |
| Concurrent transactions double-reserve | Reservations not keyed by tx id → Try replay isn't a no-op (anti-hanging violated) | Key every reservation by the transaction id (`WithTransactionID`), as the example's ledger does. |
| `StatusConfirmFailed` with nil error from `Execute` | A Confirm ultimately failed — by design not a try error | Retry/alert on Confirm (`Participant.Retry` non-zero); inspect `Result.Errors`; no Cancel-after-Confirm exists. |
| Cancel errors on "already released" | Cancel not idempotent / not empty-rollback-safe | Make Cancel tolerate a missing reservation (nil tried value) — see the example's `cancel`. |
| No spans despite `tracing=true` | starter-otel not imported / observability off | Import starter-otel and set `spring.observability.*`. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 3 (all bool, all default-on) |
| Required | 0 |
| Quickstart external deps | 0 (Jaeger only for example-otel) |
| "Watch out" entries | 5 |

Design suspects (audit ledger; originals kept, new ones added):

- README previously claimed a durable-Store selection key `spring.transaction.tcc.store`; no
  code reads it and no TCC durable-Store starter exists → README fixed to "contribute your own
  `tcc.Store` bean" (kept).
- README said "two beans"; the container actually holds four (registry, in-memory store,
  coordinator, recovery Runner) → README fixed (kept, count corrected).
- Package doc in starter.go likewise says "two beans" (lines 24-31) while `init()` registers
  four → same drift as the README, fix the comment.
- `recover-on-start` is dead by default and there is no shipped durable store (asymmetric with
  saga's `-gorm` starter) → either contribute a tcc-gorm starter or document the seam more
  loudly.
- No metrics and no health indicator: a transaction stuck `ConfirmFailed`/`CancelFailed` is
  invisible except by reading kept store rows manually → consider a gauge/health check on
  non-terminal or failed snapshots.
- Persistence errors during the in-flight log are swallowed by design (coordinator.go
  `persist`) → durable-store implementations must self-monitor; consider a warn log.
