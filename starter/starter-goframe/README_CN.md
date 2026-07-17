# starter-goframe

[English](README.md) | [中文](README_CN.md)

`starter-goframe` 将 [gogf/gf](https://github.com/gogf/gf)(goframe)的服务器接入 Go-Spring,使 goframe
服务器像其他被托管的服务器一样,由 Go-Spring 的服务器生命周期(启动、优雅关闭)统一驱动。

goframe 提供多种传输形态,因此本模块提供 **四个相互独立的子包**,共用一个内部日志桥接:

| 子包 | goframe 服务器 | 配置前缀 | 注册 bean |
| --- | --- | --- | --- |
| `go-spring.org/starter-goframe/http` | `*ghttp.Server`(HTTP/API) | `spring.goframe.http.server` | `ServiceRegister func(*ghttp.RouterGroup)` |
| `go-spring.org/starter-goframe/grpc` | `grpcx.GrpcServer`(gRPC) | `spring.goframe.grpc.server` | `ServiceRegister func(grpc.ServiceRegistrar)` |
| `go-spring.org/starter-goframe/tcp` | `*gtcp.Server`(裸 TCP) | `spring.goframe.tcp.server` | `ServiceRegister func(*gtcp.Server)` |
| `go-spring.org/starter-goframe/ws` | `*ghttp.Server`(WebSocket 升级) | `spring.goframe.ws.server` | `ServiceRegister func(*ghttp.Server)` |

按需导入对应子包即可。当子包的 `*.enabled` 属性为 `true`(默认)**且**应用提供了 `ServiceRegister`
bean 时,starter 才会注册其服务器 bean。

## 安装

```bash
go get go-spring.org/starter-goframe
```

## 快速开始 — HTTP

见 [http/example/example.go](http/example/example.go)。

```go
import _ "go-spring.org/starter-goframe/http"
```

```properties
# 让 goframe 独占端口;关闭 Go-Spring 内置 HTTP 服务器。
spring.http.server.enabled=false
spring.goframe.http.server.name=goframe-http
spring.goframe.http.server.address=:8000
```

```go
gs.Provide(func() goframehttp.ServiceRegister {
    return func(group *ghttp.RouterGroup) {
        group.ALL("/hello", func(r *ghttp.Request) { r.Response.Writeln("Hello World!") })
    }
})
```

## 快速开始 — gRPC / TCP / WebSocket

用法完全一致:导入子包、配置前缀、提供 `ServiceRegister` bean。见
[grpc/example](grpc/example/example.go)、[tcp/example](tcp/example/example.go)、
[ws/example](ws/example/example.go)。

## 服务发现(etcd)

`ghttp.Server` 与 `grpcx.GrpcServer` 原生集成 goframe 的 `gsvc`;`gtcp.Server` 没有,其 starter
手工执行 Register/Deregister。注册**默认关闭**——设置 `registry.etcd` 属性即可将服务发布到 etcd:

```properties
spring.goframe.http.server.registry.etcd=127.0.0.1:2379
```

留空则是普通服务器,由客户端直连。

## 可观测性

* **追踪** — 让路给 [`starter-otel`](../starter-otel)。goframe 的 `ghttp`/`grpcx` 会基于全局
  OpenTelemetry `TracerProvider` 自动埋点;导入 `starter-otel` 即安装该 provider,span 自动导出,
  无需按服务器单独配置。
* **指标** — `http` 子包可在同一服务器上暴露 goframe 原生的 OTel Prometheus(pull)端点。默认关闭,
  通过 `spring.goframe.http.server.metrics.enabled=true` 开启。它与 `starter-otel` 的指标是两条独立
  管线,无法统一。
* **日志** — goframe 框架日志(`glog`)被桥接进 Go-Spring 的 `log` 模块,应用只需配置一条日志管线。
