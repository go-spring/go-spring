# starter-transaction-saga Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `recovery.go`, `starter_test.go`) and the
wrapped coordinator in [`go-spring.org/cloud/experimental/transaction`](../../../cloud/experimental/transaction)
(`coordinator.go`, `global.go`, `store.go`, `transaction.go`), plus
[`cloud/observe/transaction/observer.go`](../../../cloud/observe/transaction/observer.go) for tracing.
**Saga pattern semantics (forward steps + reverse compensation, when to pick Saga over TCC/AT) are
distributed-transaction literature** — see [Seata Saga mode](https://seata.apache.org/docs/user/saga)
and the README comparison table; this page covers only the go-spring increment.

**Activation**: a blank import is enough — every key defaults on (`MatchIfMissing`). There is no
activation key; `spring.transaction.saga.enabled=false` is the opt-OUT. This is a Contributor-archetype
starter: no port, no server, only beans.

> ⚠ **No runnable example exists in this module** — there is no `example/` directory (unlike
> starter-transaction-tcc). The worked project below is built from source-verified behavior and
> the sibling tcc example's structure; it is compile/logic-verified against the coordinator and
> unit tests (`starter_test.go`), but has NOT been smoke-run as a shipped example. See §6.

---

## 1. Complete worked project

A one-process "order gateway" driving three downstream effects (inventory, payment, notification)
as one Saga. The third step deliberately fails, so the first two are compensated in reverse —
the output lines you should see are shown after the code.

```
demo/
├── go.mod
├── main.go
├── order/
│   └── order.go        # steps + OrderService + self-test
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring                 v1.3.x
    go-spring.org/cloud                  latest  // transaction coordinator lives here
    go-spring.org/starter-transaction-saga latest
    go-spring.org/starter-otel           latest  // optional: real trace export
)
```

**main.go**:

```go
package main

import (
    _ "demo/order"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"           // tracing globals (optional)
    _ "go-spring.org/starter-transaction-saga"
)

func main() { gs.Run() }
```

**order/order.go** — the entire Saga surface:

```go
package order

import (
    "context"
    "errors"
    "sync"

    "go-spring.org/cloud/experimental/transaction"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

// Two toy downstreams. In a real deployment these are HTTP/MQ calls to other
// services; what makes this a Saga is that each Action takes REAL effect
// immediately and is undone by a business-level Compensate.
type inventory struct {
    mu       sync.Mutex
    deducted map[string]int // keyed by saga id -> compensation is idempotent
}

func (i *inventory) deduct(id string, n int) error {
    i.mu.Lock(); defer i.mu.Unlock()
    if _, done := i.deducted[id]; done { return nil } // replay-safe (anti-hanging)
    i.deducted[id] = n
    log.Info(context.Background(), log.TagAppDef, "inventory: deducted %d for %s", n, id)
    return nil
}

func (i *inventory) restock(id string) error {
    i.mu.Lock(); defer i.mu.Unlock()
    if n, ok := i.deducted[id]; ok {          // idempotent: missing = already undone
        delete(i.deducted, id)
        log.Info(context.Background(), log.TagAppDef, "inventory: restocked %d for %s", n, id)
    }
    return nil
}

type payment struct {
    mu    sync.Mutex
    spent map[string]int
}

func (p *payment) charge(id string, amount int) (string, error) {
    p.mu.Lock(); defer p.mu.Unlock()
    if _, done := p.spent[id]; done { return "", nil }
    p.spent[id] = amount
    log.Info(context.Background(), log.TagAppDef, "payment: charged %d for %s", amount, id)
    return "pay:" + id, nil // token handed to Compensate via StepResults
}

func (p *payment) refund(_ context.Context, result any) error {
    // result is the value Action returned (the payment token); in real code the
    // refund API keys on it. Here we key by the token's id suffix.
    log.Info(context.Background(), log.TagAppDef, "payment: refunded %v", result)
    return nil
}

type OrderService struct {
    Coord transaction.Coordinator `autowire:""` // the starter's bean
    Reg   *transaction.StepRegistry `autowire:""`

    inv *inventory
    pay *payment
}

func init() {
    gs.Provide(newOrderService).Export(gs.As[gs.Rooter]())
}

func newOrderService() *OrderService {
    s := &OrderService{inv: &inventory{deducted: map[string]int{}},
        pay: &payment{spent: map[string]int{}}}

    // Register steps at WIRING TIME (bean construction) under a logical method
    // name — the recovery Runner looks them up by exactly this key (recovery.go).
    // Step 3 fails on purpose for the drill in §4.
    s.Reg.Register("OrderService.Place",
        transaction.Step{
            Name: "DeductInventory",
            // Actions/Compensates read the saga id from the context the
            // coordinator hands them — the same ctx place() built with WithSagaID.
            Action: func(ctx context.Context) (any, error) {
                id, _ := transaction.SagaIDFromContext(ctx)
                return nil, s.inv.deduct(id, 2)
            },
            Compensate: func(ctx context.Context, r any) error {
                id, _ := transaction.SagaIDFromContext(ctx)
                return s.inv.restock(id)
            },
        },
        transaction.Step{
            Name: "ChargePayment",
            Action: func(ctx context.Context) (any, error) {
                id, _ := transaction.SagaIDFromContext(ctx)
                return s.pay.charge(id, 50)
            },
            Compensate: func(ctx context.Context, r any) error { return s.pay.refund(ctx, r) },
        },
        transaction.Step{
            Name:   "NotifyUser", // fails -> triggers compensation of 2 then 1
            Action: func(context.Context) (any, error) { return nil, errors.New("notify: SMTP unavailable") },
            // No Compensate needed: this step never succeeds. (A nil Compensate on
            // a step that DID succeed is a compensation failure — see §5.)
        },
    )
    return s
}

func (s *OrderService) place(ctx context.Context, id string) (transaction.Result, error) {
    ctx = transaction.WithSagaID(ctx, id) // idempotency key from the edge
    steps, _ := s.Reg.Lookup("OrderService.Place")
    // Execute is the runtime entry point; Recover (§2.2/§4.3) is driven by the
    // starter's recovery Runner, not by application code.
    return s.Coord.Execute(ctx, transaction.Saga{ID: id, Method: "OrderService.Place", Steps: steps})
}
```

The decorator alternative — the no-reflection `@GlobalTransactional(SAGA)` — wraps `place` so the
method name drives the registry lookup (`transaction.GlobalTransactional(coord, reg)`); an
unregistered method calls `proceed` untouched. Both paths share the same coordinator bean.

Self-test main pattern (copy from
[starter-transaction-tcc/example/example.go](../starter-transaction-tcc/example/example.go)):
run `place` once in a goroutine after 500ms, assert `err != nil` and
`res.Status == transaction.StatusCompensated`, then `syscall.Kill(os.Getpid(), syscall.SIGTERM)`.

**conf/app.properties** — the complete, commented surface:

```properties
# --- saga --------------------------------------------------------------------
# All three default on; a blank import is enough. Shown for discoverability.
spring.transaction.saga.enabled=true
spring.transaction.saga.tracing=true

# Startup recovery scan. ⚠ No-op with the in-memory default Store (a restart
# loses the log); becomes real only with a durable Store — see below.
spring.transaction.saga.recover-on-start=true

# --- optional durable store (starter-transaction-saga-gorm) ------------------
# Uncomment WITH the blank import of that starter AND a gorm driver starter
# (mysql/postgres/...): the gorm Store then takes over the coordinator's log
# and the recovery scan (OnMissingBean), AutoMigrating saga_snapshots.
# spring.transaction.saga.store=gorm

# --- observability (starter-otel) --------------------------------------------
spring.observability.enable=true
spring.observability.service-name=saga-demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
```

**Expected output** (failure drill — `place(ctx, "order-1")` with the failing NotifyUser):

```
... saga coordinator created tracing=true
... inventory: deducted 2 for order-1
... payment: charged 50 for order-1
... payment: refunded pay:order-1        # compensation runs in REVERSE step order
... inventory: restocked 2 for order-1
```

and `place` returns `err = "notify: SMTP unavailable"`,
`res.Status = StatusCompensated`, `res.Errors[0] = {Step: NotifyUser, Phase: Action}`.

**Verify** (after `go run .`): the five lines above appear; exit code 0.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-transaction-saga
  ├─ gs.Provide(transaction.NewStepRegistry)            [condition: enabled]
  ├─ gs.Provide(MemoryStore as transaction.Store)       [enabled + OnMissingBean[Store]]
  ├─ gs.Provide(newCoordinator)                         [enabled; exported as Coordinator]
  └─ gs.Provide(newRecoveryRunner)                      [enabled + recover-on-start; exported as gs.Runner]
        │
gs.Run()
  ├─ config bind: ${spring.transaction.saga} → Config (3 value tags, all := defaults)
  ├─ bean wiring: Store autowired into the coordinator; StepRegistry + Store +
  │   Coordinator autowired into the recovery Runner
  ├─ Rooter/bean construction: YOUR code calls reg.Register(...) here — this is
  │   why registration must happen at wiring time, not from a Runner
  ├─ log: "saga coordinator created tracing=<v>"
  ├─ Runners execute (recovery Runner first-run is a no-op unless a durable Store
  │   holds StatusRunning snapshots) — never fails startup
  └─ on SIGTERM: standard container shutdown; no saga-specific drain
```

A durable-Store starter (saga-gorm) registers its `transaction.Store` under
`spring.transaction.saga.store=gorm`; because the in-memory default is `OnMissingBean`, the durable
store wins and both the coordinator and the recovery scan switch to it — no business-code change
(starter.go comment: "It steps aside (OnMissingBean) the moment a durable-Store starter contributes
its own transaction.Store").

### 2.2 One transaction, step by step — including the failure path

`place(ctx, "order-1")` with steps [DeductInventory, ChargePayment, NotifyUser(fails)]:

1. `GlobalTransactional` (or your direct call) looks up the steps by method name; an unregistered
   method falls through to `proceed` transparently (global.go: no interception without declaration).
2. Saga id comes from the context (`WithSagaID`); without one the method name is used — only correct
   for a single in-flight instance, so always set an explicit id.
3. **Step 1**: coordinator persists intent FIRST — snapshot `{Status: Running, InProgress:
   "DeductInventory"}` — then runs the Action (coordinator.go: "Record the intent before running:
   the Action may cause a side effect a crash would strand"). On success the snapshot is re-saved
   with the step folded into `Completed` and `InProgress` cleared.
4. **Step 2** likewise: intent → Action ("charged 50") → confirm completion with the payment token
   recorded in `StepResults`.
5. **Step 3 Action fails**: the failing error is appended to `Result.Errors` first, so a compensated
   saga still explains why it rolled back; then `compensate` runs the COMPLETED steps in reverse —
   ChargePayment.Compensate(token) → DeductInventory.Compensate — each under the same retry policy
   and observer as the action; a nil Compensate on a reached step sets `StatusCompensationFailed`
   and records the step as irreversible (surfaced, never silently skipped).
6. **finish**: a committed saga's log is DELETED (nothing to recover); a compensated/failed saga's
   log is KEPT with the terminal status so operators can inspect it.
7. `Execute` returns `(Result{Status: Compensated, Errors:[…]}, originalActionError)` — the
   compensation outcome never masks the action error.

Design rationale worth knowing: persistence errors during `persistRunning` are intentionally
swallowed — "the saga has already made progress and failing the whole operation on a log write
would be worse than a gap in the log" (coordinator.go). Retries reuse `resilience.Policy` via a
"default" executor so saga step retries and outbound resilience share one knob set.

---

## 3. Per-key behavior reference

Exactly three keys (verified: `grep -rhoE 'value:"[^"]+"' … | sort -u` → 3 tags). All are
TOP-LEVEL absolute keys under `spring.transaction.saga` — the starter binds one instance of
`Config`; there is no multi-instance group.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.transaction.saga.enabled` | bool | `true` | `OnProperty(...).HavingValue("true").MatchIfMissing()` — absent means ON. `false` imports the module without contributing any bean (useful when a test binary wants the types but not the wiring). | `false` by accident → no Coordinator bean → autowire of `transaction.Coordinator` fails at wiring (visible, not silent). |
| `spring.transaction.saga.tracing` | bool | `true` | Attaches `transactionobserve.SagaObserver{}` at coordinator construction → one otel child span per step phase on the starter-otel globals. No-op (no spans, no warnings) without starter-otel. | Expecting traces without starter-otel → nothing; also no error anywhere. |
| `spring.transaction.saga.recover-on-start` | bool | `true` | Registers the recovery `gs.Runner` that scans `Store.Pending()` for `StatusRunning` snapshots and compensates each (backward recovery). ⚠ **Coupled to the store seam**: with the default in-memory `MemoryStore` the scan is always empty after a restart — the key is effectively dead unless a durable Store starter (e.g. `spring.transaction.saga.store=gorm` + starter-transaction-saga-gorm) is active. | Setting `false` with a durable store → crash-stranded sagas are NEVER compensated (silently inconsistent); setting `true` without one → no-op, no warning. |

Related keys owned by OTHER modules that interact here (not this starter's surface):
`spring.transaction.saga.store=gorm` and `spring.transaction.saga.gorm.*` belong to
starter-transaction-saga-gorm; `spring.observability.*` to starter-otel.

---

## 4. Verification & fault drills

### 4.1 Compensation drill (in-process failure)

Run the worked project: the NotifyUser step fails on purpose. Assert in the self-test:

```go
res, err := svc.place(ctx, "order-1")
// err != nil, res.Status == transaction.StatusCompensated
// res.Errors[0].Step == "NotifyUser", Phase == PhaseAction
```

Both compensations must have run (reverse order — check the log lines in §1). With a deliberately
failing Compensate instead, `res.Status` becomes `StatusCompensationFailed` and `res.Errors`
contains both the action error and the compensation failure — that is the alerting signal for
manual intervention.

### 4.2 Irreversible-step drill

Give ChargePayment no `Compensate` and let NotifyUser fail: the coordinator records
`step "ChargePayment" is irreversible (no Compensate)` in `Result.Errors`, sets
`StatusCompensationFailed`, and CONTINUES compensating the remaining steps (surface, don't skip).

### 4.3 Recovery-on-restart drill (requires durable store)

1. Import starter-transaction-saga-gorm + a gorm driver starter; set
   `spring.transaction.saga.store=gorm`; point the driver at a database. On boot the store
   AutoMigrates `saga_snapshots` (fail-fast if it cannot).
2. Start a saga whose step 2 Action blocks (sleep), kill -9 the process mid-step.
3. Restart. The recovery Runner scans Pending, rebuilds steps from the StepRegistry by the
   persisted `Method`, and compensates: the in-flight step FIRST with a nil result (its Action
   return value was never recorded — the compensator must tolerate that), then the completed
   steps in reverse. Test `TestRecoveryRunner_CompensatesPendingSaga` pins the order `!b, !a`.
4. Watch: `saga recovery: saga "order-1" recovered with status Compensated`. A saga whose method
   is no longer registered logs `no steps registered for method ... skipping` and is skipped.
   Note: during recovery the Runner's ctx does NOT carry the saga id — compensate from the
   Action result handed in (`r any`), or key reservations so the nil-result in-flight case
   (empty compensation) is tolerable, exactly like the TCC empty-rollback obligation.

### 4.4 Observing

- Traces (needs starter-otel): spans `saga.action <step>` / `saga.compensate <step>` per phase,
  attributes `saga.id`, `saga.step`, `saga.phase`; failures recorded with error status. Jaeger
  query by service name; a compensated saga shows the action span failed plus the compensate
  spans beneath it.
- Logs: tag `AppDef`; lines `saga coordinator created tracing=…` and the `saga recovery: …` family.
- No metrics, no health indicator (see §6).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Autowire error on `transaction.Coordinator` | `enabled=false`, or a condition excluded the bean | Remove the disable; a blank import is the default-on path. |
| Compensation didn't run after a crash | In-memory default store — restart lost the log | Add saga-gorm (+`store=gorm`) or your own durable `transaction.Store` bean; `recover-on-start` alone is not enough. |
| `saga recovery: no steps registered for method "X"` | Steps registered from a Runner / after startup, or method name drift between register and execute | Register at bean-construction time under the exact `Method` string the saga persisted. |
| Two concurrent sagas interfere / second saga overwrites the first | No `WithSagaID` → id falls back to the method name, one log slot per method | Set the id at the edge: `transaction.WithSagaID(ctx, id)`. |
| `StatusCompensationFailed` in results | A compensation error, or a completed step with nil `Compensate` | Inspect `Result.Errors`; make Compensate idempotent + retried (`Step.Retry`); treat leftover failed logs as ops backlog. |
| `Recover requires a Store` error | `Coordinator.Recover` called with no store configured | Only relevant for manual Recover use; the starter always wires the in-memory store, so this means a hand-built coordinator. |
| No spans despite `tracing=true` | starter-otel not imported / observability disabled | Import starter-otel and set `spring.observability.*`; the observer is a silent no-op otherwise. |
| `execute` returned err but ledgers show partial effects | Compensate not idempotent, or it ignores the Action result it receives | Compensate from the handed `result` (token/id), key side effects by saga id; expect replay after recovery. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 3 (all bool, all default-on) |
| Required | 0 |
| Quickstart external deps | 0 (1 DB only for the durable-store drill; collector only for traces) |
| "Watch out" entries | 5 |

Design suspects (audit ledger; originals kept, new ones added):

- **No `example/` directory in this module** (tcc and at-gorm have one) — the worked project in §1
  is source-verified but not smoke-verified as a shipped example → add one mirroring the tcc
  example's self-test pattern.
- `recover-on-start` is a no-op with the default in-memory Store; its meaning only materializes via
  a second module → acceptable, documented (kept).
- Package doc says "the container holds two beans" (starter.go:23-28) but `init()` registers FOUR
  (registry, store, coordinator, runner) → doc drift; fix the comment.
- No metrics and no health indicator: a saga stuck `CompensationFailed` is invisible except by
  reading the kept store rows manually → consider a Pending/failed-saga gauge or health check.
- Persistence errors during a saga are swallowed by design (coordinator.go persistRunning) →
  durable-store implementations must self-monitor; consider at least a warn log.
- README quick-start shows a `Step.Action`/`Compensate` signature with a `*transaction.StepResults`
  parameter that does not exist in source (`Action func(ctx) (any, error)`) → fix README.
