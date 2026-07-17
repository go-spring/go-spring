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

### 2. Configure the Pulsar Client

Add Pulsar configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.pulsar.url=pulsar://127.0.0.1:6650
```

### 3. Inject the Pulsar Client

Refer to the [example.go](example/example.go) file.

```go
import "github.com/apache/pulsar-client-go/pulsar"

type Service struct {
    Client pulsar.Client `autowire:"__default__"`
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

## Advanced Features

* **Supports multiple Pulsar clients**: You can define multiple Pulsar clients under
  `spring.pulsar.instances` in the configuration file and reference them by name.
