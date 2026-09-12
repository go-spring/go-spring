# outbox

Transactional outbox for Go-Spring: make business writes and message publishes
atomic by writing the message to an outbox table **inside the same database
transaction**, then draining the table to a real broker in the background.

It is the Go-native answer to "I need to publish an event when this transaction
commits, and not publish when it rolls back" — without a distributed
transaction coordinator and without dual-write inconsistency.

## What lives here

This package is the broker- and storage-neutral core:

- `Record` — one outbox row's neutral projection (`pending → sent | dead`)
- `Store` — the persistence seam (`Fetch` / `MarkSent` / `MarkFailed` / `MarkDead`)
- `Relay` — the drain loop: poll → publish → retry with backoff → dead-letter
- `Observer` — the observability seam (publish / retry / dead events)
- `MemoryStore` — in-memory store for tests and demos only

The **write side** (inserting into the outbox table inside your transaction)
and the **gorm Store implementation** live in the storage backend starter —
`go-spring.org/starter-outbox-gorm` — because they need the transaction
handle. Delivery goes through any registered `messaging.Driver` (kafka, nats,
...), so the broker is a wiring choice.

## Semantics

- **At-least-once.** A crash between a successful publish and `MarkSent`
  re-delivers the message. Consumers must be idempotent (or de-duplicate by
  record ID / business key).
- **No global ordering.** Records deliver in ID order within a batch; across
  batches, instances and restarts nothing is promised. Set `Record.Key` and
  let the broker's keyed partitions provide per-entity ordering.
- **Bounded retries, then DLQ.** Failures back off exponentially (1s doubling,
  capped 1m by default); after `MaxAttempts` (8 by default) a copy stamped
  with the `messaging` DLQ header contract goes to `destination + DLQSuffix`
  (".dlq"; empty disables the copy). A failing DLQ publish keeps the record
  pending — losing a dead letter is worse than redelivering.
- **One bad row never blocks the batch.** Each record is resolved
  independently.

## Usage sketch

```go
// inside your transaction (starter-outbox-gorm):
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil { return err }
    return StarterOutboxGorm.Publish(tx, "orders", order.ID, payload, nil)
})
// the relay (wired by the starter from spring.outbox.instances.* config) drains it
```

See `starter/experimental/starter-outbox-gorm` for configuration, the table
DDL and a self-asserting example.
