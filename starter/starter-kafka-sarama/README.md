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
spring.kafka.instances.a.brokers=127.0.0.1:9092
spring.kafka.instances.a.version=3.7.0
spring.kafka.instances.b.brokers=127.0.0.1:9092
```

> Each entry under `spring.kafka.instances.<name>` becomes an independently
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

## Advanced

* **Multiple Kafka clients**: define multiple clients under
  `spring.kafka.instances` in the configuration file and reference them by name.
