# starter-kafka-sarama

[English](README.md) | [中文](README_CN.md)

`starter-kafka-sarama` provides a Kafka client wrapper based on
github.com/IBM/sarama. Use it to integrate Kafka in a Go-Spring
application.

It binds under the `spring.kafka-sarama` configuration prefix, distinct from the
franz-go based [starter-kafka](../starter-kafka) which binds under `spring.kafka`.

## Installation

```bash
go get go-spring.org/starter-kafka-sarama
```

## Quick Start

### 1. Import the `starter-kafka-sarama` package

See [main.go](example/main.go).

```go
import _ "go-spring.org/starter-kafka-sarama"
```

### 2. Configure the Kafka client

Add the Kafka configuration to your project's
[configuration file](example/conf/app.properties), for example:

```properties
spring.kafka-sarama.instances.a.brokers=127.0.0.1:9092
spring.kafka-sarama.instances.a.version=3.7.0
spring.kafka-sarama.instances.b.brokers=127.0.0.1:9092
```

> Each entry under `spring.kafka-sarama.instances.<name>` becomes an independently
> configured `sarama.Client` bean registered under that name.
> `version` must match the target cluster for features such as
> consumer groups to behave correctly. When omitted, sarama's own default is
> used.

### 3. Inject the Kafka client

See [main.go](example/main.go). Inject an instance by its name.

```go
import "github.com/IBM/sarama"

type Service struct {
    Client sarama.Client `autowire:"a"`
}
```

### 4. Use the Kafka client

See [main.go](example/main.go). sarama has no single object that both
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

This starter **declares** what each publish and consume is; it does not emit
anything itself. The signals — the span, the duration metrics, the access log —
are emitted by the resilience layer (see
[governance](../../cloud/resilience)), the single point on the
executor chain that sees a whole call, retries included. It rides the global
`TracerProvider` and propagator installed by
[starter-otel](../../starter-otel); without starter-otel they are no-ops and
change no message bytes, so instrumenting your code is a safe, zero-config
opt-in.

```go
import starter "go-spring.org/starter-kafka-sarama"

// prop is the process's traffic.Propagator; nil means the default convention.
// Producer: wrap the derived SyncProducer. The wrapper declares the publish,
// injects W3C trace context into the record headers and routes the send through
// the resilience executor, which emits the span, metrics and access log.
producer, _ := sarama.NewSyncProducerFromClient(s.Client)
producer = starter.WrapSyncProducer(s.Client, producer, prop)
msg := &sarama.ProducerMessage{Topic: "hello", Value: sarama.StringEncoder("v")}
_, _, err := producer.SendMessage(msg)

// Consumer: run each received message through Consume. It continues the trace
// carried in the record headers and runs the handler under the executor, which
// emits the consume observation.
select {
case msg := <-pc.Messages():
    err := starter.Consume(ctx, s.Client, msg, prop, func(ctx context.Context) error {
        return handle(ctx, msg)
    })
}
```

**Declared signals.** The operation is declared with
`observability.WithOperation`; the resilience layer then emits, under the
`messaging.client` prefix:

* `messaging.client.operation.duration` — one record per call (retries and
  backoff included), labelled `messaging.system` / `messaging.operation` /
  `status`;
* `messaging.client.attempt.duration` — one record per downstream attempt, so
  the broker's own latency is separate from what retrying cost the caller;
* `messaging.client.active_requests` — in-flight calls;
* the span (named `publish` / `consume`) and one access log per call under the
  `kafka` access tag. The topic rides in the span and the log
  (`messaging.destination.name`) but never as a metric label — it is unbounded.

Why call-site seams instead of a wrapped producer/consumer:

* The only official OTel instrumentation for sarama, `otelsarama`, is
  **deprecated** and still pinned to the abandoned `github.com/Shopify/sarama`
  module. This starter uses `github.com/IBM/sarama`; the two are distinct Go
  types, so `otelsarama.WrapSyncProducer` cannot wrap an IBM producer and pulling
  it in would drag a second, conflicting sarama fork into the build.
* `sarama.SyncProducer.SendMessage` takes no `context.Context`, so a producer
  *wrapper* has nowhere to receive request-scoped context from. `WrapSyncProducer`
  therefore derives the operation from the message the send carries, and the
  publish span is a new root; the trace context shipped in the record headers is
  what links the two sides.

**Metrics**: sarama emits metrics through its own `go-metrics` registry
(`sarama.Config.MetricRegistry`), a system unrelated to OTel/Prometheus. Bridging
it requires a third-party `go-metrics` to Prometheus adapter, so it stays out of
scope rather than shipping as a fragile wrapper.

## Advanced

* **Multiple Kafka clients**: define multiple clients under
  `spring.kafka-sarama` in the configuration file and reference them by name.

## Design Notes

* **One `sarama.Client` serves every role.** Producers, consumer groups and admin
  clients are derived from the shared client (`*FromClient`), so a single metadata
  cache and broker connection pool backs them all; the starter deliberately does
  not pre-create these — you own their lifecycle.
* **`brokers` is required.** There is no localhost fallback — an empty broker list
  is rejected at boot. `version` must be set for anything beyond the baseline
  protocol: SASL mechanisms, headers and the idempotent producer need a minimum
  protocol version.
* **Producer knobs are Sarama-native.** `producer.required-acks`,
  `producer.idempotent` and `producer.compression` map directly onto
  `sarama.Config` fields; the starter adds no abstraction over Sarama's semantics.
* **You close what you derive.** The instance's destroy calls `sarama.Client.Close`
  (releasing broker connections); any producer or consumer group built on top must
  be closed first — their lifecycle is yours.
