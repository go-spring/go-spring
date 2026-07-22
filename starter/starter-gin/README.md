# starter-gin

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-gin` wires the [gin-gonic/gin](https://github.com/gin-gonic/gin) web framework into Go-Spring.
The starter owns the `*gin.Engine` and its HTTP server (created from configuration); the application
only provides a `RouterRegister` bean to mount routes and middleware, and everything is served through
the Go-Spring server lifecycle.

## Installation

```bash
go get go-spring.org/starter-gin
```

## Quick Start

### 1. Import the `starter-gin` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-gin"
```

### 2. Configure the Gin Server

Add configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
# Let gin own the HTTP port; disable Go-Spring's built-in server.
spring.http.server.enabled=false
# starter-gin listens on :8001 by default in this example.
spring.gin.server.addr=:8001

# Timeouts (inherited from SimpleHttpServerConfig).
spring.gin.server.readTimeout=5s
spring.gin.server.headerTimeout=1s
spring.gin.server.writeTimeout=5s
spring.gin.server.idleTimeout=60s

# Request-body size cap in bytes (0 = unlimited).
spring.gin.server.maxBodySize=1048576

# Optional starter-served liveness endpoint.
spring.gin.server.health.enabled=true
spring.gin.server.health.path=/healthz

# HTTPS: enable and point at a PEM cert/key pair.
spring.gin.server.tls.enabled=false
spring.gin.server.tls.cert-file=
spring.gin.server.tls.key-file=

# Built-in middlewares. Recovery, RequestID and AccessLog are on by default;
# CORS, Gzip and SecureHeaders are off until opted in (see Built-in Middlewares).
spring.gin.server.middleware.recovery.enabled=true
spring.gin.server.middleware.requestId.enabled=true
spring.gin.server.middleware.requestId.header=X-Request-Id
spring.gin.server.middleware.accessLog.enabled=true
spring.gin.server.middleware.accessLog.skipPaths=
spring.gin.server.middleware.cors.enabled=false
spring.gin.server.middleware.cors.allowedOrigins=
spring.gin.server.middleware.gzip.enabled=false
spring.gin.server.middleware.gzip.level=5
spring.gin.server.middleware.secureHeaders.enabled=false
```

The starter registers its server bean when `spring.gin.server.enabled` is `true` (default) and a
`RouterRegister` bean is provided by the application.

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.

### 3. Provide a `RouterRegister` Bean

The starter creates and configures the `*gin.Engine` (release mode, plus the built-in middlewares
below) and hands it to your register. Mount routes and middleware there. Refer to the
[example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) StarterGin.RouterRegister {
    return func(e *gin.Engine) {
        e.GET("/echo/:name", c.Echo)
    }
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — the starter installs Recovery, RequestID and AccessLog by default (plus opt-in
  CORS/Gzip/SecureHeaders); the register adds a custom middleware that
  sets an `X-App: go-spring` response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Query parameter** — `GET /greet?name=...` reads `ctx.Query("name")` and returns
  `{"message":"Hi, <name>"}` as JSON.

## Built-in Middlewares

The starter installs a fixed, ordered set of cross-cutting middlewares on the `*gin.Engine` **before**
the application's `RouterRegister` runs, so they wrap every route. Each is independently toggleable via
`spring.gin.server.middleware.*`.

| Middleware | Default | Source | Notes |
|---|---|---|---|
| `recovery` | on | `gin.Recovery()` | Catches request-goroutine panics; turning it off risks a process crash. |
| `requestId` | on | `gin-contrib/requestid` | Generates/propagates `X-Request-Id`; also stored on the request context (see `RequestIDFromContext`). |
| `accessLog` | on | self (project `log` pkg) | One structured record per request; Warn on 4xx, Error on 5xx; the health path is auto-skipped. |
| `cors` | off | `gin-contrib/cors` | No safe universal default - supply `allowedOrigins` (or `allowAllOrigins` for dev). Misconfig fails at startup. |
| `gzip` | off | `gin-contrib/gzip` | `level` (1-9, -1=default), `minLength` (0=compress all). |
| `secureHeaders` | off | self | `X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`; HSTS only with TLS. |
| body limit | on when `maxBodySize>0` | self | In-chain; an over-limit 413 is logged like any response. |

Order (outermost first): `Recovery -> RequestID -> AccessLog -> SecureHeaders -> CORS -> Gzip -> BodyLimit`.
Recovery is outermost so it catches panics from every later layer; RequestID runs before AccessLog so each
access record carries the id; AccessLog wraps the policy middlewares so short-circuit responses (413, 204,
403) are still logged.

> **No request-timeout middleware by design.** Go cannot preempt a running handler without the
> goroutine-buffer hack (which breaks streaming/SSE), so the hard bound stays the `http.Server`
> read/write timeouts from `SimpleHttpServerConfig`. Metrics and tracing are not built in either - use
> `starter-actuator` and `starter-otel` (otelgin) for those.

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
