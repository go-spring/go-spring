# transaction Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`transaction` supplies the Saga abstraction — the eventual-consistency form of
distributed transaction — in the zero-dependency `stdlib` layer. It replaces
Seata Saga and Spring's `@GlobalTransactional` at the *effect* level: an
ordered list of compensable steps whose failures roll back in reverse. TCC and
AT live in the `tcc` and `at` subpackages, chosen because each pattern has
different failure semantics and it is honest to keep them separate.

## 1. Responsibilities and Boundaries

- Orchestrate compensable business steps: `Action` forward, `Compensate` on
  failure of a later step, in reverse order.
- Persist a saga log via `Store` so a crashed process can resume compensation;
  backward recovery only.
- Expose an `Observer` seam so a starter can attach otel spans without stdlib
  importing otel.
- Refuse isolation. Intermediate saga states are visible to concurrent readers;
  business code must guard dirty reads with a status flag or a `spring/lock`.
- Refuse SQL parsing / undo-log generation. That would be the AT model, which
  lives in `transaction/at` and is explicitly a separate seam.

## 2. Key Abstractions and Seams

- **`Coordinator` interface** — the bundled in-process implementation runs
  steps synchronously. A future external-store implementation satisfies the
  same interface, so switching to crash-recoverable orchestration is a
  construction change, not a business change.
- **`Store` interface** — the single pluggable persistence seam. `MemoryStore`
  is bundled for the common single-gateway process case; a durable starter
  contributes a gorm/redis/etcd-backed bean.
- **`StepRegistry` + `GlobalTransactional`** — the explicit, no-reflection
  equivalent of `@GlobalTransactional`. Business code registers a method's
  steps once at wiring time; the interceptor looks them up by joinpoint method
  name. This keeps un-declared methods transparent (proceed through) and
  avoids reflecting into business function bodies.
- **`Observer` seam** — a starter opens one span per step phase (action /
  compensate). Nil disables observation entirely.
- **`RetryPolicy = resilience.Policy` alias** — deliberate reuse of the same
  knob set (`MaxRetries`, `Timeout`, ...) so saga step retries and outbound
  resilience share one config surface instead of duplicating retry logic.

## 3. Constraints

- **Compensate must be idempotent.** It is retried per `RetryPolicy` and, after
  crash recovery, replayed against a resource that may already be in the
  compensated state. A nil `Compensate` marks the step irreversible: reaching
  it during rollback is recorded as `StatusCompensationFailed` so the operator
  is alerted, never silently skipped.
- **Backward recovery only.** After a crash, `Recover` compensates every step
  that might have effected — the in-flight step first (with a nil result,
  since its Action return value was never persisted), then the completed steps
  in reverse. Forward retry-to-commit is not attempted because the crash point
  is unknown; going forward could double-effect.
- **Log write failures do not fail the saga in-flight.** A `Store.Save` error
  during `persistRunning` is swallowed intentionally: the action already ran
  and failing the operation over a log gap would be worse than the gap. The
  terminal write is where operators need consistency, and durable-store
  implementations should surface their own persistence monitoring.
- **A committed saga's log is deleted.** A stored snapshot is therefore never
  `StatusCommitted`; `Pending` returns only `StatusRunning` snapshots for
  recovery to pick up.
- **Saga id is caller-supplied.** `WithSagaID` sits on the request context so
  the id lines up with the caller's idempotency key. The interceptor falls
  back to the method name only as a last resort — correct for a single
  in-flight instance, misleading for anything else.

## 4. Trade-offs and Alternatives Rejected

- **Saga over generic TX 2PC.** Seata's TC/TM/RM roles bring a coordinator
  service and its failure modes; a Saga's forward+compensate model matches
  what Go microservices actually need for MQ publishes, HTTP calls and cache
  writes. Strong isolation (AT) is available as an opt-in subpackage.
- **In-process coordinator first.** The single-gateway process driving several
  downstreams is the dominant Go microservice topology. An external
  coordinator would add a coordination hop for a case most services do not
  have. The interface stays open so a starter can supply one later.
- **No auto-generated compensation.** SQL-level undo logs (AT) would drag the
  package into database drivers, and every non-SQL downstream would still need
  a hand-written compensation. Requiring `Compensate` up front is cheaper.
- **Explicit `StepRegistry` over reflect-based scanning.** Java's annotation
  approach walks bytecode; Go can register at wiring time in one line. That
  keeps the aspect a plain lookup with no reflection over user code.
