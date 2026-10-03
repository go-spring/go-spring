# starter-rocketmq

[English](README.md) | [中文](README_CN.md)

`starter-rocketmq` provides [RocketMQ](https://rocketmq.apache.org/) support for
Go-Spring: multi-instance `rocketmq.Client` beans with an opt-in startup ping
probe, optional ACL credentials, a broker-neutral `messaging.Driver`, OTel
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
spring.rocketmq.instances.a.name-servers=127.0.0.1:9876
spring.rocketmq.instances.a.send-timeout=5s

# ACL (optional, access-key and secret-key must be set together)
# spring.rocketmq.instances.a.access-key=rocketmq
# spring.rocketmq.instances.a.secret-key=12345678
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

- **Multi-instance clients** — every `spring.rocketmq.instances.<name>` entry is its own
  bean with independent settings.
- **Startup ping probe (opt-in)** — with `ping=true` a TCP dial against the name server list at
  boot catches wrong addresses before the first message; off by default
  so a not-yet-up broker does not block startup.
- **Lifecycle management** — everything created through the client
  (producers, push consumers) is registered and shut down by the starter.
- **Log bridge** — the client library's internal logs are routed into
  go-spring's log.

## Messaging Driver

`NewDriver` adapts the client to the broker-neutral
`cloud/messaging` abstraction, so business code stays free of the
RocketMQ API. destination/source strings are topics; the group maps to a
RocketMQ consumer group (clustering mode).

```go
func ProvideDriver(cl *StarterRocketmq.Client) messaging.Driver {
    return StarterRocketmq.NewDriver(cl, nil)
}
```

```go
sub, _ := driver.NewSubscriber(ctx, "orders", "order-service")
_ = sub.Subscribe(ctx, func(ctx context.Context, msg *messaging.Message) error {
    fmt.Println(string(msg.Payload), msg.Headers)
    return nil // a non-nil error asks RocketMQ to redeliver
})
pub, _ := driver.NewPublisher(ctx, "orders")
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("...")})
```

Each publish and consume declares its operation and runs it under the client's
resilience executor (the single emitter — see [Observability](#observability)),
and injects/extracts the W3C trace context and the load-test marker into the
message user properties, so traces link producer to consumer and synthetic load
stays recognisable downstream.

## Observability

Every publish and consume that flows through this starter DECLARES its operation
on the call — the direction (`messaging.operation`), the backend
(`messaging.system=rocketmq`) and the topic as per-call detail — and the
resilience layer, the single emitter on the executor chain, emits the signals
from that declaration: the call span, the call-level
`messaging.client.operation.duration` histogram, the attempt-level
`messaging.client.attempt.duration` histogram (one record per retry, so
retry/backoff cost never inflates downstream latency), the in-flight gauge, the
`resilience.client.calls` counter and one access log per call. Both the
[Messaging Driver](#messaging-driver) path and the raw `GuardedSend` seam
declare through the same helper, so the two never double-report a message. They
ride the global `TracerProvider` installed by
[starter-otel](../starter-otel); without it the span and metrics are no-ops,
while the access log always writes through go-spring's log.

The starter itself emits nothing per call — it declares the identity and lets
the resilience layer emit.

For a raw send or consumer you drive yourself, the manual helpers
`StartProducerSpan` / `StartConsumerSpan` / `EndSpan` wrap them in OTel spans,
with the W3C trace context carried in the message user properties. See
`example-otel/`.

## Resilience

`GuardedSend` routes a synchronous send through the governance executor
attached to the client (rate limit, circuit breaking, fault injection):

```go
res, err := StarterRocketmq.GuardedSend(ctx, s.Client, p, msg)
```

When governance is not wired in (no `cloud/governance` bean in the container)
this behaves exactly like `p.SendSync(ctx, msg)`.

The [Messaging Driver](#messaging-driver) publish and consume paths route
through the same executor, so driver traffic is protected and declared too.

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.rocketmq.instances.orders.name-servers=10.0.0.1:9876
spring.rocketmq.instances.events.name-servers=10.0.0.2:9876
```

```go
type Service struct {
    Orders *StarterRocketmq.Client `autowire:"orders"`
    Events *StarterRocketmq.Client `autowire:"events"`
}
```

**Custom driver** — replace client assembly (e.g. to inject a custom
`primitive.NsResolver`) by providing your own `Driver` bean (its constructor
returns `StarterRocketmq.Driver`). It is an optional container bean: every
client under `spring.rocketmq.instances.*` is built through it, and the starter falls
back to its bundled `DefaultDriver` only when no `Driver` bean is present.
Embed `StarterRocketmq.DefaultDriver` and delegate `CreateClient` so the
default assembly is preserved. `CreateClient` takes the container's
`cloud.ClientParams` bundle (see `go-spring.org/cloud`) and passes
it through, so the client is assembled complete — identity and executor — in one
step; delegate it as-is and governance keeps applying:

```go
func init() {
    gs.Provide(func() StarterRocketmq.Driver {
        return myDriver{}
    })
}

type myDriver struct {
    StarterRocketmq.DefaultDriver
}

func (d myDriver) CreateClient(ctx context.Context, c StarterRocketmq.Config,
    params cloud.ClientParams) (*StarterRocketmq.Client, error) {
    return d.DefaultDriver.CreateClient(ctx, c, params)
}
```

**Health** — like the other MQ starters, no `health.Indicator` is registered
here (there is no cheap broker probe that works on every cluster topology);
export one from the application (e.g. a `NewProducer`/`Shutdown` round trip)
when you use `starter-actuator`.

## Design Notes

* **Ordered and transactional messaging lives in the raw SDK.** The
  `messaging.Driver` adapter consumes concurrently; for orderly or transactional
  sends use the raw `Client`. Message payloads stay `[]byte` — serialization is
  yours.
* **`instance-name` controls connection pooling.** Left empty, the SDK rewrites
  the shared "DEFAULT" to a per-producer/consumer `PID#nano`, which is safe in
  multi-producer processes; set it explicitly to make all remoting clients share
  one connection pool.
* **The ping probe is a TCP dial, not a broker round trip.** It catches wrong
  addresses at boot but not ACL or credential errors — deliberately cheap,
  side-effect-free and topology-independent.
