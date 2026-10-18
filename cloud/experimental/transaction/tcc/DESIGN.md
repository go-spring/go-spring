# tcc Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`tcc` is TCC (Try / Confirm / Cancel) in the zero-dependency stdlib layer,
kept in its own subpackage precisely because its failure semantics differ from
Saga: Try phases *reserve* rather than commit, and a global decision (commit
or rollback) is made only after every Try succeeds. Saga lives in the parent
[`transaction`](../DESIGN.md); AT lives in [`transaction/at`](../at/DESIGN.md).

## 1. Responsibilities and Boundaries

- Orchestrate a two-phase business transaction: Try every participant; if all
  succeed, record the commit decision and Confirm each; if any fails, record
  the rollback decision and Cancel each tried participant in reverse.
- Persist the transaction log via `Store` so a crash between phases resumes
  cleanly: forward from `StatusConfirming`, backward from `StatusTrying` /
  `StatusCancelling`.
- Expose an `Observer` seam so a starter can attach otel spans per phase
  without stdlib importing otel.
- Refuse participant business logic. The three phases are the caller's
  functions; the coordinator only sequences and retries them.
- Refuse to solve the three classic TCC hazards across process boundaries —
  idempotence, empty rollback, anti-hanging are participant obligations
  documented in the package doc.

## 2. Key Abstractions and Seams

- **`Participant` with three required functions.** Unlike a Saga step (whose
  `Compensate` may legitimately be nil), a TCC participant with any nil phase
  is a programming error rejected before side effects; the pattern's whole
  contract rests on all three phases being present.
- **`Coordinator` interface** — the bundled in-process implementation runs
  phases synchronously; a future external-store implementation satisfies the
  same interface, so switching to crash-recoverable orchestration is a
  construction change, not a business change.
- **`Store` interface** — the single pluggable persistence seam.
  `MemoryStore` is bundled; a durable starter contributes a bean.
- **`ParticipantRegistry` + `GlobalTCC`** — the explicit, no-reflection form
  of `@GlobalTransactional(type = TCC)`. Business code registers a method's
  participants once at wiring time; the interceptor looks them up by joinpoint
  method name, keeping un-declared methods transparent.
- **`Observer` seam** for otel spans (nil disables observation).
- **`RetryPolicy = resilience.Policy` alias** — reused so TCC phase retries
  and outbound resilience share one config surface. Recommended non-zero for
  Confirm/Cancel; the contract requires them to eventually succeed.
- **Decision-log-driven recovery.** Recovery reads the persisted `Status`:
  `StatusConfirming` (decision was commit) => forward Confirm; anything else
  in-flight => backward Cancel. This makes the coordinator idempotent under
  arbitrary crashes.

## 3. Constraints

- **Confirm and Cancel must be idempotent.** A crash-driven retry will replay
  them. `RetryPolicy` for these phases is the safety net.
- **Cancel must tolerate a nil result** (empty rollback). Try may have crashed
  before recording a value; the participant must key its reservation by the
  transaction id so it can detect "nothing to release" and no-op.
- **Anti-hanging is a participant obligation.** A delayed Try arriving after a
  Cancel must not re-reserve. The framework surfaces the transaction id in
  the callback; the participant uses it to reject late Tries.
- **A committed transaction's log is deleted.** A stored snapshot is therefore
  never `StatusCommitted`; recovery scans for anything non-terminal.
- **Confirm is in registration order; Cancel is in reverse.** Confirming
  forward keeps the "prepared / confirmed" mental model; cancelling reverse
  matches Saga and AT for consistency across the three sibling packages.
- **A confirm/cancel failure never masks the try failure.** `Result.Errors`
  lists the failing try first (when the cancel path ran), so a `Cancelled`
  outcome still explains why it rolled back.

## 4. Trade-offs and Alternatives Rejected

- **Separate package from Saga.** Saga compensates real effects; TCC confirms
  tentative reservations. Merging them would blur the isolation guarantee
  (TCC has one, Saga does not) and force one API to describe both.
- **Explicit `ParticipantRegistry` over reflection.** Java reads annotations
  at classload time; Go registers at wiring time in one line. The aspect stays
  a plain map lookup with no reflect-over-user-code.
- **In-process coordinator first.** A single service driving several
  participants is the dominant Go topology. The interface is open for an
  external coordinator implementation later.
- **`Retry` on Try is optional, on Confirm/Cancel is recommended.** A Try can
  legitimately fail (business validation) and its failure is the intended
  signal to Cancel; Confirm/Cancel failing is a coordination bug the retry
  policy should smooth over.
- **`GlobalTCC` falls back to the method name for the id.** Same trade-off as
  Saga's `GlobalTransactional`: caller-supplied id via `WithTransactionID` is
  correct; the method-name fallback is a last-resort safety net for a single
  in-flight instance.
