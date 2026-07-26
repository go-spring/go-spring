# starter-goframe

[English](README.md) | [中文](README_CN.md)

`starter-goframe` wires the [gogf/gf](https://github.com/gogf/gf) (goframe) servers into Go-Spring, so a
goframe server is driven through the Go-Spring server lifecycle (startup, graceful shutdown) alongside
every other managed server.

goframe exposes several transports, so this module ships **four independent sub-packages** that share a
single internal log bridge:

| Sub-package | goframe server | Config prefix | Register bean |
| --- | --- | --- | --- |
| `go-spring.org/starter-goframe/http` | `*ghttp.Server` (HTTP/API) | `spring.goframe.http.server` | `ServiceRegister func(*ghttp.RouterGroup)` |
| `go-spring.org/starter-goframe/grpc` | `grpcx.GrpcServer` (gRPC) | `spring.goframe.grpc.server` | `ServiceRegister func(grpc.ServiceRegistrar)` |
| `go-spring.org/starter-goframe/tcp` | `*gtcp.Server` (raw TCP) | `spring.goframe.tcp.server` | `ServiceRegister func(*gtcp.Server)` |
| `go-spring.org/starter-goframe/ws` | `*ghttp.Server` (WebSocket upgrade) | `spring.goframe.ws.server` | `ServiceRegister func(*ghttp.Server)` |

Import only the sub-package you need. Each starter registers its server bean when its
`*.enabled` property is `true` (default) **and** the application provides a `ServiceRegister` bean.

## Installation

```bash
go get go-spring.org/starter-goframe
```

## Quick Start — HTTP

See [http/example/example.go](http/example/example.go).

```go
import _ "go-spring.org/starter-goframe/http"
```

```properties
# Let goframe own the port; disable Go-Spring's built-in HTTP server.
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

## Quick Start — gRPC / TCP / WebSocket

The shape is identical: import the sub-package, configure the prefix, and provide a `ServiceRegister`
bean. See [grpc/example](grpc/example/example.go), [tcp/example](tcp/example/example.go) and
[ws/example](ws/example/example.go).

## Service Discovery (etcd)

`ghttp.Server` and `grpcx.GrpcServer` integrate with goframe's `gsvc` out of the box; `gtcp.Server`
does not, so its starter performs Register/Deregister by hand. Registration is **off by default** —
set the `registry.etcd` property to publish the server for discovery:

```properties
spring.goframe.http.server.registry.etcd=127.0.0.1:2379
```

Leave it empty for a plain server that clients dial directly.

## Observability

* **Tracing** — deferred to [`starter-otel`](../starter-otel). goframe's `ghttp`/`grpcx` auto-instrument
  requests off the global OpenTelemetry `TracerProvider`; importing `starter-otel` installs that provider
  and spans are exported automatically, with no per-server configuration.
* **Metrics** — the `http` sub-package can expose goframe's native OTel Prometheus (pull) endpoint on the
  same server. Off by default; enable it with `spring.goframe.http.server.metrics.enabled=true`. It is a
  separate pipeline from `starter-otel`'s metrics and cannot be unified with it.
* **Logging** — goframe's framework logs (`glog`) are bridged into Go-Spring's `log` module, so an
  application configures a single logging pipeline.
