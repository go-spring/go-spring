# starter-hertz

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-hertz` adapts the [CloudWeGo Hertz](https://github.com/cloudwego/hertz)
HTTP framework to the Go-Spring server lifecycle, so Hertz starts after the
container is ready and shuts down gracefully with the rest of the application.

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

### 2. Provide a `*server.Hertz`

You own the Hertz instance (host/port, middlewares, routes); the starter only
drives its `Run` / `Shutdown` from the Go-Spring readiness signal. Refer to the
[example.go](example/example.go) file.

```go
gs.Provide(func(c *Controller) *server.Hertz {
    h := server.Default(server.WithHostPorts("127.0.0.1:8003"))
    h.Use(func(ctx context.Context, r *app.RequestContext) {
        r.Response.Header.Set("X-App", "go-spring")
        r.Next(ctx)
    })
    h.GET("/echo/:name", c.Echo)
    h.GET("/greet", c.Greet)
    return h
})
```

> **Port convention** — the three HTTP starters use distinct ports so they can run side by side:
> `starter-gin` → `:8001`, `starter-echo` → `:8002`, `starter-hertz` → `:8003`.
> Unlike gin/echo, Hertz owns its own listener, so the port is set on the engine via
> `WithHostPorts` (there is no `spring.hertz.server.addr` config to inject it).

### 3. Disable the Built-in HTTP Server

Because Hertz manages its own listener, disable the default Go-Spring HTTP
server in [app.properties](example/conf/app.properties):

```properties
spring.http.server.enabled=false
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

* **Bring-your-own Hertz** — the starter never allocates `*server.Hertz` for
  you, so any Hertz option (TLS, tracer, custom transport, ...) works
  unchanged.
* **Managed lifecycle** — the adapter waits for the Go-Spring readiness signal
  before calling `h.Run()`, and calls `h.Shutdown(ctx)` on shutdown.
* **Opt-in registration** — set `spring.hertz.server.enabled=false` to opt out
  of the automatic server registration (enabled by default when a
  `*server.Hertz` bean is present).
