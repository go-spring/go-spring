# starter-gin

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-gin` wires the [gin-gonic/gin](https://github.com/gin-gonic/gin) web framework into Go-Spring,
so an application-provided `*gin.Engine` bean is served through the Go-Spring server lifecycle.

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
```

The starter registers its server bean when `spring.gin.server.enabled` is `true` (default) and a
`*gin.Engine` bean is provided by the application.

### 3. Provide a `*gin.Engine` Bean

Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) *gin.Engine {
    gin.SetMode(gin.ReleaseMode)
    e := gin.New()
    e.Use(gin.Recovery())
    e.GET("/echo/:name", c.Echo)
    return e
})
```

## Core Features

The [example](example/example.go) demonstrates three features exercised end-to-end via real HTTP:

* **Middleware** — `gin.Recovery()` plus a custom middleware that sets an `X-App: go-spring`
  response header on every request.
* **Path parameter + JSON** — `GET /echo/:name` returns `{"message":"Hello, <name>"}` using
  `ctx.Param` and `ctx.JSON`.
* **Query parameter** — `GET /greet?name=...` reads `ctx.Query("name")` and returns
  `{"message":"Hi, <name>"}` as JSON.

## Advanced Features

* **Custom server configuration**: tune `spring.gin.server.*` (address, TLS, timeouts, ...) via the
  standard `SimpleHttpServerConfig` binding.
* **Full gin ecosystem**: any gin middleware, route group, renderer, or binder can be composed on the
  `*gin.Engine` bean before it is handed to the starter.
