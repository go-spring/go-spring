# starter-dubbo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-dubbo` provides a lightweight [dubbo.apache.org/dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3)
server wrapper for Go-Spring applications: register your service, and the
starter takes care of building the Triple server, lifecycle, and graceful
shutdown.

## Installation

```bash
go get go-spring.org/starter-dubbo
```

## Quick Start

### 1. Import the `starter-dubbo` package

Refer to the [example.go](example/example.go) file.

```go
import StarterDubbo "go-spring.org/starter-dubbo"
```

### 2. Configure the Dubbo server

Add Dubbo configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.dubbo.server.port=20000
```

### 3. Register your service

Refer to the [example.go](example/example.go) file. A `ServiceRegister` is a
function that registers a service onto the Dubbo `server.Server`; it returns an
error because Dubbo's generated `Register*Handler` functions do.

```go
gs.Provide(func() StarterDubbo.ServiceRegister {
    return func(svr *server.Server) error {
        return greet.RegisterGreetServiceHandler(svr, &GreetProvider{})
    }
})
```

## Core Features

The [example](example/example.go) demonstrates a Dubbo Triple round-trip,
asserted end-to-end by `runTest`:

1. **Unary Greet call** — the server exports `greet.GreetService` over the
   Triple protocol on the configured port. The client dials it directly via
   `client.WithClientURL`, invokes `Greet`, and receives the request name back
   as the greeting, verifying the standard request/response path.
2. **Service-agnostic server** — `DubboServer` knows nothing about
   `GreetService`. It depends only on a `ServiceRegister` bean, so the same
   server drives any Dubbo service; the concrete registration lives in the
   application layer.

## Notes

- The starter builds a Triple server on `${spring.dubbo.server.port}` (default `20000`).
- The Dubbo server is enabled by default; disable it with
  `spring.dubbo.server.enabled=false`.
- Only a `ServiceRegister` bean is required to activate the server.
