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
spring.echo.server.tls.certFile=
spring.echo.server.tls.keyFile=
```

The starter registers its server bean when `spring.echo.server.enabled` is `true` (default) and a
`RouterRegister` bean is provided by the application.

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.

### 3. Provide a `RouterRegister` Bean

The starter creates and configures the `*echo.Echo` (banner hidden, `middleware.Recover()`) and hands
it to your register. Mount routes and middleware there. Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) StarterEcho.RouterRegister {
    return func(e *echo.Echo) {
        e.GET("/echo/:name", c.Echo)
    }
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — the starter installs `middleware.Recover()`; the register adds a custom middleware
  that sets an `X-App: go-spring` response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Route group** — `e.Group("/api")` mounts `GET /api/greet?name=...` returning
  `{"message":"Hi, <name>"}` from the query string.

## Advanced Features

* **Custom server configuration**: tune `spring.echo.server.*` (address, TLS, timeouts, ...) via the
  standard `SimpleHttpServerConfig` binding.
* **Full echo ecosystem**: any echo middleware, group, renderer, or binder can be composed on the
  `*echo.Echo` passed to the `RouterRegister`.
