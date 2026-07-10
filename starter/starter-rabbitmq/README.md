# starter-rabbitmq

[English](README.md) | [中文](README_CN.md)

`starter-rabbitmq` provides a RabbitMQ connection wrapper based on github.com/rabbitmq/amqp091-go,
making it easy to integrate and use RabbitMQ in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-rabbitmq
```

## Quick Start

### 1. Import the `starter-rabbitmq` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-rabbitmq"
```

### 2. Configure the RabbitMQ Instance

Add RabbitMQ configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.rabbitmq.url=amqp://guest:guest@127.0.0.1:5672/
```

### 3. Inject the RabbitMQ Connection

Refer to the [example.go](example/example.go) file.

```go
import amqp "github.com/rabbitmq/amqp091-go"

type Service struct {
    Conn *amqp.Connection `autowire:"__default__"`
}
```

### 4. Use the RabbitMQ Connection

Refer to the [example.go](example/example.go) file. Channels are cheap and not
thread-safe, so open one per goroutine/operation from the shared connection.

```go
ch, err := s.Conn.Channel()
defer ch.Close()
_, _ = ch.QueueDeclare("hello", false, false, false, false, nil)
_ = ch.PublishWithContext(ctx, "", "hello", false, false, amqp.Publishing{Body: []byte("value")})
```

## Advanced Features

* **Supports multiple RabbitMQ instances**: You can define multiple RabbitMQ instances under
  `spring.rabbitmq.instances` in the configuration file and reference them by name.
