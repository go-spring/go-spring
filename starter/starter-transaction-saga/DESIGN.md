# starter-transaction-saga Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-transaction-saga` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that wires the Saga
distributed-transaction capability from `stdlib/transaction` into a
Go-Spring application. It reaches `@GlobalTransactional(SAGA)` equivalence
with an in-process coordinator + aspect chain, not by replicating Seata
TC/TM/RM roles or bytecode magic.

## 1. Responsibilities & Boundaries

- **In scope:** `transaction.Coordinator` bean, `*transaction.StepRegistry`
  bean, a default in-memory `transaction.Store`, and a startup
  `gs.Runner` that compensates in-flight sagas after a crash.
- **Out of scope:** step definitions (application-owned); TCC/AT (their
  own starters); durable store implementations (a separate module such
  as `starter-transaction-saga-gorm`); network transport — Saga is
  in-process.

## 2. Key Decisions

- **Saga chosen over AT/TCC.** AT needs SQL parsing + undo_log + gorm
  plugin and only covers SQL resources; TCC forces intrusive
  Try/Confirm/Cancel triples. Saga is the only pattern with reasonable
  ROI that also covers non-SQL downstreams (MQ / HTTP / cache).
- **Not merged into a common transaction abstraction.** AT / TCC are
  shipped as separate packages / starters — their failure semantics
  differ enough that a merged abstraction dilutes expressiveness.
- **In-memory store by default, durable store via `OnMissingBean`.**
  The starter registers `MemoryStore` with
  `Condition(enabled, gs.OnMissingBean[transaction.Store]())`. A
  durable-store starter (e.g. `starter-transaction-saga-gorm`)
  contributing its own `transaction.Store` displaces the default —
  crash recovery switches on without any change to business code.
- **`Observer` seam for tracing.** `stdlib/transaction` is zero-dep and
  cannot import otel. The starter provides an `otelObserver` that opens
  one child span per phase (`saga.action|compensate <step>`) on the
  globals `starter-otel` installs — the standard zero-dep pattern
  (call-site span helpers live in the starter layer).
- **Retry policy reuses `stdlib/resilience`.** `RetryPolicy =
  resilience.Policy` (an alias in `stdlib/transaction`), so retries
  are executed through the resilience `default` driver rather than
  a second retry loop.

## 3. Recovery

- **Backward recovery only.** After a crash, every in-flight step is
  compensated in reverse. Compensations must already be idempotent, so
  this is the safest minimal semantic. Forward recovery would require
  idempotent Actions too and is deferred.
- **Steps must be registered at wiring time**, not from a custom
  `gs.Runner`, otherwise the recovery Runner may race the registration
  and skip sagas as "no steps registered".
- **Persist timing = replayable log.** Save before every Action
  (`Running` + `InProgress` + completed set); Save after success (moved
  to `Completed`, `InProgress` cleared). Committed sagas Delete;
  compensated / failed sagas keep the terminal log for post-mortem.
- **Persist write failures on log are swallowed.** The saga has already
  advanced; failing the whole call because of a log-write hiccup makes
  a bad situation worse.

## 4. Constraints & Risks

- **No isolation.** Saga has no read/write barrier — pair with
  `stdlib/lock` at the business boundary if you need one.
- **Compensations must be idempotent.** A crash can retry them at any
  offset; the coordinator collects, not first-error terminates, so a
  compensation chain with multiple failures gathers every error into
  `Result.Errors`.
- **Nil `Compensate` = `CompensationFailed`, not silent skip.** An
  irreversible step is a design decision the application must own.
- **Production needs a durable Store.** In-memory is fine for tests and
  a single-process demo; it does not survive a restart.

## 5. Trade-offs / Alternatives Rejected

- **Merge with TCC / AT into one abstraction — rejected.** Different
  failure semantics; separate packages keep each expressive.
- **`stdlib/transaction` depending on otel — rejected.** The `Observer`
  seam keeps the zero-dep invariant intact.
- **Fabricate steps at recovery time — rejected.** Actions and
  Compensations are functions and are not persisted; the registry, keyed
  by method name, is the only correct rebuild path.
