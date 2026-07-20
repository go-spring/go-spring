# starter-kafka

[English](README.md) | [中文](README_CN.md)

`starter-kafka` provides a Kafka client wrapper based on github.com/twmb/franz-go,
making it easy to integrate and use Kafka in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-kafka
```

## Quick Start

### 1. Import the `starter-kafka` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-kafka"
```

### 2. Configure the Kafka Clients

Define one or more named clients under `spring.kafka.instances.<name>` in your
project's [configuration file](example/conf/app.properties), for example:

```properties
spring.kafka.instances.a.brokers=127.0.0.1:9092
spring.kafka.instances.a.topic=hello
spring.kafka.instances.a.group=hello-group
spring.kafka.instances.b.brokers=127.0.0.1:9092
```

### 3. Inject the Kafka Client

Refer to the [example.go](example/example.go) file. Each named instance is registered
as a `*kgo.Client` bean under that name; inject the one you need by name.

```go
import "github.com/twmb/franz-go/pkg/kgo"

type Service struct {
    Client *kgo.Client `autowire:"a"`
}
```

### 4. Use the Kafka Client

Refer to the [example.go](example/example.go) file. The same `*kgo.Client`
produces and consumes records; producing uses `ProduceSync`, consuming uses
`PollFetches`.

```go
rec := &kgo.Record{Topic: "hello", Value: []byte("value")}
_ = s.Client.ProduceSync(ctx, rec).FirstErr()

fetches := s.Client.PollFetches(ctx)
fetches.EachRecord(func(r *kgo.Record) {
    fmt.Println(string(r.Value))
})
```

## Messaging Binder

Beyond the raw client, this starter can expose a broker-neutral
`messaging.Binder` (from `go-spring.org/spring/messaging`), so application code
publishes and consumes `*messaging.Message` envelopes without depending on the
franz-go API — swapping the broker underneath does not touch business code.

Register the binder as a bean from a `*kgo.Client` (select the named instance
with `gs.TagArg`):

```go
import (
    "go-spring.org/spring/gs"
    StarterKafka "go-spring.org/starter-kafka"
)

gs.Provide(StarterKafka.NewBinder, gs.TagArg("a"))
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

franz-go fixes a client's consume topics and group at construction, and one
client is a single consumer, so the subscriber polls the client's configured
topics and `source` selects among them by name while `group` comes from the
client config — **use one client bean per logical consumer**. Trace context
rides record headers via the kotel hooks, so with starter-otel a trace links
producer to consumer. The raw `*kgo.Client` bean stays available for
transactions, the admin API and other Kafka features the binder does not model.

## Advanced Features

* **Multiple Kafka clients**: Every entry under `spring.kafka.instances` becomes an
  independently configured `*kgo.Client` bean; inject them by name to talk to
  different clusters or consumer groups.
