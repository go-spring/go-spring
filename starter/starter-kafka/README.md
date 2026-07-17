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

### 2. Configure the Kafka Client

Add Kafka configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.kafka.brokers=127.0.0.1:9092
spring.kafka.topic=hello
spring.kafka.group=hello-group
```

### 3. Inject the Kafka Client

Refer to the [example.go](example/example.go) file.

```go
import "github.com/twmb/franz-go/pkg/kgo"

type Service struct {
    Client *kgo.Client `autowire:"__default__"`
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

## Advanced Features

* **Supports multiple Kafka clients**: You can define multiple Kafka clients under
  `spring.kafka.instances` in the configuration file and reference them by name.
