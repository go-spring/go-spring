# starter-kafka-sarama

[English](README.md) | [中文](README_CN.md)

`starter-kafka-sarama` 基于 github.com/IBM/sarama 提供了 Kafka 客户端封装,
方便在 Go-Spring 应用中集成和使用 Kafka。

它使用 `spring.kafka-sarama` 配置前缀,与基于 franz-go 的
[starter-kafka](../starter-kafka)（使用 `spring.kafka` 前缀）彼此独立。

## 安装

```bash
go get go-spring.org/starter-kafka-sarama
```

## 快速开始

### 1. 引入 `starter-kafka-sarama` 包

参考 [main.go](example/main.go) 文件。

```go
import _ "go-spring.org/starter-kafka-sarama"
```

### 2. 配置 Kafka 客户端

在项目的[配置文件](example/conf/app.properties)中添加 Kafka 配置,例如:

```properties
spring.kafka-sarama.instances.a.brokers=127.0.0.1:9092
spring.kafka-sarama.instances.a.version=3.7.0
spring.kafka-sarama.instances.b.brokers=127.0.0.1:9092
```

> `spring.kafka-sarama.instances.<name>` 下的每个条目都会注册为一个以该名字命名、
> 独立配置的 `sarama.Client` bean。
> `version` 需与目标集群匹配,消费组等特性才能正常工作;
> 不填时使用 sarama 自带的默认版本。

### 3. 注入 Kafka 客户端

参考 [main.go](example/main.go) 文件,按名字注入对应实例。

```go
import "github.com/IBM/sarama"

type Service struct {
    Client sarama.Client `autowire:"a"`
}
```

### 4. 使用 Kafka 客户端

参考 [main.go](example/main.go) 文件。sarama 没有同时生产与消费的单一对象,
需从共享的 `sarama.Client` 通过 `*FromClient` 构造函数派生生产者或消费者:

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

## 可观测

本 starter **声明**每次发布与消费的身份,自身不发射任何信号。信号——span、duration
指标、访问日志——由 resilience 层统一发射(见
[governance](../../cloud/resilience)),它是执行链上唯一能看到完整一次调用
(含重试)的点。它依赖 [starter-otel](../../starter-otel) 安装的全局 `TracerProvider`
与传播器;未引入 starter-otel 时它们是空操作,也不会改动任何消息字节,因此埋点是安全、
零配置的可选项。

```go
import starter "go-spring.org/starter-kafka-sarama"

// prop 为进程的 traffic.Propagator；传 nil 即用默认约定。
// 生产端:包裹派生出的 SyncProducer。包装器声明本次发布、把 W3C 链路上下文
// 注入消息头,并把发送经 resilience executor 路由——由后者发射 span、指标与访问日志。
producer, _ := sarama.NewSyncProducerFromClient(s.Client)
producer = starter.WrapSyncProducer(s.Client, producer, prop)
msg := &sarama.ProducerMessage{Topic: "hello", Value: sarama.StringEncoder("v")}
_, _, err := producer.SendMessage(msg)

// 消费端:把每条收到的消息过一遍 Consume。它延续消息头里携带的链路,并在 executor
// 下执行 handler——由后者发射消费观测。
select {
case msg := <-pc.Messages():
    err := starter.Consume(ctx, s.Client, msg, prop, func(ctx context.Context) error {
        return handle(ctx, msg)
    })
}
```

**声明的信号。** 操作通过 `observability.WithOperation` 声明;resilience 层随后在
`messaging.client` 前缀下发射:

* `messaging.client.operation.duration` —— 每次调用一条(含重试与退避),标签为
  `messaging.system` / `messaging.operation` / `status`;
* `messaging.client.attempt.duration` —— 每次下游尝试一条,把 broker 自身延迟与
  "重试让调用方多花了多少"分开;
* `messaging.client.active_requests` —— 在途调用数;
* span(名为 `publish` / `consume`)与每次调用一条访问日志,位于 `kafka` 访问 tag 下。
  topic 进入 span 与日志(`messaging.destination.name`),但绝不作为指标标签——它无界。

为什么用调用点接缝,而不是包装 producer/consumer:

* sarama 唯一的官方 OTel 埋点包 `otelsarama` 已**废弃**,且仍锁定在被弃用的
  `github.com/Shopify/sarama` 模块。本 starter 使用 `github.com/IBM/sarama`,二者是
  不同的 Go 类型,`otelsarama.WrapSyncProducer` 无法包装 IBM 的 producer,强行引入还会
  把第二个相互冲突的 sarama fork 带进构建。
* `sarama.SyncProducer.SendMessage` 不接收 `context.Context`,因此 producer *包装器*
  无处获取请求级上下文。`WrapSyncProducer` 于是从消息本身推导操作,发布 span 是一个
  新的根;真正把两侧串起来的是随消息头一起发出的链路上下文。

**Metrics**:sarama 通过自带的 `go-metrics` 注册表(`sarama.Config.MetricRegistry`)
上报指标,这套体系与 OTel/Prometheus 无关。桥接它需要第三方 `go-metrics`→Prometheus
适配器,因此这里有意不纳入范围,而不是硬塞一个脆弱的包装。

## 高级特性

* **支持多个 Kafka 客户端**:可以在配置文件中的 `spring.kafka-sarama` 下定义多个
  Kafka 客户端,并通过名称引用它们。

## 设计说明

* **一个 `sarama.Client` 服务所有角色。** producer、consumer group、admin 客户端都从共享 client
  派生（`*FromClient`），一套 metadata 缓存与 broker 连接池为其服务；starter 刻意不预建它们——
  生命周期归你。
* **`brokers` 必填。** 没有 localhost 兜底——空 broker 列表在启动期被拒。`version` 不设则退到最基线
  协议：SASL 机制、header、幂等 producer 都要求最低协议版本。
* **Producer 参数是 Sarama 原生。** `producer.required-acks`、`producer.idempotent`、
  `producer.compression` 直接映射到 `sarama.Config` 字段；starter 不在 Sarama 语义之上再叠抽象。
* **你派生的，由你关闭。** 实例销毁时调用 `sarama.Client.Close`（释放 broker 连接）；架在其上的
  producer 或 consumer group 必须先关——那是你的生命周期。
