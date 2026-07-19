# starter-grpc

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-grpc` 基于 [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc)
为 Go-Spring 服务提供轻量的 gRPC 服务器封装：只需注册服务，Starter 会自动完成监听、
生命周期和优雅停机。

## 安装

```bash
go get go-spring.org/starter-grpc
```

## 快速开始

### 1. 引入 `starter-grpc` 包

参见 [example.go](example/example.go) 文件。

```go
import StarterGrpc "go-spring.org/starter-grpc"
```

### 2. 配置 gRPC 服务器

在项目的[配置文件](example/conf/app.properties)中添加 gRPC 配置：

```properties
spring.http.server.enabled=false
spring.grpc.server.addr=:9494

# 消息大小上限与并发限制（0 表示使用 gRPC 默认值）。
spring.grpc.server.maxRecvMsgSize=4194304
spring.grpc.server.maxSendMsgSize=4194304
spring.grpc.server.maxConcurrentStreams=100
spring.grpc.server.connectionTimeout=0

# 服务端 keepalive 策略（0 表示保持 gRPC 默认值）。
spring.grpc.server.keepalive.time=2h
spring.grpc.server.keepalive.timeout=20s
spring.grpc.server.keepalive.maxConnectionIdle=0
spring.grpc.server.keepalive.maxConnectionAge=0

# 标准 grpc_health_v1 健康检查服务（默认开启）。
spring.grpc.server.health.enabled=true

# 传输层 TLS：启用并指定 PEM 证书/私钥路径。
spring.grpc.server.tls.enabled=false
spring.grpc.server.tls.cert-file=
spring.grpc.server.tls.key-file=
```

### 3. 注册 gRPC 服务

参见 [example.go](example/example.go) 文件。

```go
gs.Provide(&Controller{})
gs.Provide(func(c *Controller) StarterGrpc.ServiceRegister {
    return func(svr *grpc.Server) {
        proto.RegisterEchoServiceServer(svr, c)
    }
})
```

## 核心功能

[示例](example/example.go) 展示了 3 个 gRPC 关键能力，均在 `runTest` 中做了端到端断言：

1. **一元 Echo 调用**：客户端调用 `EchoService.Echo` 并拿到原样返回的消息，验证标准的
   请求/响应链路。
2. **服务端一元拦截器（中间件）**：`LoggingInterceptor` 是一个真实的
   `grpc.UnaryServerInterceptor`，会打印被调用的方法名并读取入向 metadata 中的
   `x-app`。因为当前 Starter 内部通过 `grpc.NewServer()` 构造服务器，未暴露
   `grpc.ServerOption`，示例通过 `interceptedEchoServer` 包装器在 handler 层完成
   拦截链的组装 —— 效果等同于 `grpc.ChainUnaryInterceptor`。客户端使用
   `metadata.NewOutgoingContext` 发送 `x-app=go-spring`，调用仍然成功，证明拦截器
   已运行且未影响业务返回。
3. **通过 `grpc.SetHeader` 回写响应头**：handler 在响应头中写入
   `x-handler=echo`；客户端通过 `grpc.Header(&md)` 调用选项捕获并断言。

## 说明

- Starter 监听地址由 `${spring.grpc.server.addr}` 决定，默认 `:9494`。
- gRPC 服务器默认开启，可通过 `spring.grpc.server.enabled=false` 关闭。
- 只需要注册一个 `ServiceRegister` Bean 即可激活整个服务器。
