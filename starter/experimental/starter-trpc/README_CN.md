# starter-trpc

[English](README.md) | [中文](README_CN.md)

`starter-trpc` 为 Go-Spring 应用提供一个轻量的 [trpc-group/trpc-go](https://github.com/trpc-group/trpc-go)
服务封装：注册你的 service，starter 负责构建 tRPC server、驱动其生命周期，
并与其它 Go-Spring server 一起优雅关停。

刻意的设计取舍：把 tRPC-Go 的配置统一进 Go-Spring 的属性体系。starter 把
`spring.trpc.server` 前缀下的属性翻译为一个 tRPC 的 `*Config`，再调用
`trpc.NewServerWithConfig(cfg)` —— **完全不使用 `trpc_go.yaml` 文件**。

## 安装

```bash
go get go-spring.org/starter-trpc
```

## 快速开始

### 1. 导入 `starter-trpc` 包

参考 [example.go](example/example.go) 文件。

```go
import StarterTrpc "go-spring.org/starter-trpc"
```

### 2. 配置 tRPC server

在项目的[配置文件](example/conf/app.properties)中添加 tRPC 配置：

```properties
spring.http.server.enabled=false
spring.trpc.server.addr=127.0.0.1:8000
spring.trpc.server.service.name=trpc.helloworld.greet.GreetService
```

### 3. 注册你的 service

参考 [example.go](example/example.go) 文件。把生成的
`xxx.RegisterXxxServiceService` 包进一个 `StarterTrpc.ServiceRegister` bean —
starter 会构建 `*server.Server` 并调用它来挂载你的 handler，因此 starter
本身不依赖任何生成代码：

```go
gs.Provide(func() StarterTrpc.ServiceRegister {
    return func(s *server.Server) {
        greet.RegisterGreetServiceService(s, &GreetServiceImpl{})
    }
})
```

## 核心特性

[example](example/example.go) 在同一个二进制里同时跑 server 和一个进程内
client，并通过 `runTest` 端到端断言一次 unary Greet 往返：

1. **Unary Greet 调用** —— client 直连 `ip://127.0.0.1:8000` 调用
   `GreetService.Greet`，验证标准的请求/响应路径。
2. **service 无关的 server** —— `SimpleTrpcServer` 只依赖一个
   `ServiceRegister` 函数，生成的桩代码在应用层挂载，starter 因此可复用于
   任意 tRPC service。
3. **无 `trpc_go.yaml`** —— tRPC 的 `*Config` 由 Go-Spring 属性以编程方式
   构建，所有配置都落在 `conf/app.properties`，与其它 Go-Spring 服务一致。

## 日志（内置）

导入本 starter 会把 tRPC 纳入 go-spring 的管理：它的内部日志（server 装配、
传输错误，以及 handler 的 `trpclog.Infof` 调用）会被自动桥接进 go-spring 的
`log` 模块（在 `init()` 中安装，无需配置），取代 tRPC 默认的 zap 控制台 sink。

tRPC 的基础 `Logger` 接口不带 `context.Context`，因此转发的日志行在这条路径上
无法被打上进入请求的 `trace_id`/`span_id` —— 与其它框架桥接的非 ctx 路径
一样的限制。

桥接只重定向"谁来写日志"；你仍需配置一个 go-spring 日志 sink，否则转发的日志
会落到 go-spring 默认的控制台，而非你应用自己的输出。照常配置一个 root logger，
例如：

```properties
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=../logs
logging.logger.root.file=app.log
logging.logger.root.layout.type=JSONLayout
```

## 信号处理 —— 一个注意点

tRPC-Go 的 `server.Serve()` 会自己注册 OS 信号处理器
(`SIGINT`、`SIGTERM`、`SIGSEGV`、`SIGUSR2`)。它与 Go-Spring 的生命周期共存：
Go-Spring 关停时会调用 `SimpleTrpcServer.Stop()`，后者进而调用
`server.Close(nil)` 以干净地解阻塞 `Serve()`。把 tRPC-Go 嵌入到另一个
生命周期宿主内时，需注意这一"信号共管"的事实。

## 说明

- starter 监听 `${spring.trpc.server.addr}`（默认 `127.0.0.1:8000`）。
- tRPC server 默认启用；可通过 `spring.trpc.server.enabled=false` 关闭。
- 只需一个 `ServiceRegister` bean 即可激活 server。
- 本 starter 使用**直连**方式拨号，不接注册中心，因此运行示例无需 docker。
