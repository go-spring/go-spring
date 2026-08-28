# starter-outbox-gorm

A gorm-backed transactional outbox for Go-Spring: business writes and message
publishes commit atomically in one database transaction, then a background
relay drains the `outbox_message` table to any registered `messaging.Binder`
(kafka, nats, ...). The neutral core lives in `cloud/experimental/outbox`.

## Wiring

Blank-import the starter and add one entry per relay under `spring.outbox`:

```properties
spring.outbox.main.binder=kafka
spring.outbox.main.auto-migrate=true
```

Each entry autowires a `*gorm.DB` (from whichever gorm driver starter you
already use; `db` selects a named bean), resolves the binder at startup,
contributes a health indicator (`outbox:<name>`) and runs the relay loop on
the bean's Init/Destroy — graceful shutdown drains in-flight records.

## Publishing

The write side is a plain function inside your own transaction — no callbacks,
no magic:

```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil { return err }
    return StarterOutboxGorm.Publish(tx, "orders", order.ID, payload, nil)
})
```

`PublishMessage(tx, dest, msg)` takes a `*messaging.Message` envelope.

## Table

`auto-migrate` (default off) creates the table via `Migrate`; when you manage
schema externally use:

```sql
CREATE TABLE outbox_message (
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    destination   VARCHAR(255) NOT NULL,
    msg_key       VARCHAR(255),
    payload       BLOB         NOT NULL,
    headers       TEXT,
    status        VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempts      INT          NOT NULL DEFAULT 0,
    next_retry_at DATETIME     NOT NULL,
    last_error    TEXT,
    created_at    DATETIME     NOT NULL,
    sent_at       DATETIME
);
CREATE INDEX idx_outbox_dispatch ON outbox_message (status, next_retry_at);
```

On mysql (8+) and postgres the relay fetches with `FOR UPDATE SKIP LOCKED`, so
several app instances can run relays concurrently. SQLite serializes writes on
its own. MySQL 5.7 lacks `SKIP LOCKED` — run a single relay instance there.

## Configuration

| key | default | meaning |
|---|---|---|
| `db` | (autowire) | `*gorm.DB` bean name backing the table |
| `binder` | — required | messaging binder name (kafka, nats, ...) |
| `auto-migrate` | `false` | create the table at startup |
| `poll-interval` | `1s` | wait between drained polls (floor 100ms) |
| `batch-size` | `100` | records fetched per poll |
| `max-attempts` | `8` | delivery attempts before dead-letter |
| `backoff-base` / `backoff-max` | `1s` / `1m` | exponential retry backoff |
| `dlq-suffix` | `.dlq` | dead-letter destination suffix; empty disables the DLQ copy |

Delivery is **at-least-once**; consumers must be idempotent. Ordering is
per-batch ID order only — set the message key for per-entity ordering. See
`cloud/experimental/outbox/DESIGN.md` for the full semantics.

## Example

`example/` runs the whole pattern on in-memory sqlite and an in-process binder,
self-asserting atomicity (commit delivers, rollback doesn't), retry-with-backoff
and dead-lettering. `example/check.sh` is its smoke test.
### Log tag

Runtime logs from this module carry the tag `_app_outbox` (transactional outbox). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.outbox.type=Logger
logger.outbox.level=WARN
logger.outbox.tag=_app_outbox
```
