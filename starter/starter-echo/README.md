# starter-echo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-echo` wires the [labstack/echo](https://github.com/labstack/echo) web framework into Go-Spring.
The starter owns the `*echo.Echo` and its HTTP server (created from configuration); the application only
provides a `RouterRegister` bean to mount routes and middleware, and everything is served through the
Go-Spring server lifecycle.

## Installation

```bash
go get go-spring.org/starter-echo
```

## Quick Start

### 1. Import the `starter-echo` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-echo"
```

### 2. Configure the Echo Server

Add configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
# Let echo own the HTTP port; disable Go-Spring's built-in server.
spring.http.server.enabled=false
# starter-echo listens on :8002 by default in this example.
spring.echo.server.addr=:8002

# Timeouts (inherited from SimpleHttpServerConfig).
spring.echo.server.readTimeout=5s
spring.echo.server.headerTimeout=1s
spring.echo.server.writeTimeout=5s
spring.echo.server.idleTimeout=60s

# Request-body size cap in bytes (0 = unlimited).
spring.echo.server.maxBodySize=1048576

# Optional starter-served liveness endpoint.
spring.echo.server.health.enabled=true
spring.echo.server.health.path=/healthz

# HTTPS: enable and point at a PEM cert/key pair.
spring.echo.server.tls.enabled=false
spring.echo.server.tls.cert-file=
spring.echo.server.tls.key-file=

# Built-in middlewares. Recovery, RequestID and AccessLog are on by default;
# CORS, Gzip and SecureHeaders are off until opted in (see Built-in Middlewares).
spring.echo.server.middleware.recovery.enabled=true
spring.echo.server.middleware.requestId.enabled=true
spring.echo.server.middleware.accessLog.enabled=true
spring.echo.server.middleware.accessLog.skipPaths=
spring.echo.server.middleware.cors.enabled=false
spring.echo.server.middleware.cors.allowedOrigins=
spring.echo.server.middleware.gzip.enabled=false
spring.echo.server.middleware.gzip.level=5
spring.echo.server.middleware.secureHeaders.enabled=false
```

The starter registers its server bean when `spring.echo.server.enabled` is `true` (default) and a
`RouterRegister` bean is provided by the application.

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.

### 3. Provide a `RouterRegister` Bean

The starter creates and configures the `*echo.Echo` (banner hidden, plus the built-in middlewares
below) and hands it to your register. Mount routes and middleware there. Refer to the
[example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) StarterEcho.RouterRegister {
    return func(e *echo.Echo) {
        e.GET("/echo/:name", c.Echo)
    }
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — the starter installs Recovery, RequestID and AccessLog by default (plus opt-in
  CORS/Gzip/SecureHeaders); the register adds a custom middleware that
  that sets an `X-App: go-spring` response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Route group** — `e.Group("/api")` mounts `GET /api/greet?name=...` returning
  `{"message":"Hi, <name>"}` from the query string.

## Built-in Middlewares

The starter installs a fixed, ordered set of cross-cutting middlewares on the `*echo.Echo` **before**
the application's `RouterRegister` runs, so they wrap every route. Each is independently toggleable via
`spring.echo.server.middleware.*`. Echo ships all of these in its official `middleware` package, so only
AccessLog is self-implemented (to route records through the project `log` package with request-id
correlation).

| Middleware | Default | Source | Notes |
|---|---|---|---|
| `recovery` | on | `middleware.Recover()` | Catches request-goroutine panics; turning it off risks a process crash. |
| `requestId` | on | `middleware.RequestID()` | Generates/propagates `X-Request-Id`; also stored on the request context (see `RequestIDFromContext`). |
| `accessLog` | on | self (project `log` pkg) | One structured record per request; Warn on 4xx, Error on 5xx; the health path is auto-skipped. |
| `cors` | off | `middleware.CORS()` | No safe universal default - supply `allowedOrigins` (or `allowAllOrigins` for dev). |
| `gzip` | off | `middleware.Gzip()` | `level` (1-9, -1=default). |
| `secureHeaders` | off | `middleware.Secure()` | `X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`; HSTS only with TLS. |
| body limit | on when `maxBodySize>0` | `middleware.BodyLimit()` | In-chain; an over-limit 413 is logged like any response. |

Order (outermost first): `Recovery -> RequestID -> AccessLog -> SecureHeaders -> CORS -> Gzip -> BodyLimit`.
Recovery is outermost so it catches panics from every later layer; RequestID runs before AccessLog so each
access record carries the id; AccessLog wraps the policy middlewares so short-circuit responses (413, 204,
403) are still logged.

> **No request-timeout middleware by design.** Go cannot preempt a running handler without the
> goroutine-buffer hack (which breaks streaming/SSE), so the hard bound stays the `http.Server`
> read/write timeouts from `SimpleHttpServerConfig`. Metrics and tracing are not built in either - use
> `starter-actuator` and `starter-otel` for those.

To stamp the request id onto business logs, wire the log package's context hook once:

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
    if rid := StarterEcho.RequestIDFromContext(ctx); rid != "" {
        return []log.Field{log.String("request_id", rid)}
    }
    return nil
}
```

## Advanced Features

* **Custom server configuration**: tune `spring.echo.server.*` (address, TLS, timeouts, ...) via the
  standard `SimpleHttpServerConfig` binding.
* **Full echo ecosystem**: any echo middleware, group, renderer, or binder can be composed on the
  `*echo.Echo` passed to the `RouterRegister`.
