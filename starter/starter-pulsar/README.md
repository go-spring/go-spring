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

Define one or more named clients under `spring.pulsar.instances.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.pulsar.instances.a.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.b.url=pulsar://127.0.0.1:6650
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
spring.pulsar.instances.a.metrics.enabled=true
spring.pulsar.instances.a.metrics.port=9091
spring.pulsar.instances.a.metrics.path=/metrics
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

## Advanced Features

* **Multiple Pulsar clients**: Every entry under `spring.pulsar.instances` becomes
  an independently configured `pulsar.Client` bean; inject them by name to talk to
  different clusters.
