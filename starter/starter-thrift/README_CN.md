# starter-thrift

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-thrift` 基于 [Apache Thrift](https://thrift.apache.org/) 为
Go-Spring 服务提供轻量的 Thrift 服务器封装：只需注册一个
`thrift.TProcessor` Bean，Starter 会自动完成 `TSimpleServer` 的监听、
生命周期与优雅停机。

## 安装

```bash
go get go-spring.org/starter-thrift
```

## 快速开始

### 1. 引入 `starter-thrift` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-thrift"
```

### 2. 配置 Thrift 服务器

在项目的[配置文件](example/conf/app.properties)中添加 Thrift 配置：

```properties
spring.http.server.enabled=false
spring.thrift.server.addr=:9292

# 服务端 socket 的单连接客户端超时（0 表示不超时）。
spring.thrift.server.clientTimeout=30s

# 传输协议：binary（默认）/ compact / json。必须与客户端一致。
spring.thrift.server.protocol=binary

# 传输层包装：none（裸 socket，默认）/ buffered / framed。
# 必须与客户端一致。很多跨语言客户端要求使用 framed。
spring.thrift.server.transport=none

# buffered/framed 传输的缓冲区 / 最大帧大小（字节）。
spring.thrift.server.bufferSize=4096

# TLS 服务端传输：启用并指定 PEM 证书/私钥路径。
spring.thrift.server.tls.enabled=false
spring.thrift.server.tls.certFile=
spring.thrift.server.tls.keyFile=
```

### 3. 注册 Processor

参见 [example.go](example/example.go) 文件。

```go
gs.Provide(&Controller{})
gs.Provide(func(c *Controller) thrift.TProcessor {
    return proto.NewEchoServiceProcessor(c)
})
```

## 核心功能

[示例](example/example.go) 展示了 3 个 Thrift 关键能力，均在 `runTest` 中做了端到端断言：

1. **Echo RPC**：客户端调用 `EchoService.Echo`，传入 `"Hello, Thrift!"`，
   并断言响应体原样返回，验证在所配置的 `compact` 协议 + `framed`
   传输下的标准请求/响应链路。
2. **TProcessor 中间件 / 装饰器**：`loggingProcessor` 是一个真实的
   `thrift.TProcessor` 实现，包装了生成代码的 `EchoServiceProcessor`。
   每次 RPC 都会读取方法名并打印日志，然后转发到内层 processor 对应
   方法的 `TProcessorFunction`。这是 Thrift 侧对齐 gRPC
   `UnaryServerInterceptor` 的写法。`gs.Provide` 暴露的正是这个包装
   后的 processor，`TSimpleServer` 直接使用，Starter 无需任何改动。
   `runTest` 结束前会检查一个原子调用计数器，证明中间件在每次 RPC
   都执行了一次。
3. **第二次不同 payload 的往返**：客户端再次调用 `Echo`，传入
   `"Middleware works!"` 并断言返回值一致。配合第 2 点，可以证明装饰器
   在多次独立调用中都能正确转发，并且中间件按 RPC 粒度触发（计数器 = 2）。

### 关于传输层

Starter 通过 `NewTSimpleServer4` 构建服务端，同时暴露协议工厂与传输工厂：

- **协议**（`spring.thrift.server.protocol`）：`binary`（默认）、
  `compact`、`json`。客户端的协议工厂必须与之匹配。
- **传输**（`spring.thrift.server.transport`）：`none`（裸 socket，
  历史默认值）、`buffered`、`framed`。`framed` 会为每条消息添加长度
  前缀，很多跨语言客户端要求使用它。客户端的传输必须与之匹配。

客户端/服务端在协议或传输上不匹配会导致读死锁或协议错乱。
[示例](example/example.go) 将服务端配置为 `compact` + `framed`，并让
客户端相应对齐：用 `TFramedTransport` 包装 socket + `TCompactProtocol`。

### 关于服务器模型

Go 版 Thrift 库只提供 `TSimpleServer`。虽然名字叫 "Simple"，它的
`AcceptLoop` 对每个连接都会 `go` 一个新 goroutine，本身就是每连接一
goroutine 的并发模型。`THsHaServer` / `TThreadPoolServer` 是 Java/C++
的概念，Go 库并不提供 —— 不存在可切换的"多线程 server"。

## 说明

- Starter 监听地址由 `${spring.thrift.server.addr}` 决定，默认 `:9292`。
- Thrift 服务器默认开启，可通过 `spring.thrift.server.enabled=false` 关闭。
- 只需要注册一个 `thrift.TProcessor` Bean 即可激活整个服务器。
