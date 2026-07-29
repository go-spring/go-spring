# starter-gin

[English](README.md) | [中文](README_CN.md)

`starter-gin` wires the [gin-gonic/gin](https://github.com/gin-gonic/gin) web framework into
Go-Spring. The starter owns the `*gin.Engine` and its HTTP server (created from configuration); the
application only provides a `RouterRegister` bean to mount routes and middleware, and everything
is served through the Go-Spring server lifecycle.

## Installation

```bash
go get go-spring.org/starter-gin
```

## Quick Start

### 1. Import the `starter-gin` Package

```go
import _ "go-spring.org/starter-gin"
```

### 2. Configure the Gin Server

Add configuration in your project's [configuration file](example/conf/app.properties):

```properties
# Let gin own the HTTP port; disable Go-Spring's built-in server.
spring.http.server.enabled=false
# starter-gin listens on :8001 by default in this example.
spring.gin.server.addr=:8001

# Timeouts (inherited from SimpleHttpServerConfig).
spring.gin.server.readTimeout=5s
spring.gin.server.writeTimeout=5s
spring.gin.server.idleTimeout=60s

# Optional starter-served liveness endpoint.
spring.gin.server.health.enabled=true
spring.gin.server.health.path=/healthz

# HTTPS: enable and point at a PEM cert/key pair.
spring.gin.server.tls.enabled=false
spring.gin.server.tls.cert-file=
spring.gin.server.tls.key-file=

# Built-in middlewares. The `enabled` master switch (default true) turns the
# whole built-in set on/off; when false the register owns the entire chain,
# including Recovery. With it on, Recovery/Tracing/Metrics/AccessLog are bundled
# into the always-on Observe middleware; RequestID is on by default; CORS, Gzip
# and SecureHeaders are off until opted in (see Built-in Middlewares).
spring.gin.server.middleware.enabled=true
spring.gin.server.middleware.requestId.enabled=true
spring.gin.server.middleware.requestId.header=X-Request-Id
spring.gin.server.middleware.accessLog.skipPaths=
spring.gin.server.middleware.accessLog.payload.enabled=true
spring.gin.server.middleware.cors.enabled=false
spring.gin.server.middleware.cors.allowedOrigins=
spring.gin.server.middleware.gzip.enabled=false
spring.gin.server.middleware.gzip.level=5
spring.gin.server.middleware.secureHeaders.enabled=false
spring.gin.server.middleware.secureHeaders.frameOptions=DENY
spring.gin.server.middleware.secureHeaders.referrerPolicy=no-referrer
```

The starter registers its server bean when `spring.gin.server.enabled` is `true` (default) and a
`RouterRegister` bean is provided by the application.

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.

### 3. Provide a `RouterRegister` Bean

The starter creates and configures the `*gin.Engine` (release mode, plus the built-in middlewares
below) and hands it to your register. Mount routes and middleware there.

```go
gs.Provide(func(c *Controller) StarterGin.RouterRegister {
    return func(e *gin.Engine) {
        e.GET("/echo/:name", c.Echo)
    }
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — the starter installs Recovery, Tracing, Metrics and AccessLog by default (bundled
  into one Observe middleware, plus RequestID and opt-in CORS/Gzip/SecureHeaders); the register adds a
  custom middleware that sets an `X-App: go-spring` response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Query parameter** — `GET /greet?name=...` reads `ctx.Query("name")` and returns
  `{"message":"Hi, <name>"}` as JSON.

## Built-in Middlewares

The starter installs a fixed, ordered set of cross-cutting middlewares on the `*gin.Engine` **before**
the application's `RouterRegister` runs, so they wrap every route. The `middleware.enabled` master
switch (default true) turns the whole set on; when an application sets it to `false`, the starter
installs nothing and the register owns the entire chain - including Recovery. With the set on,
Recovery, Tracing, Metrics and AccessLog are mandatory and always on (bundled into one `Observe`
middleware); RequestID is on by default; the rest are off until opted in via `spring.gin.server.middleware.*`.

| Middleware | Default | Source | Notes |
|---|---|---|---|
| `observe` | on (always) | self | Bundles Recovery + Tracing + Metrics + AccessLog into one per-request lifecycle: a single deferred finalize ends the span, records metrics, and emits the access log even on a handler panic. Tracing/Metrics ride the OTel globals (no-op without `starter-otel`); the access log is Warn on 4xx, Error on 5xx, auto-skips the health path, carries `request_id`/`trace_id`/`span_id`, and (via `accessLog.payload.enabled`, default on) captures request body+query+headers and response body, each capped at 512 KiB with binary content masked. SSE responses (`text/event-stream`) are logged per flushed event in real time, not buffered to stream close. |
| `requestId` | on | `gin-contrib/requestid` | Generates/propagates `X-Request-Id`; also stored on the request context (see `RequestIDFromContext`). |
| `cors` | off | `gin-contrib/cors` | No safe universal default - supply `allowedOrigins` (or `allowAllOrigins` for dev). Misconfig fails at startup. |
| `gzip` | off | `gin-contrib/gzip` | `level` (1-9, -1=default), `minLength` (0=compress all). |
| `secureHeaders` | off | self | `X-Content-Type-Options` (always nosniff) + configurable `frameOptions` (default DENY) / `referrerPolicy` (default no-referrer); HSTS only with TLS. |

Order (outermost first): `Observe -> RequestID -> SecureHeaders -> CORS -> Gzip`.
Observe is outermost so its single defer recovers panics from every later layer and finalizes the span,
metrics, and access log together; RequestID runs inside Observe so each access record carries the id and
stays within the recovered span; the policy middlewares sit inside the chain so short-circuit responses
(204, 403) are still observed.

> **No request-timeout middleware by design.** Go cannot preempt a running handler without the
> goroutine-buffer hack (which breaks streaming/SSE), so the hard bound stays the `http.Server`
> read/write timeouts from `SimpleHttpServerConfig`.

To stamp the request id onto business logs, wire the log package's context hook once:

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
    if rid := StarterGin.RequestIDFromContext(ctx); rid != "" {
        return []log.Field{log.String("request_id", rid)}
    }
    return nil
}
```

## Advanced Features

* **Custom server configuration**: tune `spring.gin.server.*` (address, TLS, timeouts, ...) via the
  standard `SimpleHttpServerConfig` binding.
* **Full gin ecosystem**: any gin middleware, route group, renderer, or binder can be composed on the
  `*gin.Engine` passed to the `RouterRegister`.
