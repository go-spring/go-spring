# starter-go-zero

[English](README.md) | [中文](README_CN.md)

`starter-go-zero` 将 [zeromicro/go-zero](https://github.com/zeromicro/go-zero) 的服务端接入 Go-Spring，
让 go-zero 的 server 随 Go-Spring 的服务生命周期（启动、优雅关闭）统一被托管。

go-zero 有**两种** server 类型，因此本模块拆成**两个相互独立的子包**，共用一个内部日志桥接：

| 子包 | go-zero server | 配置前缀 | 注册 bean |
| --- | --- | --- | --- |
| `go-spring.org/starter-go-zero/rest` | `rest.Server`（HTTP/API；WebSocket 作为其上的升级路由） | `spring.go-zero.rest.server` | `HandlerRegister func(*rest.Server)` |
| `go-spring.org/starter-go-zero/zrpc` | `zrpc.RpcServer`（gRPC + etcd 服务发现） | `spring.go-zero.zrpc.server` | `ServiceRegister func(*grpc.Server)` |

按需只引用你要的子包——引用 `rest` 不会把 zrpc server 编进你的二进制，反之亦然。

## 安装

```bash
go get go-spring.org/starter-go-zero
```

## 快速开始 —— REST（HTTP/API）

参考 [rest/example/example.go](rest/example/example.go)。

### 1. 引入子包

```go
import _ "go-spring.org/starter-go-zero/rest"
```

### 2. 配置 server

```properties
# 让 go-zero 独占端口，关闭 Go-Spring 内置 HTTP server。
spring.http.server.enabled=false
spring.go-zero.rest.server.name=greet
spring.go-zero.rest.server.host=0.0.0.0
spring.go-zero.rest.server.port=8888
```

当 `spring.go-zero.rest.server.enabled` 为 `true`（默认）**且**应用提供了 `HandlerRegister` bean 时，
starter 才会注册 server bean。

### 3. 提供 `HandlerRegister` bean

```go
gs.Provide(func() gozerorest.HandlerRegister {
    return func(server *rest.Server) {
        server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/greet", Handler: ...})
    }
})
```

## 快速开始 —— zRPC（gRPC）

参考 [zrpc/example/example.go](zrpc/example/example.go)。

### 1. 引入子包

```go
import _ "go-spring.org/starter-go-zero/zrpc"
```

### 2. 配置 server

```properties
spring.http.server.enabled=false
spring.go-zero.zrpc.server.name=greet-rpc
spring.go-zero.zrpc.server.listen-on=0.0.0.0:8081
# 可选：注册进 etcd 做服务发现，留空则为直连模式。
# spring.go-zero.zrpc.server.etcd.addr=127.0.0.1:2379
# spring.go-zero.zrpc.server.etcd.key=greet.rpc
```

当 `spring.go-zero.zrpc.server.enabled` 为 `true`（默认）**且**应用提供了 `ServiceRegister` bean 时，
starter 才会注册 server bean。

### 3. 提供 `ServiceRegister` bean

```go
gs.Provide(func() gozerozrpc.ServiceRegister {
    return func(s *grpc.Server) {
        pb.RegisterYourServer(s, &YourServer{})
    }
})
```

## 可观测性

* **Tracing** —— 默认交给 [`starter-otel`](../starter-otel)（`tracing.disabled=true`）。go-zero 的
  REST/gRPC 中间件通过全局 OpenTelemetry `TracerProvider` 产生 span；只要引入 `starter-otel`，它会装好
  全局 provider，span 自动导出，无需在 server 侧再配置。若想改用 go-zero 原生 OTLP 导出，设
  `tracing.disabled=false`（并配 `tracing.endpoint`）。
* **Metrics** —— go-zero 不产出 OpenTelemetry metrics，其指标是纯 Prometheus，由 go-zero 自带的
  DevServer `/metrics` 端点提供。**默认关闭**，需要抓取 go-zero 原生 registry 时用 `metrics.enabled=true`
  打开。无法与 `starter-otel` 的 metrics 管线统一。
* **Logging** —— go-zero 的框架日志（`logx`）桥接进 Go-Spring 的 `log` 模块，应用只需配置一套日志管线。
  桥接后 `logx` 仅 `log.level` 仍然生效。
