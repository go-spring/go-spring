# starter-rocketmq

[English](README.md) | [中文](README_CN.md)

`starter-rocketmq` provides [RocketMQ](https://rocketmq.apache.org/) support for
Go-Spring: multi-instance `rocketmq.Client` beans with fail-fast startup
probes, optional ACL credentials, a broker-neutral `messaging.Binder`, OTel
tracing helpers, and opt-in call-site resilience on the synchronous send path.
It is built on the official [rocketmq-client-go](https://github.com/apache/rocketmq-client-go)
v2 client and works against RocketMQ 4.x and 5.x clusters through the
NameServer protocol.

## Installation

```bash
go get go-spring.org/starter-rocketmq
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-rocketmq"
```

### 2. Configure

```properties
spring.rocketmq.a.name-servers=127.0.0.1:9876
spring.rocketmq.a.send-timeout=5s

# ACL (optional, access-key and secret-key must be set together)
# spring.rocketmq.a.access-key=rocketmq
# spring.rocketmq.a.secret-key=12345678
```

### 3. Inject

```go
type Service struct {
    Client *StarterRocketmq.Client `autowire:"a"`
}
```

### 4. Use

```go
// Producer (raw SDK path, already started)
p, err := s.Client.NewProducer()
defer p.Shutdown()
res, err := p.SendSync(ctx, primitive.NewMessage("topic", []byte("hello")))

// Push consumer (raw SDK path: Subscribe before Start)
c, err := s.Client.NewPushConsumer(consumer.WithGroupName("g"))
err = c.Subscribe("topic", consumer.MessageSelector{Type: consumer.TAG, Expression: "*"}, handler)
err = c.Start()
```

Producers and consumers created through the client inherit the name server
list, credentials and instance name from configuration; both are shut down
automatically when the application closes.

## Core Features

- **Multi-instance clients** — every `spring.rocketmq.<name>` entry is its own
  bean with independent settings.
- **Fail-fast startup probe** — a TCP dial against the name server list at
  boot catches wrong addresses before the first message (disable with
  `fail-fast=false`).
- **Lifecycle management** — everything created through the client
  (producers, push consumers) is registered and shut down by the starter.
- **Log bridge** — the client library's internal logs are routed into
  go-spring's log.

## Messaging Binder

`NewBinder` adapts the client to the broker-neutral
`cloud/messaging` abstraction, so business code stays free of the
RocketMQ API. destination/source strings are topics; the group maps to a
RocketMQ consumer group (clustering mode).

```go
func ProvideBinder(cl *StarterRocketmq.Client) messaging.Binder {
    return StarterRocketmq.NewBinder(cl)
}
```

```go
sub, _ := binder.NewSubscriber(ctx, "orders", "order-service")
_ = sub.Subscribe(ctx, func(ctx context.Context, msg *messaging.Message) error {
    fmt.Println(string(msg.Payload), msg.Headers)
    return nil // a non-nil error asks RocketMQ to redeliver
})
pub, _ := binder.NewPublisher(ctx, "orders")
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("...")})
```

The binder injects/extracts W3C trace context and the load-test marker into
the message user properties, so traces link producer to consumer and
synthetic load stays recognisable downstream.

## Observability

- **Tracing**: `StartProducerSpan` / `StartConsumerSpan` / `EndSpan` wrap raw
  sends and handlers in OTel spans; the binder path is instrumented
  automatically. Everything rides the globals installed by `starter-otel`
  and is a no-op without it. See `example-otel/`.
- **Access log**: the binder emits per-message observations through the
  observe kit at the `brief` level by default.

## Resilience

`GuardedSend` routes a synchronous send through the governance executor
attached to the client (rate limit, circuit breaking, fault injection):

```go
res, err := StarterRocketmq.GuardedSend(ctx, s.Client, p, msg)
```

When `starter-governance` is not imported this behaves exactly like
`p.SendSync(ctx, msg)`.

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.rocketmq.orders.name-servers=10.0.0.1:9876
spring.rocketmq.events.name-servers=10.0.0.2:9876
```

```go
type Service struct {
    Orders *StarterRocketmq.Client `autowire:"orders"`
    Events *StarterRocketmq.Client `autowire:"events"`
}
```

**Custom driver** — replace client assembly (e.g. to inject a custom
`primitive.NsResolver`) by registering your own `Driver` and selecting it with
`driver=<name>`:

```go
func init() {
    StarterRocketmq.RegisterDriver("my-driver", myDriver{})
}
```

**Health** — like the other MQ starters, no `health.Indicator` is registered
here (there is no cheap broker probe that works on every cluster topology);
export one from the application (e.g. a `NewProducer`/`Shutdown` round trip)
when you use `starter-actuator`.
