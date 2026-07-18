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
队列组（每条消息只投递给一个成员）、以及 JetStream（向 stream 发布后再拉回消息）。

连接层事件（异步错误、断连、重连、关闭）会被桥接进 go-spring 日志。

## 高级功能

* **JetStream**：在某个实例上设置 `spring.nats.instances.<name>.jetstream.enabled=true`，
  即可在该实例的 `Conn.JetStream` 上暴露 JetStream 上下文，它派生自同一条连接。
* **多连接**：`spring.nats.instances` 下的每一项都会成为一个独立配置的 `*Conn` bean，
  按名称注入即可访问不同的集群或 JetStream 域。
