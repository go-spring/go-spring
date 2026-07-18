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

## 可观测性

分布式链路追踪通过原生 OTel 辅助函数提供,依赖
[starter-otel](../starter-otel) 安装的全局 `TracerProvider` 与传播器。未引入
starter-otel 时它们为 no-op,也不改动任何消息字节,因此埋点是安全的零配置可选项。

```go
import starter "go-spring.org/starter-rabbitmq"

// 生产者:开启 span 并把 W3C 链路上下文注入消息 headers。
pub := amqp.Publishing{ContentType: "text/plain", Body: []byte("v")}
ctx, span := starter.StartPublishSpan(ctx, exchange, routingKey, &pub)
err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, pub)
starter.EndSpan(span, err)

// 消费者:延续 delivery headers 中携带的链路上下文。
ctx, span := starter.StartConsumeSpan(ctx, &delivery)
err := handle(ctx, delivery)
starter.EndSpan(span, err)
```

为什么用调用点辅助函数,而不是包装 channel/publisher:

* `amqp091-go` 没有官方 OTel instrumentation,而 starter 的 bean 是
  `*amqp.Connection`——channel、publish、delivery 全由调用方创建,没有可自动埋点的
  接缝。包装器需要重新暴露整个 `Channel` 接口,而且仍会漏掉对裸连接的使用。
* `amqp.Publishing` 携带 `Headers` 表且每个 delivery 都会回传,因此在调用点埋点——
  即你已持有 `Publishing` / `Delivery` 的地方——才能传播链路上下文,把生产者与消费者
  跨服务串联起来。

## 高级功能

* **多 RabbitMQ 实例**：`spring.rabbitmq.instances` 下的每一项都会成为一个独立配置的
  `*amqp.Connection` bean，按名称注入即可访问不同的 broker 或 vhost。
