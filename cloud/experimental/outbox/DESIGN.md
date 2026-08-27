# outbox — Design

## Problem

Publishing a message "when the transaction commits" is deceptively hard. The
two naive options both lose:

1. **Publish after commit** — a crash between commit and publish silently drops
   the event.
2. **Publish before commit** — a rollback leaves a phantom event, and the
   consumer sees a write that never happened.

The transactional outbox pattern makes the message itself a row in the same
database and transaction as the business write: commit publishes (eventually),
rollback discards. A background relay moves rows to the real broker. This is
the standard non-XA answer, and the only one that composes with plain SQL
databases.

## Position in the tree

`cloud/experimental/outbox`, a sibling of `messaging` — deliberately NOT under
`cloud/experimental/transaction`. The transaction family (saga / tcc / at)
shares a "global transaction + compensation" problem domain with Coordinators
and Stores; the outbox has no such concepts. Its only bloodline is messaging:
it reuses `Message`, `Binder` and the DLQ header contract. The package imports
only `cloud/experimental/messaging` plus the standard library — no gorm, no
spring, no otel.

## Shape

```
             (business tx)                      (background)
  Publish(tx, dest, msg) ──► outbox table ──► Relay ──Binder──► broker
  (in starter-outbox-gorm)    (Store impl)      │
                                                  ├─ ok ──► MarkSent
                                                  ├─ fail ─► MarkFailed (backoff)
                                                  └─ exhausted ─► DLQ copy ─► MarkDead
```

- **Store is the only storage seam.** The Relay is pure logic over it:
  testable with fakes, reusable across storage engines. `Fetch`'s contract
  requires that concurrent relays never receive the same pending record —
  HOW is backend detail (gorm: `FOR UPDATE SKIP LOCKED`; SQLite: single
  writer).
- **The write side is a plain function in the starter**, not a callback. An
  automatic gorm callback cannot know the destination, and magic interception
  violates the "assemble, don't adjudicate" container principle. Explicit
  `Publish(tx, ...)` inside the app's own transaction is honest about what it
  does and costs one line.
- **The relay runs on a bean Init/Destroy loop**, not as a `gs.Server`: it is a
  worker, not an endpoint, and blocking readiness on it would be wrong.

## Delivery semantics (and why)

- **At-least-once**, not exactly-once: the publish→MarkSent window is
  unbounded in any design that doesn't put the broker inside the transaction;
  exactly-once is a consumer-side concern (idempotency / de-dup). Stating
  this lets every other choice stay simple.
- **No global ordering promise.** Per-key serialization inside the relay would
  add head-of-line blocking and a second state machine; the broker's keyed
  partitions already solve per-entity ordering when the producer sets
  `Record.Key`. Within one batch the relay delivers in ID order — free, and
  the common single-writer case gets FIFO for nothing.
- **DLQ before dead.** An exhausted record whose DLQ copy fails to publish
  stays pending (retried next round): losing a dead letter is worse than
  redelivering. This mirrors `messaging.DeadLetter`'s philosophy on the
  consumer side; both routes stamp the same header contract
  (`x-dlq-error` / `x-dlq-retries` / `x-dlq-key`).
- **A poisoned row never blocks its batch.** Each record resolves
  independently; one bad destination must not starve the others.
- **Mark failures are tolerated.** If `MarkSent` fails after a successful
  publish, the record stays pending and is re-delivered next round — the
  stated at-least-once semantics absorb it.

## Knobs

`Config` is deliberately small and zero-value-usable: poll interval (1s, floor
100ms), batch size (100), max attempts (8), backoff base/cap (1s / 1m), DLQ
suffix (".dlq", empty disables). No per-destination overrides, no cron
scheduling, no LISTEN/NOTIFY — each of those has broken someone somewhere and
none is needed for correctness.

## Observability

`Observer` is a seam, not a dependency: cloud imports no tracing library. The
gorm starter ships a log adapter (retry → WARN, dead → ERROR); an otel adapter
follows the same pattern the transaction family uses when needed.

## Non-goals

- Inbox / consumer-side de-duplication table (a symmetric pattern, separate
   package if ever needed).
- Retention / archiving of sent rows — operational concern; `sent` rows are
  kept for audit and pruned by the operator.
- Sharded relay coordination beyond what `SKIP LOCKED` gives for free.
