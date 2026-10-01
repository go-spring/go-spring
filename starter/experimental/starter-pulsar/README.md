# starter-pulsar

[English](README.md) | [中文](README_CN.md)

`starter-pulsar` provides a Pulsar client wrapper based on
github.com/apache/pulsar-client-go for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-pulsar
```

## Quick Start

### 1. Import the `starter-pulsar` Package

```go
import _ "go-spring.org/starter-pulsar"
```

### 2. Configure the Pulsar Clients

Define one or more named clients under `spring.pulsar.instances.<name>` in your
project's [configuration file](example/conf/app.properties):

```properties
spring.pulsar.instances.a.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.b.url=pulsar://127.0.0.1:6650
```

### 3. Inject the Pulsar Client

Each named instance is registered as a `pulsar.Client` bean under that name; inject the one you need by name.

```go
import "github.com/apache/pulsar-client-go/pulsar"

type Service struct {
    Client pulsar.Client `autowire:"a"`
}
```

### 4. Use the Pulsar Client

Create a producer or consumer from the shared client and close them when done.

```go
producer, _ := s.Client.CreateProducer(pulsar.ProducerOptions{Topic: "hello"})
defer producer.Close()
_, _ = producer.Send(ctx, &pulsar.ProducerMessage{Payload: []byte("value")})

consumer, _ := s.Client.Subscribe(pulsar.ConsumerOptions{
    Topic:            "hello",
    SubscriptionName: "hello-sub",
    Type:             pulsar.Shared,
})
defer consumer.Close()
msg, _ := consumer.Receive(ctx)
consumer.Ack(msg)
```

## Observability

Every publish and consume that flows through this starter DECLARES its operation
on the call — the direction (`messaging.operation`), the backend
(`messaging.system=pulsar`) and the topic as per-call detail — and the resilience
layer, the single emitter on the executor chain, emits the signals from that
declaration: the call span, the call-level `messaging.client.operation.duration`
histogram, the attempt-level `messaging.client.attempt.duration` histogram (one
record per retry, so retry/backoff cost never inflates downstream latency), the
in-flight gauge, the `resilience.client.calls` counter and one access log per
call. Both the [messaging.Driver](#messaging-driver) path and the raw
`GuardedSend` seam declare through the same helper, so the two never
double-report a message. They ride the global `TracerProvider` installed by
[starter-otel](../starter-otel); without it the span and metrics are no-ops,
while the access log always writes through go-spring's log.

The starter itself emits nothing per call — it declares the identity and lets
the resilience layer emit. Pulsar's native `pulsar_client_*` metrics (below)
stay in the starter: they are library-native connection/producer/consumer
stats, not per-call signals.

### Metrics (native Prometheus)

pulsar-client-go has no OTel contrib, but the client always emits
producer/consumer/connection metrics into a `prometheus.Registerer`. go-spring's
observability layer ([starter-otel](../../starter-otel)) is a separate OTel
pipeline, so rather than force a fragile bridge, this starter exposes pulsar's
native metrics the pure-Prometheus way — the same approach the
[contrib/go-zero](../../../contrib/go-zero) example uses.

Enable a per-instance `/metrics` endpoint in the configuration file:

```properties
spring.pulsar.instances.a.metrics.enabled=true
spring.pulsar.instances.a.metrics.port=9091
spring.pulsar.instances.a.metrics.path=/metrics
```

Each instance gets its own `prometheus.Registry` and standalone HTTP server, so
several clients never collide on identical `pulsar_client_*` metric names; give
each a distinct `port`. The endpoint is disabled by default so importing the
starter never binds a port unexpectedly, and the server is shut down when the
client bean is destroyed. Point Prometheus at `http://<host>:<port>/metrics`.

### Tracing (manual helpers)

Code that drives the raw `pulsar.Client` itself — bypassing the driver and the
`GuardedSend` seam — can open its own span with the call-site helpers below. They
are built on the OTel API, ride the global `TracerProvider` and propagator
installed by starter-otel, and carry the W3C trace context in the message
`Properties`; without starter-otel they are no-ops and change no message bytes.
A send routed through `GuardedSend` is already spanned by the resilience layer
from the operation it declares, so do not wrap the same send in both.

```go
import starter "go-spring.org/starter-pulsar"

// Producer: start a span and inject trace context into the message properties.
msg := &pulsar.ProducerMessage{Payload: []byte("v")}
ctx, span := starter.StartProducerSpan(ctx, msg)
_, err := producer.Send(ctx, msg)
starter.EndSpan(span, err)

// Consumer: continue the trace carried in the message properties.
ctx, span := starter.StartConsumerSpan(ctx, msg)
err := handle(ctx, msg)
starter.EndSpan(span, err)
```

## Messaging Driver

Beyond the raw client, this starter can expose a broker-neutral
`messaging.Driver` (from `go-spring.org/cloud/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
Pulsar client API — swapping the broker underneath does not touch business code.

Register the driver as a bean from a `pulsar.Client` (select the named instance
with `gs.TagArg`):

```go
import (
    "go-spring.org/spring/gs"
    StarterPulsar "go-spring.org/starter-pulsar"
)

gs.Provide(StarterPulsar.NewDriver, gs.TagArg("a"))
```

Then publish and subscribe through the envelope:

```go
pub, _ := driver.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := driver.NewSubscriber(ctx, "orders", "workers")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // handle m.Payload / m.Headers
    return nil
})
```

`destination` and `source` are topics. The subscriber `group` becomes the Pulsar
subscription name in `Shared` mode (competing consumers); an empty group derives
`go-spring-<topic>`. Each publisher owns a Producer and each subscriber owns a
Consumer with a background receive loop — a handler error nacks the message for
redelivery while success acks it. Trace context rides the message properties, so
with starter-otel a trace links producer to consumer. The raw `pulsar.Client`
bean stays available for readers, the admin API, schemas and other Pulsar
features the driver does not model.

## Advanced Features

* **Multiple Pulsar clients**: Every entry under `spring.pulsar` becomes
  an independently configured `pulsar.Client` bean; inject them by name to talk to
  different clusters.
