# starter-pulsar

[English](README.md) | [中文](README_CN.md)

`starter-pulsar` 基于 github.com/apache/pulsar-client-go 提供了 Pulsar 客户端封装,
方便在 Go-Spring 应用中集成和使用 Apache Pulsar。

## 安装

```bash
go get go-spring.org/starter-pulsar
```

## 快速开始

### 1. 引入 `starter-pulsar` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-pulsar"
```

### 2. 配置 Pulsar 客户端

在项目的[配置文件](example/conf/app.properties)中,在 `spring.pulsar.instances.<name>`
下定义一个或多个具名客户端,例如:

```properties
spring.pulsar.instances.a.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.b.url=pulsar://127.0.0.1:6650
```

### 3. 注入 Pulsar 客户端

参考 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`pulsar.Client` bean,按名称注入所需实例即可。

```go
import "github.com/apache/pulsar-client-go/pulsar"

type Service struct {
    Client pulsar.Client `autowire:"a"`
}
```

### 4. 使用 Pulsar 客户端

参考 [example.go](example/example.go) 文件。从共享的客户端创建生产者或消费者,
使用完毕后关闭它们。

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

## 可观测

### Metrics(原生 Prometheus)

pulsar-client-go 没有 OTel contrib,但客户端始终会把 producer/consumer/连接指标上报到
一个 `prometheus.Registerer`。go-spring 的可观测层([starter-otel](../starter-otel))是
独立的 OTel 流水线,因此与其硬塞一个脆弱的桥接,本 starter 选择用纯 Prometheus 的方式暴露
pulsar 的原生指标——与 [contrib/go-zero](../../contrib/go-zero) 示例一致的做法。

在配置文件中为实例开启 `/metrics` 端点:

```properties
spring.pulsar.instances.a.metrics.enabled=true
spring.pulsar.instances.a.metrics.port=9091
spring.pulsar.instances.a.metrics.path=/metrics
```

每个实例拥有独立的 `prometheus.Registry` 和独立的 HTTP 服务,因此多个客户端不会在相同的
`pulsar_client_*` 指标名上冲突,请为每个实例指定不同的 `port`。该端点默认关闭,避免引入
starter 时意外占用端口;客户端 bean 销毁时对应的服务会被关闭。将 Prometheus 指向
`http://<host>:<port>/metrics` 抓取即可。

### Tracing(原生 OTel 辅助函数)

pulsar 自身没有 span 注入点,因此消息级链路追踪通过基于 OTel API 的调用点辅助函数完成。
它们依赖 starter-otel 安装的全局 `TracerProvider` 与传播器,并把 W3C 链路上下文携带在消息
`Properties` 里;未引入 starter-otel 时它们是空操作,也不会改动任何消息字节。

```go
import starter "go-spring.org/starter-pulsar"

// 生产端:开启 span 并把链路上下文注入到消息 properties。
msg := &pulsar.ProducerMessage{Payload: []byte("v")}
ctx, span := starter.StartProducerSpan(ctx, msg)
_, err := producer.Send(ctx, msg)
starter.EndSpan(span, err)

// 消费端:延续消息 properties 里携带的链路。
ctx, span := starter.StartConsumerSpan(ctx, msg)
err := handle(ctx, msg)
starter.EndSpan(span, err)
```

## 消息 Binder

除原生客户端外,本 starter 还可暴露一个 broker 中立的 `messaging.Binder`
(来自 `go-spring.org/spring/messaging`),让业务代码收发 `*messaging.Message`
信封而不依赖 Pulsar 客户端 API —— 底层换 broker 时业务代码无需改动。

从 `pulsar.Client` 注册一个 binder bean(用 `gs.TagArg` 选取具名实例):

```go
import (
    "go-spring.org/spring/gs"
    StarterPulsar "go-spring.org/starter-pulsar"
)

gs.Provide(StarterPulsar.NewBinder, gs.TagArg("a"))
```

然后通过信封收发:

```go
pub, _ := binder.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := binder.NewSubscriber(ctx, "orders", "workers")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // 处理 m.Payload / m.Headers
    return nil
})
```

`destination` 与 `source` 都是 topic。订阅方的 `group` 映射为 `Shared` 模式下的
Pulsar 订阅名(竞争消费);group 为空时派生 `go-spring-<topic>`。每个 publisher 持有一个
Producer,每个 subscriber 持有一个 Consumer 并跑后台接收循环 —— handler 出错则 Nack 触发
重投,成功则 Ack。trace context 骑在消息 Properties 上,配合 starter-otel 即可串联
producer 与 consumer 链路。原生 `pulsar.Client` bean 仍可用于 reader、admin API、schema
等 binder 未建模的 Pulsar 能力。

## 高级特性

* **多 Pulsar 客户端**:`spring.pulsar.instances` 下的每一项都会成为一个独立配置的
  `pulsar.Client` bean,按名称注入即可访问不同的集群。
