# starter-kratos

[English](README.md) | [中文](README_CN.md)

`starter-kratos` 把 [go-kratos/kratos](https://github.com/go-kratos/kratos) 的 transport server 接入
Go-Spring,使 kratos server 与其他被托管的 server 一样,由 Go-Spring 的 server 生命周期驱动(启动、优雅关停、
etcd 注册)。

kratos 暴露的多个 transport 实质上是**相互独立的 server**(各自的 listener、各自的端口)。本模块按 transport 拆分为
**三个独立子包**,共享同一个内部日志桥接。每个子包构建自己的 `kratos.App`,以自己的服务名注册进 etcd —— 用哪个就
只 import 哪个:

| 子包 | kratos server | 配置前缀 | 注册 bean |
| --- | --- | --- | --- |
| `go-spring.org/starter-kratos/http` | `transport/http.Server` | `spring.kratos.http.server` | `ServiceRegister func(*http.Server) error` |
| `go-spring.org/starter-kratos/grpc` | `transport/grpc.Server` | `spring.kratos.grpc.server` | `ServiceRegister func(*grpc.Server) error` |
| `go-spring.org/starter-kratos/ws`   | [`tx7do/kratos-transport`](https://github.com/tx7do/kratos-transport) `websocket.Server` | `spring.kratos.ws.server` | `ServiceRegister func(*websocket.Server) error` |

只 import 你需要的子包 —— 引入 `http` 或 `grpc` **不会**把 WebSocket transport(及其固定在 `v1.3.1` 的
`github.com/tx7do/kratos-transport` 依赖)链接进你的二进制。

## 安装

```bash
go get go-spring.org/starter-kratos
```

## 快速开始 —— HTTP

参见 [contrib/go-kratos/http](../../contrib/go-kratos/http) 下的示例。

### 1. Import 子包

```go
import _ "go-spring.org/starter-kratos/http"
```

### 2. 配置 server

```properties
# 让 kratos 独占端口;关掉 Go-Spring 内置的 HTTP server。
spring.http.server.enabled=false
spring.kratos.http.server.name=kratos-http
spring.kratos.http.server.addr=0.0.0.0:8000
# 可选:发布到 etcd 做服务发现。留空则为直连。
# spring.kratos.http.server.etcd.addr=127.0.0.1:2379
```

当 `spring.kratos.http.server.enabled` 为 `true`(默认)**且**应用提供了 `ServiceRegister` bean 时,
starter 才会注册它的 server bean。

### 3. 提供 `ServiceRegister` bean

```go
gs.Provide(func() kratoshttp.ServiceRegister {
    return func(hs *http.Server) error {
        v1.RegisterGreeterHTTPServer(hs, &GreeterService{})
        return nil
    }
})
```

## 快速开始 —— gRPC

形态与 HTTP 完全一致;把 import 换成 `go-spring.org/starter-kratos/grpc`,配置前缀换成
`spring.kratos.grpc.server`(默认 addr `0.0.0.0:9000`),用 `v1.RegisterGreeterServer` 注册。

## 快速开始 —— WebSocket

import `go-spring.org/starter-kratos/ws`,在 `spring.kratos.ws.server` 前缀下配置(默认 addr
`0.0.0.0:9002`,`path=/`),用 `websocket.RegisterServerMessageHandler` 绑定消息处理器。WebSocket 承载的是
应用自定义的分帧消息,而非 proto RPC;固定版本与二进制载荷的说明见 `ws/starter.go`。

## 可观测

* **Tracing** —— 让路给 [`starter-otel`](../starter-otel)。HTTP 与 gRPC server 安装 kratos 的
  `tracing.Server()` 中间件,通过全局 OpenTelemetry `TracerProvider` 发出 span;当 import 了 `starter-otel`
  时它会装上该 provider,span 自动导出,无需任何 per-server 配置。缺少 `starter-otel` 时,该中间件为 no-op。
* **Metrics** —— 每个 server 通过 `spring.kratos.<proto>.server.metrics.enable=true` 按需开启。开启后,
  kratos 的 metrics 中间件把请求计数器与延迟直方图记录进进程级全局 OpenTelemetry meter,因此 exporter 与抓取
  端点归 `starter-otel` 所有 —— 本 starter 自己不起任何 Prometheus 端点。
* **Logging** —— kratos 的框架日志被桥接进 Go-Spring 的 `log` 模块(见 `internal/logbridge`),应用只需配置
  单一的日志管线。
* **WebSocket** —— WebSocket transport 没有中间件链,有意**不**做 tracing / metrics 埋点。
