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

所有经由本 starter 的 publish 与 consume 都会在调用上**声明(declare)**该操作的语义身份
——方向(`messaging.operation`)、后端(`messaging.system=pulsar`)以及作为逐调用细节的
topic —— 而由 resilience 层(executor 链上**唯一的发射点(emitter)**)依据声明发射信号:
调用 span、调用级 `messaging.client.operation.duration` 直方图、尝试级
`messaging.client.attempt.duration` 直方图(每次重试一条记录,因此重试/退避开销不会
计入下游时延)、in-flight 计量、`resilience.client.calls` 计数器,以及每次调用一条访问
日志。messaging.Driver 路径与原生 `GuardedSend` 接缝都走同一个声明辅助函数,因此两者
不会对同一条消息重复上报。这些信号依赖 [starter-otel](../starter-otel) 安装的全局
`TracerProvider`;未引入 starter-otel 时 span 与指标为空操作,访问日志则始终经 go-spring
的 log 写出。

starter 自身不再做任何逐调用发射 —— 它只声明身份,由 resilience 层发射。pulsar 原生的
`pulsar_client_*` 指标(见下)留在 starter 内:它们是库原生的连接/生产者/消费者统计,
而非逐调用信号。

### Metrics(原生 Prometheus)

pulsar-client-go 没有 OTel contrib,但客户端始终会把 producer/consumer/连接指标上报到
一个 `prometheus.Registerer`。go-spring 的可观测层([starter-otel](../../starter-otel))是
独立的 OTel 流水线,因此与其硬塞一个脆弱的桥接,本 starter 选择用纯 Prometheus 的方式暴露
pulsar 的原生指标——与 [contrib/go-zero](../../../contrib/go-zero) 示例一致的做法。

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

### Tracing(手动辅助函数)

对于自行直接操作原生 `pulsar.Client`(绕过 driver 与 `GuardedSend` 接缝)的代码,可用下面的
调用点辅助函数自行开启 span。它们基于 OTel API,依赖 starter-otel 安装的全局
`TracerProvider` 与传播器,并把 W3C 链路上下文携带在消息 `Properties` 里;未引入
starter-otel 时它们是空操作,也不会改动任何消息字节。经 `GuardedSend` 路由的发送已由
resilience 层依据其声明的操作开启 span,不要对同一次发送两者都包。

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

## 消息 Driver

除原生客户端外,本 starter 还可暴露一个 broker 中立的 `messaging.Driver`
(来自 `go-spring.org/cloud/messaging`),让业务代码收发 `*messaging.Message`
信封而不依赖 Pulsar 客户端 API —— 底层换 broker 时业务代码无需改动。

从 `pulsar.Client` 注册一个 driver bean(用 `gs.TagArg` 选取具名实例):

```go
import (
    "go-spring.org/spring/gs"
    StarterPulsar "go-spring.org/starter-pulsar"
)

gs.Provide(StarterPulsar.NewDriver, gs.TagArg("a"))
```

然后通过信封收发:

```go
pub, _ := driver.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("hello")})

sub, _ := driver.NewSubscriber(ctx, "orders", "workers")
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
等 driver 未建模的 Pulsar 能力。

## 高级特性

* **多 Pulsar 客户端**:`spring.pulsar` 下的每一项都会成为一个独立配置的
  `pulsar.Client` bean,按名称注入即可访问不同的集群。
