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
   并断言响应体原样返回，验证标准的 `TSocket` + `TBinaryProtocol`
   请求/响应链路。
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

Starter 内部的 `NewTSimpleServer2` 使用默认的 identity 传输工厂（不做
`TFramedTransport` 包装）+ `TBinaryProtocol`。示例客户端严格对齐：裸
`TSocket` + `TBinaryProtocol`。若你把服务端切换到 framed 传输，客户端
需要用 `thrift.NewTFramedTransportConf` 包装 socket —— 客户端/服务端
传输层不匹配会导致读死锁或协议错乱。

## 说明

- Starter 监听地址由 `${spring.thrift.server.addr}` 决定，默认 `:9292`。
- Thrift 服务器默认开启，可通过 `spring.thrift.server.enabled=false` 关闭。
- 只需要注册一个 `thrift.TProcessor` Bean 即可激活整个服务器。
