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

* **Middleware** — a `h.Use(...)` middleware sets the `X-App: go-spring`
  response header on every request; the test asserts the header round-trips.
* **Path parameter + JSON** — `GET /echo/:name` reads `c.Param("name")` and
  responds with `{"message":"Hello, <name>"}`; the test calls
  `/echo/hertz` and asserts `message == "Hello, hertz"`.
* **Query parameter + JSON** — `GET /greet` reads `c.Query("name")` and
  responds with `{"message":"Hi, <name>"}`; the test calls
  `/greet?name=world` and asserts `message == "Hi, world"`.

## Advanced Features

* **Framework-owned engine** — the starter builds the `*server.Hertz` from
  configuration and hands it to your `RouterRegister`; any Hertz option added by
  `server.Default` (TLS, tracer, custom transport, ...) applies uniformly.
* **Managed lifecycle** — the adapter waits for the Go-Spring readiness signal
  before calling `h.Run()`, and calls `h.Shutdown(ctx)` on shutdown.
* **Opt-in registration** — set `spring.hertz.server.enabled=false` to opt out
  of the automatic server registration (enabled by default when a
  `RouterRegister` bean is present).
