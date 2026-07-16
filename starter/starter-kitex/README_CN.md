# starter-kitex

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-kitex` 基于 [github.com/cloudwego/kitex](https://pkg.go.dev/github.com/cloudwego/kitex)
为 Go-Spring 服务提供轻量的 Kitex 服务器封装：只需注册服务，Starter 会自动完成服务器构建、
可选的 etcd 注册、生命周期以及优雅停机。

## 安装

```bash
go get go-spring.org/starter-kitex
```

## 快速开始

### 1. 引入 `starter-kitex` 包

参见 [example.go](example/example.go) 文件。

```go
import StarterKitex "go-spring.org/starter-kitex"
```

### 2. 配置 Kitex 服务器

在项目的[配置文件](example/conf/app.properties)中添加 Kitex 配置：

```properties
spring.http.server.enabled=false
spring.kitex.server.addr=:8888
# Thrift 生成的服务需要一元兼容中间件：
spring.kitex.server.compatible-unary-middleware=true
```

### 3. 注册 Kitex 服务

参见 [example.go](example/example.go) 文件。将生成的 `xxxservice.RegisterService`
包装成一个 `StarterKitex.ServiceRegister` Bean —— Starter 会构建原始的
`server.Server` 并调用它来绑定你的 handler，因此 Starter 本身不依赖任何生成代码：

```go
gs.Provide(func() StarterKitex.ServiceRegister {
    return func(svr server.Server) error {
        return echoservice.RegisterService(svr, &EchoServiceImpl{})
    }
})
```

## 核心功能

[示例](example/example.go) 在同一个进程内同时运行服务器和客户端，并在 `runTest` 中对
一元 Echo 调用做了端到端断言：

1. **一元 Echo 调用**：客户端调用 `EchoService.Echo` 并拿到原样返回的消息，验证标准的
   请求/响应链路。
2. **与具体服务解耦的服务器**：`SimpleKitexServer` 只依赖一个 `ServiceRegister` 函数，
   生成的桩代码在应用层完成装配，Starter 因此可复用于任意 Kitex 服务（thrift、protobuf
   或 generic）。
3. **可选的 etcd 服务发现**：不配置 `registry.etcd`（如示例所示）即以免注册中心模式运行，
   客户端通过 host:port 直连；配置后则会将服务以其服务名注册进 etcd 供发现。

## 说明

- Starter 监听地址由 `${spring.kitex.server.addr}` 决定，默认 `:8888`。
- Kitex 服务器默认开启，可通过 `spring.kitex.server.enabled=false` 关闭。
- 只需要注册一个 `ServiceRegister` Bean 即可激活整个服务器。
- Thrift 服务需设置 `spring.kitex.server.compatible-unary-middleware=true`
  （Kitex 的 thrift 代码生成会在其 `NewServer` 中加入该中间件）；protobuf/gRPC 服务
  则应保持关闭，因为它们本就不包含该中间件。
