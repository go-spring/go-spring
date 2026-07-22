# starter-hertz

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-hertz` adapts the [CloudWeGo Hertz](https://github.com/cloudwego/hertz)
HTTP framework to the Go-Spring server lifecycle. The starter owns the
`*server.Hertz` and its listener (address from configuration); the application
only provides a `RouterRegister` bean to mount routes and middleware. Hertz
starts after the container is ready and shuts down gracefully with the rest of
the application.

## Installation

```bash
go get go-spring.org/starter-hertz
```

## Quick Start

### 1. Import the `starter-hertz` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-hertz"
```

### 2. Provide a `RouterRegister` Bean

The starter creates the `*server.Hertz` on the configured address and hands it
to your register; you mount middleware and routes there. Refer to the
[example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) StarterHertz.RouterRegister {
    return func(h *server.Hertz) {
        h.Use(func(ctx context.Context, r *app.RequestContext) {
            r.Response.Header.Set("X-App", "go-spring")
            r.Next(ctx)
        })
        h.GET("/echo/:name", c.Echo)
        h.GET("/greet", c.Greet)
    }
})
```

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.
> Hertz owns its own listener, but the address is still taken from
> `spring.hertz.server.addr` and passed to the engine via `WithHostPorts`.

### 3. Configure the Address and Disable the Built-in HTTP Server

Set the Hertz listen address and disable the default Go-Spring HTTP server
(Hertz drives its own listener) in [app.properties](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.hertz.server.addr=127.0.0.1:8003

# Timeouts (naming mirrors SimpleHttpServerConfig; applied via Hertz options).
spring.hertz.server.readTimeout=5s
spring.hertz.server.writeTimeout=5s
spring.hertz.server.idleTimeout=60s

# Request-body size cap in bytes (0 = Hertz default).
spring.hertz.server.maxBodySize=1048576

# Optional starter-served liveness endpoint.
spring.hertz.server.health.enabled=true
spring.hertz.server.health.path=/healthz

# HTTPS: enable and point at a PEM cert/key pair.
spring.hertz.server.tls.enabled=false
spring.hertz.server.tls.cert-file=
spring.hertz.server.tls.key-file=

# Built-in middlewares. Recovery, RequestID and AccessLog are on by default;
# CORS, Gzip and SecureHeaders are off until opted in (see Built-in Middlewares).
spring.hertz.server.middleware.recovery.enabled=true
spring.hertz.server.middleware.requestId.enabled=true
spring.hertz.server.middleware.accessLog.enabled=true
spring.hertz.server.middleware.accessLog.skipPaths=
spring.hertz.server.middleware.cors.enabled=false
spring.hertz.server.middleware.cors.allowedOrigins=
spring.hertz.server.middleware.gzip.enabled=false
spring.hertz.server.middleware.gzip.level=5
spring.hertz.server.middleware.secureHeaders.enabled=false
```

### 4. Run the Application

```go
func main() {
    gs.Run()
}
```

## Core Features

The [example.go](example/example.go) file demonstrates three core Hertz
features and asserts each one via real HTTP calls in `runTest`:

* **Middleware** — the starter installs Recovery, RequestID and AccessLog by default; a `h.Use(...)` middleware sets the `X-App: go-spring`
  response header on every request; the test asserts the header round-trips.
* **Path parameter + JSON** — `GET /echo/:name` reads `c.Param("name")` and
  responds with `{"message":"Hello, <name>"}`; the test calls
  `/echo/hertz` and asserts `message == "Hello, hertz"`.
* **Query parameter + JSON** — `GET /greet` reads `c.Query("name")` and
  responds with `{"message":"Hi, <name>"}`; the test calls
  `/greet?name=world` and asserts `message == "Hi, world"`.

## Built-in Middlewares

The starter installs a fixed, ordered set of cross-cutting middlewares on the
`*server.Hertz` **before** the application's `RouterRegister` runs, so they wrap
every route. Each is independently toggleable via `spring.hertz.server.middleware.*`.
Recovery comes from the hertz core; RequestID/CORS/Gzip from hertz-contrib;
AccessLog and SecureHeaders are self-implemented.

| Middleware | Default | Source | Notes |
|---|---|---|---|
| `recovery` | on | core `recovery.Recovery()` | Catches request-goroutine panics; turning it off risks a process crash. The starter uses `server.New` (not `server.Default`) so this is configurable. |
| `requestId` | on | hertz-contrib/requestid | Generates/propagates `X-Request-Id`; also stored on the request context (see `RequestIDFromContext`). |
| `accessLog` | on | self (project `log` pkg) | One structured record per request; Warn on 4xx, Error on 5xx; the health path is auto-skipped. |
| `cors` | off | hertz-contrib/cors | No safe universal default - supply `allowedOrigins` (or `allowAllOrigins` for dev). Misconfig fails at startup. |
| `gzip` | off | hertz-contrib/gzip | `level` (1-9, -1=default). |
| `secureHeaders` | off | self | `X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`; HSTS only with TLS. (hertz-contrib/secure defaults to a 10-year HSTS + SSL redirect, so it is intentionally not used.) |
| body limit | on when `maxBodySize>0` | engine option `WithMaxRequestBodySize` | Not a middleware; an over-limit 413 is logged like any response. |

Order (outermost first): `Recovery -> RequestID -> AccessLog -> SecureHeaders -> CORS -> Gzip`.
Recovery is outermost so it catches panics from every later layer; RequestID runs before AccessLog
so each access record carries the id; AccessLog wraps the policy middlewares so short-circuit
responses (204, 403) are still logged.

> **No request-timeout middleware by design.** Go cannot preempt a running handler without the
> goroutine-buffer hack (which breaks streaming/SSE), so the hard bound stays the Hertz
> read/write timeouts from `${spring.hertz.server}`. Metrics and tracing are not built in either -
> use `starter-actuator` and `starter-otel` for those.

To stamp the request id onto business logs, wire the log package's context hook once:

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
    if rid := StarterHertz.RequestIDFromContext(ctx); rid != "" {
        return []log.Field{log.String("request_id", rid)}
    }
    return nil
}
```

## Advanced Features

* **Framework-owned engine** — the starter builds the `*server.Hertz` from
  configuration and hands it to your `RouterRegister`; any Hertz server option (TLS, tracer, custom transport, ...) applies uniformly.
* **Managed lifecycle** — the adapter waits for the Go-Spring readiness signal
  before calling `h.Run()`, and calls `h.Shutdown(ctx)` on shutdown.
* **Opt-in registration** — set `spring.hertz.server.enabled=false` to opt out
  of the automatic server registration (enabled by default when a
  `RouterRegister` bean is present).
