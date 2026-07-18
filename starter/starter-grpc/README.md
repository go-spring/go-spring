# starter-grpc

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-grpc` provides a lightweight [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc)
server wrapper for Go-Spring applications: register your service, and the
starter takes care of listener setup, lifecycle, and graceful shutdown.

## Installation

```bash
go get go-spring.org/starter-grpc
```

## Quick Start

### 1. Import the `starter-grpc` package

Refer to the [example.go](example/example.go) file.

```go
import StarterGrpc "go-spring.org/starter-grpc"
```

### 2. Configure the gRPC server

Add gRPC configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.grpc.server.addr=:9494

# Message-size caps and concurrency limits (0 keeps the gRPC default).
spring.grpc.server.maxRecvMsgSize=4194304
spring.grpc.server.maxSendMsgSize=4194304
spring.grpc.server.maxConcurrentStreams=100
spring.grpc.server.connectionTimeout=0

# Server-side keepalive enforcement (0 leaves gRPC defaults intact).
spring.grpc.server.keepalive.time=2h
spring.grpc.server.keepalive.timeout=20s
spring.grpc.server.keepalive.maxConnectionIdle=0
spring.grpc.server.keepalive.maxConnectionAge=0

# Standard grpc_health_v1 health service (on by default).
spring.grpc.server.health.enabled=true

# Transport TLS: enable and point at a PEM cert/key pair.
spring.grpc.server.tls.enabled=false
spring.grpc.server.tls.certFile=
spring.grpc.server.tls.keyFile=
```

### 3. Register your service

Refer to the [example.go](example/example.go) file.

```go
gs.Provide(&Controller{})
gs.Provide(func(c *Controller) StarterGrpc.ServiceRegister {
    return func(svr *grpc.Server) {
        proto.RegisterEchoServiceServer(svr, c)
    }
})
```

## Core Features

The [example](example/example.go) demonstrates three core gRPC building blocks,
each asserted end-to-end by `runTest`:

1. **Unary Echo call** — the client invokes `EchoService.Echo` and receives
   the same message back, verifying the standard unary request/response path.
2. **Server-side unary interceptor (middleware)** — `LoggingInterceptor` is a
   real `grpc.UnaryServerInterceptor` that logs the invoked method and reads
   the incoming `x-app` metadata key. It is wired into the service via an
   `interceptedEchoServer` wrapper (because the starter currently constructs
   `grpc.NewServer()` without exposing `grpc.ServerOption`s, we compose the
   interceptor chain at the handler layer — the effect is identical to
   `grpc.ChainUnaryInterceptor`). The client sends `x-app=go-spring` via
   `metadata.NewOutgoingContext` and the call still succeeds, proving the
   interceptor ran without breaking the RPC.
3. **Response header via `grpc.SetHeader`** — the handler attaches
   `x-handler=echo` to the response headers. The client passes
   `grpc.Header(&md)` as a call option and asserts the header round-tripped.

## Notes

- The starter listens on `${spring.grpc.server.addr}` (default `:9494`).
- The gRPC server is enabled by default; disable it with
  `spring.grpc.server.enabled=false`.
- Only a `ServiceRegister` bean is required to activate the server.
