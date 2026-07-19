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

Import path: `go-spring.org/stdlib/messaging`.

```go
package main

import (
    "context"
    "log"

    "go-spring.org/stdlib/messaging"
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
