# starter-pulsar

[English](README.md) | [中文](README_CN.md)

`starter-pulsar` provides a Pulsar client wrapper based on github.com/apache/pulsar-client-go,
making it easy to integrate and use Apache Pulsar in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-pulsar
```

## Quick Start

### 1. Import the `starter-pulsar` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-pulsar"
```

### 2. Configure the Pulsar Clients

Define one or more named clients under `spring.pulsar.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.pulsar.a.url=pulsar://127.0.0.1:6650
spring.pulsar.b.url=pulsar://127.0.0.1:6650
```

### 3. Inject the Pulsar Client

Refer to the [example.go](example/example.go) file. Each named instance is registered
as a `pulsar.Client` bean under that name; inject the one you need by name.

```go
import "github.com/apache/pulsar-client-go/pulsar"

type Service struct {
    Client pulsar.Client `autowire:"a"`
}
```

### 4. Use the Pulsar Client

Refer to the [example.go](example/example.go) file. Create a producer or a
consumer from the shared client and close them when done.

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

### Metrics (native Prometheus)

pulsar-client-go has no OTel contrib, but the client always emits
producer/consumer/connection metrics into a `prometheus.Registerer`. go-spring's
observability layer ([starter-otel](../starter-otel)) is a separate OTel
pipeline, so rather than force a fragile bridge, this starter exposes pulsar's
native metrics the pure-Prometheus way — the same approach the
[contrib/go-zero](../../contrib/go-zero) example uses.

Enable a per-instance `/metrics` endpoint in the configuration file:

```properties
spring.pulsar.a.metrics.enabled=true
spring.pulsar.a.metrics.port=9091
spring.pulsar.a.metrics.path=/metrics
```

Each instance gets its own `prometheus.Registry` and standalone HTTP server, so
several clients never collide on identical `pulsar_client_*` metric names; give
each a distinct `port`. The endpoint is disabled by default so importing the
starter never binds a port unexpectedly, and the server is shut down when the
client bean is destroyed. Point Prometheus at `http://<host>:<port>/metrics`.

### Tracing (native OTel helpers)

pulsar exposes no span injection point of its own, so message-level tracing is
done with small call-site helpers built on the OTel API. They ride the global
`TracerProvider` and propagator installed by starter-otel and carry the W3C trace
context in the message `Properties`; without starter-otel they are no-ops and
change no message bytes.

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

## Messaging Binder

Beyond the raw client, this starter can expose a broker-neutral
`messaging.Binder` (from `go-spring.org/spring/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
Pulsar client API — swapping the broker underneath does not touch business code.

Register the binder as a bean from a `pulsar.Client` (select the named instance
with `gs.TagArg`):

```go
import (
    "go-spring.org/spring/gs"
    StarterPulsar "go-spring.org/starter-pulsar"
)

gs.Provide(StarterPulsar.NewBinder, gs.TagArg("a"))
```

Then publish and subscribe through the envelope:

```go
pub, _ := binder.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := binder.NewSubscriber(ctx, "orders", "workers")
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
features the binder does not model.

## Advanced Features

* **Multiple Pulsar clients**: Every entry under `spring.pulsar` becomes
  an independently configured `pulsar.Client` bean; inject them by name to talk to
  different clusters.
