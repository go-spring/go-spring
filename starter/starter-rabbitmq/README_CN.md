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

## 消息 Binder

除原生连接外,本 starter 还可暴露一个 broker 中立的 `messaging.Binder`
(来自 `go-spring.org/stdlib/messaging`),让业务代码收发 `*messaging.Message`
信封而不依赖 `amqp` API —— 底层换 broker 时业务代码无需改动。

从 `*amqp.Connection` 注册一个 binder bean(用 `gs.TagArg` 选取具名实例):

```go
import (
    "go-spring.org/spring/gs"
    StarterRabbitMQ "go-spring.org/starter-rabbitmq"
)

gs.Provide(StarterRabbitMQ.NewBinder, gs.TagArg("a"))
```

然后通过信封收发:

```go
pub, _ := binder.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := binder.NewSubscriber(ctx, "orders", "")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // 处理 m.Payload / m.Headers
    return nil
})
```

`destination` 与 `source` 都是队列名;publisher 走默认 exchange、routingKey 取队列名,
两侧都会幂等地 `QueueDeclare`。每个 publisher 与 subscriber 各自持有一个 channel
(channel 非并发安全)。RabbitMQ 队列本身就是竞争消费组,因此 `group` 不使用。订阅方
range 消费 deliveries —— handler 出错则 Nack 并 requeue,成功则 Ack。trace context 骑在
消息 headers 上,配合 starter-otel 即可串联 producer 与 consumer 链路。原生
`*amqp.Connection` bean 仍可用于自定义 exchange、路由、publisher confirm 等 binder 未建模
的 AMQP 能力。

## 高级功能

* **多 RabbitMQ 实例**：`spring.rabbitmq.instances` 下的每一项都会成为一个独立配置的
  `*amqp.Connection` bean，按名称注入即可访问不同的 broker 或 vhost。
