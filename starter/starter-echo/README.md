# starter-echo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-echo` wires the [labstack/echo](https://github.com/labstack/echo) web framework into Go-Spring,
so an application-provided `*echo.Echo` bean is served through the Go-Spring server lifecycle.

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
```

The starter registers its server bean when `spring.echo.server.enabled` is `true` (default) and an
`*echo.Echo` bean is provided by the application.

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.

### 3. Provide an `*echo.Echo` Bean

Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) *echo.Echo {
    e := echo.New()
    e.HideBanner = true
    e.Use(middleware.Recover())
    e.GET("/echo/:name", c.Echo)
    return e
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — `middleware.Recover()` plus a custom middleware that sets an `X-App: go-spring`
  response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Route group** — `e.Group("/api")` mounts `GET /api/greet?name=...` returning
  `{"message":"Hi, <name>"}` from the query string.

## Advanced Features

* **Custom server configuration**: tune `spring.echo.server.*` (address, TLS, timeouts, ...) via the
  standard `SimpleHttpServerConfig` binding.
* **Full echo ecosystem**: any echo middleware, group, renderer, or binder can be composed on the
  `*echo.Echo` bean before it is handed to the starter.
