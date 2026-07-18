# starter-rabbitmq

[English](README.md) | [中文](README_CN.md)

`starter-rabbitmq` 提供了基于 github.com/rabbitmq/amqp091-go 的 RabbitMQ 连接封装，
方便在 Go-Spring 服务中快速集成和使用 RabbitMQ。

## 安装

```bash
go get go-spring.org/starter-rabbitmq
```

## 快速开始

### 1. 引入 `starter-rabbitmq` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-rabbitmq"
```

### 2. 配置 RabbitMQ 实例

在项目的[配置文件](example/conf/app.properties)中，在 `spring.rabbitmq.instances.<name>`
下定义一个或多个具名实例，比如：

```properties
spring.rabbitmq.instances.a.url=amqp://guest:guest@127.0.0.1:5672/
spring.rabbitmq.instances.b.url=amqp://guest:guest@127.0.0.1:5672/
```

### 3. 注入 RabbitMQ 连接

参见 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`*amqp.Connection` bean，按名称注入所需实例即可。

```go
import amqp "github.com/rabbitmq/amqp091-go"

type Service struct {
    Conn *amqp.Connection `autowire:"a"`
}
```

### 4. 使用 RabbitMQ 连接

参见 [example.go](example/example.go) 文件。Channel 开销很小且非并发安全，
应从共享连接上按 goroutine/操作各自打开一个。

```go
ch, err := s.Conn.Channel()
defer ch.Close()
_, _ = ch.QueueDeclare("hello", false, false, false, false, nil)
_ = ch.PublishWithContext(ctx, "", "hello", false, false, amqp.Publishing{Body: []byte("value")})
```

## 核心功能

[example](example/example.go) 演示了 RabbitMQ 的三个核心用法：

1. **默认交换机的发布/消费**：使用队列名作为 routing key 通过默认交换机发布消息，
   再通过 `ch.Get` 将其取回。
2. **Direct 交换机 + 路由键绑定**：声明一个 `direct` 交换机，将队列以路由键（如 `info`）
   绑定，向交换机按该路由键投递消息，然后从绑定的队列中消费。
3. **QoS + 手动 ack**：调用 `ch.Qos(1, 0, false)` 将 prefetch 限制为 1，使用
   `autoAck=false` 消费消息，处理后显式调用 `msg.Ack(false)`。

## 高级功能

* **多 RabbitMQ 实例**：`spring.rabbitmq.instances` 下的每一项都会成为一个独立配置的
  `*amqp.Connection` bean，按名称注入即可访问不同的 broker 或 vhost。
