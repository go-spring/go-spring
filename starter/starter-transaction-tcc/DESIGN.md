# starter-transaction-tcc Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-transaction-tcc` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that contributes the TCC
(Try / Confirm / Cancel) distributed-transaction capability from
`spring/transaction/tcc`. It targets short, strongly-consistent flows
that need a resource *held* between the try and the commit.

## 1. Responsibilities & Boundaries

- **In scope:** `tcc.Coordinator`, `*tcc.ParticipantRegistry`, default
  in-memory `tcc.Store`, and a `gs.Runner` that drives interrupted
  transactions to their decided outcome on startup.
- **Out of scope:** the participant business logic (each `Try` /
  `Confirm` / `Cancel` is application code); durable `Store`
  implementations (separate modules).

## 2. Why a separate package from Saga

Failure semantics differ enough that merging would dilute expressiveness:

| | Saga | TCC |
|---|---|---|
| Forward step | real effect | reserves a resource |
| On failure | compensating function | cancel the reservation |
| Isolation | none | reservation invisible until confirm |
| `Compensate == nil` | irreversible step (allowed) | programming error |

The `spring/transaction/tcc/` subpackage lives inside the `stdlib`
module (no new `go.mod`) but is deliberately not folded into the Saga
`transaction` package.

## 3. Key Decisions

- **`validate` runs before any side-effect.** All three phase functions
  must be non-nil, participant names must be unique — a TCC participant
  missing a phase is always a programming error, unlike Saga's tolerated
  irreversible step. Fail-fast at Execute entry.
- **Try-all → Confirm-all-forward / Cancel-tried-reverse.** Confirm goes
  in the original order (idempotent replay), Cancel in reverse (the
  compensation-like path).
- **Confirm failure returns `(res, nil)`.** The transaction has already
  decided to commit; a Confirm error is reported through
  `StatusConfirmFailed` + `Result.Errors`, not through the top-level
  error return (which is reserved for Try failures).
- **Crash recovery is decision-log-driven.** Statuses split into
  in-flight `Trying` / `Confirming` / `Cancelling` and terminals
  `Committed` / `Cancelled` / `ConfirmFailed` / `CancelFailed`. Each
  Try persists `Trying + InProgress`; when all Try phases succeed the
  starter persists **`Confirming` (the commit decision)** *before*
  running Confirms.
  Recovery reads status:
  - `Confirming` → forward Confirm (decision already made; idempotent
    replay).
  - `Trying` / `Cancelling` → backward Cancel (no commit decision;
    InProgress leads the cancel list with a nil `tried` value =
    empty rollback).
  - terminal / not-found → idempotent no-op.
- **`Store.Pending` returns every non-terminal**, unlike Saga's
  Running-only scan — TCC has more in-flight statuses to resume.
- **Same `OnMissingBean` swap for durable Stores.** Default in-memory
  `MemoryStore` is registered with `gs.OnMissingBean`, so a durable
  Store starter takes over both the coordinator and the recovery scan.
- **Observer seam mirrors Saga.** `otelObserver` opens
  `tcc.{try|confirm|cancel} <participant>` spans on `starter-otel`
  globals — same zero-dep pattern.

## 4. Participant Obligations

The stdlib cannot enforce these across process boundaries; the starter
documents them and the example (stock + balance ledger) exercises them:

- **Idempotence** — Confirm/Cancel may be replayed; the second call is a
  no-op.
- **Empty rollback** — Cancel may run for a participant whose Try never
  recorded a result (`tried == nil`); do nothing then.
- **Anti-hanging** — a delayed Try arriving after a Cancel must not
  re-reserve. Keying by transaction id lets you detect this.

## 5. Trade-offs / Alternatives Rejected

- **Merge TCC into Saga's `transaction` package — rejected.** Different
  failure semantics; separate abstractions keep each expressive.
- **Return Confirm errors from Execute — rejected.** Confirm failures
  are logically post-commit and belong on the Result, not the top-level
  error channel.
