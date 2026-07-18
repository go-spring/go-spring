# starter-trpc

[English](README.md) | [中文](README_CN.md)

`starter-trpc` provides a lightweight [trpc-group/trpc-go](https://github.com/trpc-group/trpc-go)
server wrapper for Go-Spring applications: register your service, and the
starter builds the tRPC server, drives its lifecycle, and shuts it down
gracefully alongside every other Go-Spring server.

The deliberate design point: unify tRPC-Go's configuration into Go-Spring's
property system. The starter translates properties under the
`spring.trpc.server` prefix into a tRPC `*Config`, then calls
`trpc.NewServerWithConfig(cfg)` — **no `trpc_go.yaml` file is used**.

## Installation

```bash
go get go-spring.org/starter-trpc
```

## Quick Start

### 1. Import the `starter-trpc` package

Refer to the [example.go](example/example.go) file.

```go
import StarterTrpc "go-spring.org/starter-trpc"
```

### 2. Configure the tRPC server

Add tRPC configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.trpc.server.addr=127.0.0.1:8000
spring.trpc.server.service.name=trpc.helloworld.greet.GreetService
```

### 3. Register your service

Refer to the [example.go](example/example.go) file. Wrap the generated
`xxx.RegisterXxxServiceService` in a `StarterTrpc.ServiceRegister` bean — the
starter builds the `*server.Server` and calls this to bind your handler, so it
never depends on generated code:

```go
gs.Provide(func() StarterTrpc.ServiceRegister {
    return func(s *server.Server) {
        greet.RegisterGreetServiceService(s, &GreetServiceImpl{})
    }
})
```

## Core Features

The [example](example/example.go) runs the server and an in-process client in
one binary and asserts a unary Greet round-trip end-to-end via `runTest`:

1. **Unary Greet call** — the client dials `ip://127.0.0.1:8000` directly and
   invokes `GreetService.Greet`, verifying the standard request/response path.
2. **Service-agnostic server** — `SimpleTrpcServer` depends only on a
   `ServiceRegister` function, so the generated stubs are wired at the
   application layer and the starter stays reusable across any tRPC service.
3. **No `trpc_go.yaml`** — the tRPC `*Config` is built programmatically from
   Go-Spring properties, so all configuration lives in `conf/app.properties`
   like every other Go-Spring service.

## Logging (built in)

Importing this starter puts tRPC under go-spring's management: its internal log
(server wiring, transport errors, and handler `trpclog.Infof` calls) is bridged
into go-spring's `log` module automatically (installed in an `init()`, no
configuration needed) instead of tRPC's default zap console sink.

tRPC's base `Logger` interface carries no `context.Context`, so forwarded lines
cannot be tagged with the incoming `trace_id`/`span_id` on this path — the same
limitation the non-ctx paths of the other framework bridges have.

The bridge only redirects *who writes the log*; you must still configure a
go-spring log sink, otherwise the forwarded lines land on go-spring's default
console rather than your app's output. Configure a root logger as usual, e.g.:

```properties
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=../logs
logging.logger.root.file=app.log
logging.logger.root.layout.type=JSONLayout
```

## Signal handling — a caveat

tRPC-Go's `server.Serve()` installs its own OS signal handlers
(`SIGINT`, `SIGTERM`, `SIGSEGV`, `SIGUSR2`). It co-exists with Go-Spring's
lifecycle: when Go-Spring shuts down it calls `SimpleTrpcServer.Stop()`, which
invokes `server.Close(nil)` to unblock `Serve()` cleanly. Be aware of this
signal co-ownership when embedding tRPC-Go inside another lifecycle owner.

## Notes

- The starter listens on `${spring.trpc.server.addr}` (default `127.0.0.1:8000`).
- The tRPC server is enabled by default; disable it with
  `spring.trpc.server.enabled=false`.
- Only a `ServiceRegister` bean is required to activate the server.
- This starter uses **direct-connect** dialing; there is no service registry,
  so no docker is required to run the example.
