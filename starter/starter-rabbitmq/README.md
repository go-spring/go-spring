# starter-rabbitmq

[English](README.md) | [中文](README_CN.md)

`starter-rabbitmq` provides a RabbitMQ connection wrapper based on github.com/rabbitmq/amqp091-go,
making it easy to integrate and use RabbitMQ in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-rabbitmq
```

## Quick Start

### 1. Import the `starter-rabbitmq` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-rabbitmq"
```

### 2. Configure the RabbitMQ Instances

Define one or more named instances under `spring.rabbitmq.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.rabbitmq.a.url=amqp://guest:guest@127.0.0.1:5672/
spring.rabbitmq.b.url=amqp://guest:guest@127.0.0.1:5672/
```

### 3. Inject the RabbitMQ Connection

Refer to the [example.go](example/example.go) file. Each named instance is registered
as an `*amqp.Connection` bean under that name; inject the one you need by name.

```go
import amqp "github.com/rabbitmq/amqp091-go"

type Service struct {
    Conn *amqp.Connection `autowire:"a"`
}
```

### 4. Use the RabbitMQ Connection

Refer to the [example.go](example/example.go) file. Channels are cheap and not
thread-safe, so open one per goroutine/operation from the shared connection.

```go
ch, err := s.Conn.Channel()
defer ch.Close()
_, _ = ch.QueueDeclare("hello", false, false, false, false, nil)
_ = ch.PublishWithContext(ctx, "", "hello", false, false, amqp.Publishing{Body: []byte("value")})
```

## Core Features

The [example](example/example.go) demonstrates three core RabbitMQ patterns:

1. **Default-exchange publish/consume** — publish a message to the default exchange using the
   queue name as the routing key, then pull it back with `ch.Get`.
2. **Direct exchange + routing key binding** — declare a `direct` exchange, bind a queue with a
   routing key (e.g. `info`), publish to the exchange with that key, and consume from the bound
   queue.
3. **QoS + manual ack** — call `ch.Qos(1, 0, false)` to enforce a prefetch of one, consume with
   `autoAck=false`, and explicitly call `msg.Ack(false)` after processing.

## Observability

Distributed tracing is available through native OTel helpers that ride the
global `TracerProvider` and propagator installed by
[starter-otel](../starter-otel). Without starter-otel they are no-ops and change
no message bytes, so instrumenting your code is a safe, zero-config opt-in.

```go
import starter "go-spring.org/starter-rabbitmq"

// Producer: start a span and inject W3C trace context into the message headers.
pub := amqp.Publishing{ContentType: "text/plain", Body: []byte("v")}
ctx, span := starter.StartPublishSpan(ctx, exchange, routingKey, &pub)
err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, pub)
starter.EndSpan(span, err)

// Consumer: continue the trace carried in the delivery headers.
ctx, span := starter.StartConsumeSpan(ctx, &delivery)
err := handle(ctx, delivery)
starter.EndSpan(span, err)
```

Why call-site helpers instead of a wrapped channel/publisher:

* `amqp091-go` has no official OTel instrumentation, and the starter's bean is an
  `*amqp.Connection` — channels, publishes and deliveries are all created by the
  caller, so there is no seam to auto-instrument. A wrapper would have to
  re-expose the entire `Channel` surface and still miss raw-connection usage.
* `amqp.Publishing` carries a `Headers` table that every delivery echoes back, so
  instrumenting at the call site — where you already hold the `Publishing` /
  `Delivery` — is what propagates trace context and links producer to consumer
  across services.

## Messaging Binder

Beyond the raw connection, this starter can expose a broker-neutral
`messaging.Binder` (from `go-spring.org/spring/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
`amqp` API — swapping the broker underneath does not touch business code.

Register the binder as a bean from an `*amqp.Connection` (select the named
instance with `gs.TagArg`):

```go
import (
    "go-spring.org/spring/gs"
    StarterRabbitMQ "go-spring.org/starter-rabbitmq"
)

gs.Provide(StarterRabbitMQ.NewBinder, gs.TagArg("a"))
```

Then publish and subscribe through the envelope:

```go
pub, _ := binder.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := binder.NewSubscriber(ctx, "orders", "")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // handle m.Payload / m.Headers
    return nil
})
```

`destination` and `source` are queue names; the publisher sends to the default
exchange keyed by the queue name, and both sides declare the queue idempotently.
Each publisher and subscriber owns its own channel (channels are not
concurrency-safe). A RabbitMQ queue is itself the competing-consumer group, so
`group` is unused. The subscriber ranges over deliveries — a handler error nacks
with requeue while success acks. Trace context rides the message headers, so with
starter-otel a trace links producer to consumer. The raw `*amqp.Connection` bean
stays available for custom exchanges, routing, publisher confirms and other AMQP
features the binder does not model.

## Advanced Features

* **Multiple RabbitMQ instances**: Every entry under `spring.rabbitmq`
  becomes an independently configured `*amqp.Connection` bean; inject them by name
  to talk to different brokers or vhosts.
