# starter-nats

[English](README.md) | [中文](README_CN.md)

`starter-nats` 提供了基于 github.com/nats-io/nats.go 的 NATS 客户端封装，
方便在 Go-Spring 服务中快速集成核心消息与 JetStream。该库为纯 Go 实现（无 cgo），
交叉编译保持干净。

## 安装

```bash
go get go-spring.org/starter-nats
```

## 快速开始

### 1. 引入 `starter-nats` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-nats"
```

### 2. 配置 NATS 连接

在项目的[配置文件](example/conf/app.properties)中，在 `spring.nats.instances.<name>`
下定义一个或多个具名连接，比如：

```properties
spring.nats.instances.main.url=nats://127.0.0.1:4222
spring.nats.instances.main.jetstream.enabled=true
spring.nats.instances.work.url=nats://127.0.0.1:4222
```

### 3. 注入 NATS 连接

参见 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`*Conn` bean；注入的 bean 内嵌 `*nats.Conn`，因此可以直接在其上调用
`Publish`/`Subscribe`/`Request`；当该实例启用 JetStream 时，`Conn.JetStream` 非空。

```go
import StarterNats "go-spring.org/starter-nats"

type Service struct {
    Conn *StarterNats.Conn `autowire:"main"`
}
```

### 4. 使用连接

参见 [example.go](example/example.go) 文件。连接在启动时建立、在关闭时优雅排空（drain），
因此可以直接进行发布和订阅。

```go
_ = s.Conn.Publish("demo.subject", []byte("value"))
reply, _ := s.Conn.Request("demo.rpc", []byte("ping"), time.Second)
```

## 核心功能

[example](example/example.go) 针对真实服务自断言了四项功能：核心发布/订阅、请求-应答、
队列组（每条消息只投递给一个成员）、以及 JetStream（向 stream 发布后再拉回消息）。运行前
还会检查 `Conn.Healthy()` 报告连接处于可用状态。

连接层事件（异步错误、断连、重连、关闭）会被桥接进 go-spring 日志。

## 消息 Binder

除原生连接外,本 starter 还可暴露一个 broker 中立的 `messaging.Binder`
(来自 `go-spring.org/spring/messaging`),让业务代码收发 `*messaging.Message`
信封而不依赖 `nats.go` API —— 底层换 broker 时业务代码无需改动。

从 `*Conn` 注册一个 binder bean(用 `gs.TagArg` 选取具名实例):

```go
import (
    "go-spring.org/spring/gs"
    StarterNats "go-spring.org/starter-nats"
)

gs.Provide(StarterNats.NewBinder, gs.TagArg("main"))
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

订阅方的 `group` 映射为 NATS queue group(竞争消费)。trace context 骑在消息
`Header` 上,配合 starter-otel 即可让 producer 与 consumer 链路串联。原生 `*Conn`
bean 仍可用于 JetStream、请求-应答等 binder 未建模的 NATS 能力。

## 高级功能

* **JetStream**：在某个实例上设置 `spring.nats.instances.<name>.jetstream.enabled=true`，
  即可在该实例的 `Conn.JetStream` 上暴露 JetStream 上下文，它派生自同一条连接。
* **多连接**：`spring.nats.instances` 下的每一项都会成为一个独立配置的 `*Conn` bean，
  按名称注入即可访问不同的集群或 JetStream 域。
* **健康检查**：`Conn.Healthy()` 反映自动重连客户端的实时状态，健康/就绪探针可随时查询，
  无需只依赖连接事件日志。
* **鉴权**：除用户名/密码与 token 外，还支持 NATS 2.x 去中心化鉴权——凭据文件
  （`creds-file`）或 nkey seed 文件（`nkey-file`）。
* **TLS**：设置 `spring.nats.instances.<name>.tls.enabled=true` 即可协商 TLS，可选地指定
  CA 证书（`tls.ca-file`）并提供客户端证书（`tls.cert-file`/`tls.key-file`）以实现双向 TLS。

## 可观测性

分布式链路追踪通过原生 OTel 辅助函数提供,依赖
[starter-otel](../starter-otel) 安装的全局 `TracerProvider` 与传播器。未引入
starter-otel 时它们为 no-op,也不改动任何消息字节,因此埋点是安全的零配置可选项。

```go
import starter "go-spring.org/starter-nats"

// 生产者:使用 PublishMsg 发布,使链路上下文可随消息 header 传播。
msg := &nats.Msg{Subject: "demo.pubsub", Data: []byte("hello")}
_, span := starter.StartPublishSpan(ctx, msg)
err := conn.PublishMsg(msg)
starter.EndSpan(span, err)

// 消费者:延续消息 header 中携带的链路上下文。
ctx, span := starter.StartConsumeSpan(ctx, msg)
err := handle(ctx, msg)
starter.EndSpan(span, err)
```

为什么用调用点辅助函数,而不是包装连接:

* `nats.go` 没有官方 OTel instrumentation,其 API(`Publish`、`Subscribe`、
  `Request`、`QueueSubscribe` 以及 JetStream)面广且以回调为主,包装器需要影子实现
  全部接口,而且仍会漏掉 JetStream 上下文。
* `nats.Msg` 携带 `Header`(自 NATS 2.2 起)且能存活于 broker 往返。传播上下文需要用
  `PublishMsg`/`RespondMsg` 发布 `*nats.Msg`,而非无 header 的 `Publish(subject, data)`;
  在调用点埋点才能把生产者与消费者跨服务串联起来。

## 配置项

`spring.nats.instances.<name>` 下每条连接读取以下配置：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `url` | （必填） | NATS 服务 URL，集群可用逗号分隔多个。 |
| `name` | `` | 上报给服务端的连接名。 |
| `username` / `password` | `` | 用户名/密码鉴权。 |
| `token` | `` | token 鉴权（用户名/密码的替代方案）。 |
| `creds-file` | `` | NATS 凭据文件（JWT + nkey seed）路径，用于去中心化鉴权。 |
| `nkey-file` | `` | nkey seed 文件路径（`creds-file` 的替代方案）。 |
| `tls.enabled` | `false` | 为连接协商 TLS。 |
| `tls.ca-file` | `` | 校验服务端证书的 PEM CA 包；为空时用系统根证书。 |
| `tls.cert-file` / `tls.key-file` | `` | 双向 TLS 的客户端证书与私钥（须同时设置）。 |
| `tls.insecure-skip-verify` | `false` | 关闭服务端证书校验（仅测试用）。 |
| `max-reconnects` | `60` | 最大重连次数；`-1` 表示无限。 |
| `reconnect-wait` | `2s` | 每次重连之间的等待时长。 |
| `connect-timeout` | `5s` | 初次拨号的超时上限。 |
| `jetstream.enabled` | `false` | 在 `Conn.JetStream` 上暴露 JetStream 上下文。 |
