# messaging
[English](README.md) | [中文](README_CN.md)

`messaging` is a broker-neutral publish/subscribe abstraction. Application code
publishes and consumes `Message` envelopes through one `Publisher` /
`Subscriber` pair, so switching broker (NATS, Kafka, Pulsar, RabbitMQ, MQTT,
...) is a wiring change, not a business-code rewrite.

## The API

| API | What it does |
| --- | --- |
| `Message{Key, Payload, Headers, Timestamp}` | The broker-neutral envelope. `Payload` is opaque `[]byte` — encoding lives one layer up. `Key` is the optional partition/ordering key. `Headers` carries metadata and doubles as the W3C trace-context carrier (nil-safe `Header`/`SetHeader`). |
| `Driver` | Opens publishers/subscribers against one broker connection; the destination/source/group strings are interpreted **in the broker's own terms** (subject, topic, queue, consumer group). |
| `Publisher` / `Subscriber` | Each bound to one destination/source at creation. `Subscribe` returns after delivery is established, not on completion; `Close` releases that subscriber/publisher, not the shared client. |
| `Handler` | `func(ctx, *Message) error` — a non-nil return signals delivery failure; how it surfaces (nack, redelivery, log) is broker-specific and documented by each starter. |
| `RegisterDriver` / `GetDriver` | Driver-registry idiom (panics on empty name, nil, duplicate) for a process-wide driver chosen by configured name. Starters usually wire the driver as a bean over a live client instead. |
| `Retry(h, RetryPolicy)` | Retries the handler in-process with exponential backoff (`MaxRetries`, `InitialInterval`, `Multiplier`, `MaxInterval`); success on any attempt acks, exhaustion returns the error so the broker takes over. Zero value = one attempt. |
| `DeadLetter(h, dlq, RetryPolicy)` | After retries are exhausted, publishes to the `dlq` publisher (bound to e.g. `"orders.dlq"`) and acks the original. The copy keeps the original headers plus `x-dlq-error` / `x-dlq-retries` / `x-dlq-key`. If the DLQ publish itself fails, the original error is returned — losing a dead letter is worse than redelivering. |
| `Recover(h)` | Converts a panicking handler into an error so one bad message cannot kill the delivery loop. |
| `NewMessageID()` / `HeaderMessageID` | A fresh 32-hex id + the reserved header it goes in. At-least-once delivery makes consumer-side de-duplication a must; key it on this id. Driver publishers auto-stamp it via `EnsureMessageID` when absent — set it yourself (or dedup on a business key) and the stamp is a no-op. |
| `HeaderDeliveryAttempt` | Reserved header the driver fills on consume from the broker's redelivery count (1-based; absent or "1" = first delivery). Read it to decide "retry" vs "straight to DLQ" without in-process state a restart would lose. |
| `PublishBatch(ctx, p, msgs...)` | Batch send through one call. Uses the optional `BatchPublisher` capability when the publisher implements it, otherwise publishes one by one in order, stopping at the first error (not transactional). |

Broker starters implementing `Driver`: `starter-nats`, `starter-kafka`,
`starter-kafka-sarama`, `starter-pulsar`, `starter-rabbitmq`, `starter-mqtt`,
`starter-rocketmq`. Each also exposes its raw client bean (`*nats.Conn`,
`*kgo.Client`, ...) as the escape hatch for features this abstraction
deliberately does not model.

## Usage

### 1. Publish and consume

```go
pub, err := driver.NewPublisher(ctx, "orders")
if err != nil {
    return err
}
defer pub.Close()

sub, err := driver.NewSubscriber(ctx, "orders", "order-workers")
if err != nil {
    return err
}
defer sub.Close()

_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    log.Printf("received %s: %s", m.Key, m.Payload)
    return nil
})

return pub.Publish(ctx, &messaging.Message{
    Key:     "order-1",
    Payload: []byte(`{"id":1}`),
})
```

An empty `group` means broadcast (every subscriber gets every message) where
the broker supports it; brokers whose queue is intrinsically
competing-consumer (RabbitMQ) ignore the parameter — each starter documents
its exact interpretation.

### 2. Idempotent consumption

```go
// Driver publishers already stamp message-id when absent; set it yourself
// only if you own the id:
msg.SetHeader(messaging.HeaderMessageID, messaging.NewMessageID())
// consumer side — at-least-once means the same id may arrive twice:
if seen := store.CheckOnce(m.Header(messaging.HeaderMessageID)); seen {
    return nil // duplicate delivery, ack and move on
}
if m.Header(messaging.HeaderDeliveryAttempt) == "5" {
    // broker has redelivered 5 times — skip retries, dead-letter now
}
```

### 3. Retry, dead-letter, panic-guard

```go
dlq, _ := driver.NewPublisher(ctx, "orders.dlq")
_ = sub.Subscribe(ctx, messaging.DeadLetter(
    messaging.Recover(handle),
    dlq,
    messaging.RetryPolicy{MaxRetries: 2, InitialInterval: 100 * time.Millisecond},
))
```

Wrap order matters: `Retry(Recover(h), p)` (or `DeadLetter` wrapping
`Recover`, as above) converts a panic to an error once so it is retried
like any other failure.

Brokers with native dead-lettering (RabbitMQ DLX, RocketMQ DLQ topics) can be
configured instead — both routes carry the original payload; only the failure
metadata differs.

## What the abstraction deliberately does not model

- **No delay / scheduled messages.** Broker support is wildly uneven; compose
  `cloud/experimental/outbox` + `cloud/scheduling` for broker-independent
  delayed delivery instead.
- **No batch consume.** `Publish` is one message at a time (`PublishBatch`
  batches the send side); consumers wanting throughput get batching inside
  the driver, behind the same per-message Handler.
- **No schema registry, no typed payloads.** `Payload` is opaque; marshal
  where you publish.
- **No Supplier/Function/Consumer sugar layer** — the raw-client escape hatch
  stays one import away.
- MQTT (3.1.1) has no per-message metadata on the wire: its starter is
  payload-only and skips trace propagation. Kafka (franz-go) fixes
  topics/group on the client: one client bean = one logical consumer; build
  multiple clients (or use the sarama variant) for many groups.
