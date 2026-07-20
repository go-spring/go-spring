# at Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`at` is the AT (Automatic Transaction) pattern in the zero-dependency stdlib
layer. It gives Go the effect of Seata AT — undo/rollback derived automatically
from before-images captured at the resource — without pulling any SQL/ORM
knowledge into stdlib. Sibling patterns Saga and TCC live in
[`spring/transaction`](../DESIGN.md) and [`spring/transaction/tcc`](../tcc/DESIGN.md).

## 1. Responsibilities and Boundaries

- Orchestrate a global AT transaction: `Begin` a global transaction, let
  branches `Register` as their resource writes, then `Commit` (drop undo logs)
  or `Rollback` (restore before-images) each registered branch.
- Own the global-lock lifecycle: acquire lock is the branch's job (it happens
  in the same local transaction as the write); release lock happens once per
  global transaction, in the coordinator.
- Refuse SQL knowledge. Image capture, undo-log persistence and row
  restoration are ORM/driver-specific and live behind the `Branch` seam. The
  coordinator drives commit/rollback without ever touching a `*sql.DB`.
- Refuse to be an isolation solution for reads. AT gives write-write isolation
  via `GlobalLock`; read isolation is out of scope (read committed still
  applies at each local transaction).

## 2. Key Abstractions and Seams

- **`Branch` interface as the resource seam.** One `Branch` per database
  participating in a global transaction; a starter (gorm/other ORMs) supplies
  the implementation. The coordinator deduplicates by `Branch.ID()`, so a
  resource that runs several statements in one XID is committed/rolled back
  exactly once.
- **`GlobalLock` interface.** `MemoryGlobalLock` is bundled for a single
  process (real write-write isolation for concurrent in-process global
  transactions); a distributed deployment supplies a shared (redis/db) lock.
  A nil lock disables isolation — acceptable for single-writer setups and
  tests only.
- **XID on context.** The coordinator generates an XID in `Begin` and puts it
  on the context via `WithXID`. Resource interceptors read it back with
  `XIDFromContext` to decide whether they are inside a global transaction —
  the "not present" case is how a starter stays transparent for non-AT calls.
- **`GlobalAT` aspect.** Unlike Saga's `GlobalTransactional` and TCC's
  `GlobalTCC`, AT has no hand-written steps to register: branches
  self-register from the resource interceptor. The aspect just begins/resolves
  the transaction and injects the XID.
- **`Observer` seam** for per-branch-phase spans, so a starter attaches otel
  without stdlib importing otel.
- **`RetryPolicy = resilience.Policy` alias** — reused so second-phase retries
  and outbound resilience share one config surface.

## 3. Constraints

- **`Branch.Commit` and `Branch.Rollback` must be idempotent.** A crash or a
  retry may replay them, and rollback in particular may run against rows
  already restored. The bundled `RetryPolicy` assumes this contract.
- **Rollback unwinds branches in reverse registration order** — the same
  reverse-order convention as Saga/TCC — so a later branch's undo runs before
  earlier ones.
- **`take` makes resolution single-shot.** Commit/Rollback both remove the
  XID's branch list atomically; a second call finds nothing and returns
  `ErrUnknownTransaction`. This makes double-resolution safe by construction.
- **Global-lock release is best-effort.** A `Release` error is swallowed
  because the transaction is already resolved; a lock backend's TTL /
  monitoring reclaims a stranded key. This is deliberate — failing an already-
  resolved operation over a lock cleanup would be worse.
- **A commit failure is a cleanup failure, not a consistency loss.** The
  business data was already committed locally in phase one; only the undo
  logs are stale, so operators clean them up separately. Rollback failure is
  different — that means rows may not have been restored and needs alerting
  (`StatusRollbackFailed`).
- **Nested `GlobalAT` reuses the outer XID.** The aspect proceeds transparently
  when an XID is already on the context; inner branches join the outer global
  transaction. There is no nested global transaction in AT.

## 4. Trade-offs and Alternatives Rejected

- **Automatic compensation over hand-written compensation.** That is AT's
  defining property — business code writes only forward SQL. Saga and TCC live
  in sibling packages precisely because their compensation semantics differ.
- **Per-resource branch bean, no driver registry.** A branch needs a live
  connection and its ORM's DML interception, not a declarative policy, so the
  seam is the interface type — the same choice `spring/lock` and
  `spring/batch` make.
- **In-process coordinator first.** A single service driving several databases
  is the dominant Go topology; an external Seata TC would add a coordination
  hop for a case most services do not have. The interface stays open so a
  starter can supply a remote coordinator later.
- **No read isolation.** Real AT read isolation requires reading through the
  coordinator to see other in-flight transactions' state, which pulls a
  network dependency into every read. Write-write isolation via the global
  lock covers the common conflict case and keeps reads plain SQL.
