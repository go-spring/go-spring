# starter-rabbitmq

[English](README.md) | [中文](README_CN.md)

`starter-rabbitmq` provides a RabbitMQ connection wrapper based on
github.com/rabbitmq/amqp091-go for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-rabbitmq
```

## Quick Start

### 1. Import the `starter-rabbitmq` Package

```go
import _ "go-spring.org/starter-rabbitmq"
```

### 2. Configure the RabbitMQ Instances

Define one or more named instances under `spring.rabbitmq.instances.<name>` in your
project's [configuration file](example/conf/app.properties):

```properties
spring.rabbitmq.instances.a.url=amqp://guest:guest@127.0.0.1:5672/
spring.rabbitmq.instances.b.url=amqp://guest:guest@127.0.0.1:5672/
```

### 3. Inject the RabbitMQ Connection

Each named instance is registered as an `*amqp.Connection` bean under that name; inject the one you need by name.

```go
import amqp "github.com/rabbitmq/amqp091-go"

type Service struct {
    Conn *amqp.Connection `autowire:"a"`
}
```

### 4. Use the RabbitMQ Connection

Channels are cheap and not thread-safe, so open one per goroutine/operation from the shared connection.

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

The starter **declares** the identity of every publish and consume; the signals
themselves — the span, the duration metrics (call-level and attempt-level) and
one access log per call — are **emitted** by the resilience layer, the single
emitter on the executor chain. The starter contains no per-call emission code of
its own.

Declaring is automatic. A publish you route through
`StarterRabbitMQ.GuardedPublish` (and every driver `Publish`, which rides it)
attaches the operation to the `ctx` before running the call under the
connection's resilience executor; so does every driver consume. The executor
opens the `publish` / `consume` span, records `messaging.client.operation.duration`
(the whole call, retries included) and the per-attempt
`messaging.client.attempt.duration`, bumps the `resilience.client.calls` counter,
and writes the access log — tagged `_app_rabbitmq_access`.

Everything rides the global `TracerProvider` / `MeterProvider` and propagator
installed by [starter-otel](../../starter-otel). Without starter-otel they are
no-ops and change no message bytes, so instrumenting your code is a safe,
zero-config opt-in. `GuardedPublish` also injects the current W3C trace context
into the message headers, so a trace links producer to consumer across services.

```go
import starter "go-spring.org/starter-rabbitmq"

// Declare and protect: the executor emits the span/metrics/log and injects the
// W3C trace context into pub.Headers.
pub := amqp.Publishing{ContentType: "text/plain", Body: []byte("v")}
err := starter.GuardedPublish(ctx, conn, ch, exchange, routingKey, false, false, pub)
```

The one signal the starter keeps local is **not** a per-call signal: a
`messaging.client.connection.state_changes` counter, driven by the amqp091
connection's own close / blocked / unblocked notifications, which the resilience
layer never sees.

Why a call-site helper instead of a wrapped channel/publisher:

* `amqp091-go` has no official OTel instrumentation, and the starter's bean is an
  `*amqp.Connection` — channels, publishes and deliveries are all created by the
  caller, so there is no seam to auto-instrument. A wrapper would have to
  re-expose the entire `Channel` surface and still miss raw-connection usage.
* `amqp.Publishing` carries a `Headers` table that every delivery echoes back, so
  declaring at the call site — where you already hold the `Publishing` /
  `Delivery` — is what propagates trace context and links producer to consumer
  across services.

## Messaging Driver

Beyond the raw connection, this starter can expose a broker-neutral
`messaging.Driver` (from `go-spring.org/cloud/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
`amqp` API — swapping the broker underneath does not touch business code.

Register the driver as a bean from an `*amqp.Connection` (select the named
instance with `gs.TagArg`):

```go
import (
    "go-spring.org/spring/gs"
    StarterRabbitMQ "go-spring.org/starter-rabbitmq"
)

gs.Provide(StarterRabbitMQ.NewDriver, gs.TagArg("a"))
```

Then publish and subscribe through the envelope:

```go
pub, _ := driver.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := driver.NewSubscriber(ctx, "orders", "")
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
features the driver does not model.

## Advanced Features

* **Multiple RabbitMQ instances**: Every entry under `spring.rabbitmq`
  becomes an independently configured `*amqp.Connection` bean; inject them by name
  to talk to different brokers or vhosts.
