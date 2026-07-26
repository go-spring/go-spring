# starter-kratos

[English](README.md) | [中文](README_CN.md)

`starter-kratos` wires the [go-kratos/kratos](https://github.com/go-kratos/kratos) transport servers
into Go-Spring, so a kratos server is driven through the Go-Spring server lifecycle (startup, graceful
shutdown, etcd registration) alongside every other managed server.

kratos exposes several transports that are, in practice, **independent servers** (each its own listener
and port). This module ships **three independent sub-packages**, one per transport, that share a single
internal log bridge. Each sub-package builds its own `kratos.App` and registers as its own service in
etcd — pick the transports you need and import only those:

| Sub-package | kratos server | Config prefix | Register bean |
| --- | --- | --- | --- |
| `go-spring.org/starter-kratos/http` | `transport/http.Server` | `spring.kratos.http.server` | `ServiceRegister func(*http.Server) error` |
| `go-spring.org/starter-kratos/grpc` | `transport/grpc.Server` | `spring.kratos.grpc.server` | `ServiceRegister func(*grpc.Server) error` |
| `go-spring.org/starter-kratos/ws`   | [`tx7do/kratos-transport`](https://github.com/tx7do/kratos-transport) `websocket.Server` | `spring.kratos.ws.server` | `ServiceRegister func(*websocket.Server) error` |

Import only the sub-package you need — importing `http` or `grpc` does **not** pull the WebSocket
transport (and its `github.com/tx7do/kratos-transport` dependency, pinned to `v1.3.1`) into your binary.

## Installation

```bash
go get go-spring.org/starter-kratos
```

## Quick Start — HTTP

See the example under [contrib/go-kratos/http](../../contrib/go-kratos/http).

### 1. Import the sub-package

```go
import _ "go-spring.org/starter-kratos/http"
```

### 2. Configure the server

```properties
# Let kratos own the port; disable Go-Spring's built-in HTTP server.
spring.http.server.enabled=false
spring.kratos.http.server.name=kratos-http
spring.kratos.http.server.addr=0.0.0.0:8000
# Optional: publish into etcd for service discovery. Leave empty for direct-connect.
# spring.kratos.http.server.etcd.addr=127.0.0.1:2379
```

The starter registers its server bean when `spring.kratos.http.server.enabled` is `true` (default)
**and** the application provides a `ServiceRegister` bean.

### 3. Provide a `ServiceRegister` bean

```go
gs.Provide(func() kratoshttp.ServiceRegister {
    return func(hs *http.Server) error {
        v1.RegisterGreeterHTTPServer(hs, &GreeterService{})
        return nil
    }
})
```

## Quick Start — gRPC

Identical shape to HTTP; swap the import for `go-spring.org/starter-kratos/grpc`, the config prefix for
`spring.kratos.grpc.server` (default addr `0.0.0.0:9000`), and register with `v1.RegisterGreeterServer`.

## Quick Start — WebSocket

Import `go-spring.org/starter-kratos/ws`, configure under `spring.kratos.ws.server` (default addr
`0.0.0.0:9002`, `path=/`), and bind message handlers with `websocket.RegisterServerMessageHandler`.
WebSocket carries application-defined framed messages, not proto RPCs; see the pinned-version and
binary-payload notes in `ws/starter.go`.

## Observability

* **Tracing** — deferred to [`starter-otel`](../starter-otel). The HTTP and gRPC servers install
  kratos' `tracing.Server()` middleware, which emits spans through the global OpenTelemetry
  `TracerProvider`; when `starter-otel` is imported it installs that provider and spans are exported
  automatically, with no per-server configuration. Absent `starter-otel`, the middleware is a no-op.
* **Metrics** — opt-in per server via `spring.kratos.<proto>.server.metrics.enable=true`. When enabled,
  kratos' metrics middleware records the request counter and latency histogram into the process-global
  OpenTelemetry meter, so the exporter and scrape endpoint are owned by `starter-otel` — this starter
  stands up no Prometheus endpoint of its own.
* **Logging** — kratos' framework logs are bridged into Go-Spring's `log` module (see
  `internal/logger`), so an application configures a single logging pipeline.
* **WebSocket** — the WebSocket transport has no middleware chain and is intentionally **not**
  instrumented for tracing or metrics.
