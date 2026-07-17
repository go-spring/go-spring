# starter-go-zero

[English](README.md) | [中文](README_CN.md)

`starter-go-zero` wires the [zeromicro/go-zero](https://github.com/zeromicro/go-zero) servers into
Go-Spring, so a go-zero server is driven through the Go-Spring server lifecycle (startup, graceful
shutdown) alongside every other managed server.

go-zero exposes **two** server types, so this module ships **two independent sub-packages** that share
a single internal log bridge:

| Sub-package | go-zero server | Config prefix | Register bean |
| --- | --- | --- | --- |
| `go-spring.org/starter-go-zero/rest` | `rest.Server` (HTTP/API; WebSocket rides on it as an upgrade route) | `spring.go-zero.rest.server` | `HandlerRegister func(*rest.Server)` |
| `go-spring.org/starter-go-zero/zrpc` | `zrpc.RpcServer` (gRPC + etcd service discovery) | `spring.go-zero.zrpc.server` | `ServiceRegister func(*grpc.Server)` |

Import only the sub-package you need — importing `rest` does not pull the zrpc server into your binary,
and vice versa.

## Installation

```bash
go get go-spring.org/starter-go-zero
```

## Quick Start — REST (HTTP/API)

See [rest/example/example.go](rest/example/example.go).

### 1. Import the sub-package

```go
import _ "go-spring.org/starter-go-zero/rest"
```

### 2. Configure the server

```properties
# Let go-zero own the port; disable Go-Spring's built-in HTTP server.
spring.http.server.enabled=false
spring.go-zero.rest.server.name=greet
spring.go-zero.rest.server.host=0.0.0.0
spring.go-zero.rest.server.port=8888
```

The starter registers its server bean when `spring.go-zero.rest.server.enabled` is `true` (default)
**and** the application provides a `HandlerRegister` bean.

### 3. Provide a `HandlerRegister` bean

```go
gs.Provide(func() gozerorest.HandlerRegister {
    return func(server *rest.Server) {
        server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/greet", Handler: ...})
    }
})
```

## Quick Start — zRPC (gRPC)

See [zrpc/example/example.go](zrpc/example/example.go).

### 1. Import the sub-package

```go
import _ "go-spring.org/starter-go-zero/zrpc"
```

### 2. Configure the server

```properties
spring.http.server.enabled=false
spring.go-zero.zrpc.server.name=greet-rpc
spring.go-zero.zrpc.server.listen-on=0.0.0.0:8081
# Optional: publish into etcd for service discovery. Leave empty for direct-connect.
# spring.go-zero.zrpc.server.etcd.addr=127.0.0.1:2379
# spring.go-zero.zrpc.server.etcd.key=greet.rpc
```

The starter registers its server bean when `spring.go-zero.zrpc.server.enabled` is `true` (default)
**and** the application provides a `ServiceRegister` bean.

### 3. Provide a `ServiceRegister` bean

```go
gs.Provide(func() gozerozrpc.ServiceRegister {
    return func(s *grpc.Server) {
        pb.RegisterYourServer(s, &YourServer{})
    }
})
```

## Observability

* **Tracing** — deferred to [`starter-otel`](../starter-otel) by default (`tracing.disabled=true`).
  go-zero's REST/gRPC middleware emit spans through the global OpenTelemetry `TracerProvider`; when
  `starter-otel` is imported it installs that provider and the spans are exported automatically, no
  per-server configuration. Set `tracing.disabled=false` (with `tracing.endpoint`) to use go-zero's
  own native OTLP export instead.
* **Metrics** — go-zero does not emit OpenTelemetry metrics; its metrics are Prometheus-only, served
  from go-zero's own DevServer `/metrics` endpoint. This is **off by default**; enable it with
  `metrics.enabled=true` when you want to scrape go-zero's native registry. It cannot be unified with
  `starter-otel`'s metrics pipeline.
* **Logging** — go-zero's framework logs (`logx`) are bridged into Go-Spring's `log` module, so an
  application configures a single logging pipeline. Only `log.level` still applies to `logx`.
