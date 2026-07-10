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

## Core Features

The [example](example/example.go) demonstrates three core RabbitMQ patterns:

1. **Default-exchange publish/consume** — publish a message to the default exchange using the
   queue name as the routing key, then pull it back with `ch.Get`.
2. **Direct exchange + routing key binding** — declare a `direct` exchange, bind a queue with a
   routing key (e.g. `info`), publish to the exchange with that key, and consume from the bound
   queue.
3. **QoS + manual ack** — call `ch.Qos(1, 0, false)` to enforce a prefetch of one, consume with
   `autoAck=false`, and explicitly call `msg.Ack(false)` after processing.

## Advanced Features

* **Supports multiple RabbitMQ instances**: You can define multiple RabbitMQ instances under
  `spring.rabbitmq.instances` in the configuration file and reference them by name.
