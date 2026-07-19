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
```

The starter registers its server bean when `spring.gin.server.enabled` is `true` (default) and a
`RouterRegister` bean is provided by the application.

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.

### 3. Provide a `RouterRegister` Bean

The starter creates and configures the `*gin.Engine` (release mode, `gin.Recovery()`) and hands it to
your register. Mount routes and middleware there. Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) StarterGin.RouterRegister {
    return func(e *gin.Engine) {
        e.GET("/echo/:name", c.Echo)
    }
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — the starter installs `gin.Recovery()`; the register adds a custom middleware that
  sets an `X-App: go-spring` response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Query parameter** — `GET /greet?name=...` reads `ctx.Query("name")` and returns
  `{"message":"Hi, <name>"}` as JSON.

## Advanced Features

* **Custom server configuration**: tune `spring.gin.server.*` (address, TLS, timeouts, ...) via the
  standard `SimpleHttpServerConfig` binding.
* **Full gin ecosystem**: any gin middleware, route group, renderer, or binder can be composed on the
  `*gin.Engine` passed to the `RouterRegister`.
