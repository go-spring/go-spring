# starter-kitex

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-kitex` provides a lightweight [github.com/cloudwego/kitex](https://pkg.go.dev/github.com/cloudwego/kitex)
server wrapper for Go-Spring applications: register your service, and the
starter takes care of server construction, optional etcd registration,
lifecycle, and graceful shutdown.

## Installation

```bash
go get go-spring.org/starter-kitex
```

## Quick Start

### 1. Import the `starter-kitex` package

Refer to the [example.go](example/example.go) file.

```go
import StarterKitex "go-spring.org/starter-kitex"
```

### 2. Configure the Kitex server

Add Kitex configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.kitex.server.addr=:8888
# Thrift-generated services need the unary-compatible middleware:
spring.kitex.server.compatible-unary-middleware=true
```

### 3. Register your service

Refer to the [example.go](example/example.go) file. Wrap the generated
`xxxservice.RegisterService` in a `StarterKitex.ServiceRegister` bean — the
starter builds the raw `server.Server` and calls this to bind your handler,
so it never depends on generated code:

```go
gs.Provide(func() StarterKitex.ServiceRegister {
    return func(svr server.Server) error {
        return echoservice.RegisterService(svr, &EchoServiceImpl{})
    }
})
```

## Core Features

The [example](example/example.go) runs the server and an in-process client in
one binary and asserts a unary Echo round-trip end-to-end via `runTest`:

1. **Unary Echo call** — the client invokes `EchoService.Echo` and receives the
   same message back, verifying the standard request/response path.
2. **Service-agnostic server** — `SimpleKitexServer` depends only on a
   `ServiceRegister` function, so the generated stubs are wired at the
   application layer and the starter stays reusable across any Kitex service
   (thrift, protobuf, or generic).
3. **Optional etcd discovery** — leaving `registry.etcd` empty (as the example
   does) runs a registry-free server dialed directly by host:port; setting it
   publishes the service into etcd for discovery under its service name.

## Notes

- The starter listens on `${spring.kitex.server.addr}` (default `:8888`).
- The Kitex server is enabled by default; disable it with
  `spring.kitex.server.enabled=false`.
- Only a `ServiceRegister` bean is required to activate the server.
- Set `spring.kitex.server.compatible-unary-middleware=true` for thrift
  services (Kitex's thrift codegen adds it in its generated `NewServer`); leave
  it off for protobuf/gRPC services, which omit it.
