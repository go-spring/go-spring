# starter-kafka-sarama

[English](README.md) | [中文](README_CN.md)

`starter-kafka-sarama` provides a Kafka client wrapper based on
github.com/IBM/sarama, making it easy to integrate and use Kafka in a Go-Spring
application.

It shares the `spring.kafka` configuration prefix with the franz-go based
[starter-kafka](../starter-kafka), so switching between the two implementations
only requires swapping the imported package.

## Installation

```bash
go get go-spring.org/starter-kafka-sarama
```

## Quick Start

### 1. Import the `starter-kafka-sarama` package

See [example.go](example/example.go).

```go
import _ "go-spring.org/starter-kafka-sarama"
```

### 2. Configure the Kafka client

Add the Kafka configuration to your project's
[configuration file](example/conf/app.properties), for example:

```properties
spring.kafka.a.brokers=127.0.0.1:9092
spring.kafka.a.version=3.7.0
spring.kafka.b.brokers=127.0.0.1:9092
```

> Each entry under `spring.kafka.<name>` becomes an independently
> configured `sarama.Client` bean registered under that name.
> `version` must match the target cluster for features such as
> consumer groups to behave correctly. When omitted, sarama's own default is
> used.

### 3. Inject the Kafka client

See [example.go](example/example.go). Inject an instance by its name.

```go
import "github.com/IBM/sarama"

type Service struct {
    Client sarama.Client `autowire:"a"`
}
```

### 4. Use the Kafka client

See [example.go](example/example.go). sarama has no single object that both
produces and consumes; instead, derive a producer or consumer from the shared
`sarama.Client` via the `*FromClient` constructors:

```go
producer, _ := sarama.NewSyncProducerFromClient(s.Client)
defer producer.Close()
producer.SendMessage(&sarama.ProducerMessage{
    Topic: "hello",
    Value: sarama.StringEncoder("value"),
})

consumer, _ := sarama.NewConsumerFromClient(s.Client)
defer consumer.Close()
pc, _ := consumer.ConsumePartition("hello", 0, sarama.OffsetOldest)
defer pc.Close()
msg := <-pc.Messages()
fmt.Println(string(msg.Value))
```

## Observability

Distributed tracing is available through native OTel helpers that ride the
global `TracerProvider` and propagator installed by
[starter-otel](../starter-otel). Without starter-otel they are no-ops and change
no message bytes, so instrumenting your code is a safe, zero-config opt-in.

```go
import starter "go-spring.org/starter-kafka-sarama"

// Producer: start a span and inject W3C trace context into the record headers.
msg := &sarama.ProducerMessage{Topic: "hello", Value: sarama.StringEncoder("v")}
_, span := starter.StartProducerSpan(ctx, msg)
_, _, err := producer.SendMessage(msg)
starter.EndSpan(span, err)

// Consumer: continue the trace carried in the record headers.
ctx, span := starter.StartConsumerSpan(ctx, msg)
err := handle(ctx, msg)
starter.EndSpan(span, err)
```

Why call-site helpers instead of a wrapped producer/consumer:

* The only official OTel instrumentation for sarama, `otelsarama`, is
  **deprecated** and still pinned to the abandoned `github.com/Shopify/sarama`
  module. This starter uses `github.com/IBM/sarama`; the two are distinct Go
  types, so `otelsarama.WrapSyncProducer` cannot wrap an IBM producer and pulling
  it in would drag a second, conflicting sarama fork into the build.
* `sarama.SyncProducer.SendMessage` takes no `context.Context`, so a producer
  *wrapper* has nowhere to receive request-scoped context from and could only
  emit disconnected root spans. Passing `ctx` explicitly at the call site is what
  lets traces link across services.

**Metrics**: sarama emits metrics through its own `go-metrics` registry
(`sarama.Config.MetricRegistry`), a system unrelated to OTel/Prometheus. Bridging
it requires a third-party `go-metrics`→Prometheus adapter, so it is intentionally
left out of scope here rather than shipped as a fragile wrapper.

## Advanced

* **Multiple Kafka clients**: define multiple clients under
  `spring.kafka` in the configuration file and reference them by name.
