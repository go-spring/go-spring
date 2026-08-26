# messaging
[English](README.md) | [中文](README_CN.md)

`messaging` is a framework-agnostic, zero-dependency publish/subscribe
abstraction — the Go-idiomatic equivalent of Spring Cloud Stream's binder
model. Application code publishes and consumes `Message` envelopes through a
uniform `Publisher` / `Subscriber` pair, so switching broker (NATS, Kafka,
Pulsar, RabbitMQ, MQTT, ...) is a wiring change rather than a business-code
rewrite.

## Features

- Zero third-party dependencies in the abstraction.
- `Message{Key, Payload, Headers, Timestamp}` broker-neutral envelope. `Headers`
  doubles as the W3C trace-context carrier for observability.
- `Publisher` / `Subscriber` bound to a destination / source at creation time;
  `Binder` opens them against one broker connection.
- `RegisterBinder` / `GetBinder` / `MustGetBinder` — driver-registry idiom for
  callers that want to pick a process-wide binder by configured name. Broker
  starters usually wire the binder as a bean over a live client instead.
- Existing broker starters that implement `Binder`: `starter-nats`,
  `starter-kafka`, `starter-kafka-sarama`, `starter-pulsar`,
  `starter-rabbitmq`, `starter-mqtt`.

## Quick Start

Import path: `go-spring.org/cloud/experimental/messaging`.

```go
package main

import (
    "context"
    "log"

    "go-spring.org/cloud/experimental/messaging"
)

func run(ctx context.Context, binder messaging.Binder) error {
    pub, err := binder.NewPublisher(ctx, "orders")
    if err != nil {
        return err
    }
    defer pub.Close()

    sub, err := binder.NewSubscriber(ctx, "orders", "order-workers")
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
}
```

Obtain the `Binder` from a broker starter (`starter-nats`, `starter-kafka`,
...); those starters also expose their raw client bean (e.g. `*nats.Conn`,
`*kgo.Client`) as an escape hatch for broker-specific features this
abstraction deliberately does not model.

## Retry & Dead-Letter

A handler error is surfaced to the binder's failure path (nack / broker
redelivery) by default. Two composable handlers bound that behavior:

- `Retry(h, RetryPolicy)` — retries the handler in-process up to
  `MaxRetries` times with exponential backoff (`InitialInterval`,
  `Multiplier`, `MaxInterval`); a success on any attempt acks, exhaustion
  returns the last error so the broker takes over.
- `DeadLetter(h, dlq, RetryPolicy)` — after retries are exhausted, publishes
  the message to the `dlq` publisher (typically bound to `"<queue>.dlq"`)
  and acks the original so the broker stops redelivering. The copy carries
  the original headers plus `x-dlq-error` / `x-dlq-retries` / `x-dlq-key`.
  If the DLQ publish itself fails, the original error is returned (nack) —
  losing a dead letter is worse than redelivering.

```go
dlq, _ := binder.NewPublisher(ctx, "orders.dlq")
_ = sub.Subscribe(ctx, messaging.DeadLetter(
    messaging.SafeHandler(handle),
    dlq,
    messaging.RetryPolicy{MaxRetries: 2, InitialInterval: 100 * time.Millisecond},
))
```

Wrap `Retry` OUTSIDE `SafeHandler` (`Retry(SafeHandler(h), p)`) so a panic
converts to an error once and is retried like any other failure. Brokers
with native dead-lettering (RabbitMQ DLX, RocketMQ DLQ topics) can be
configured instead, at which point `DeadLetter` is unnecessary — both routes
carry the original payload; only the failure metadata differs.
