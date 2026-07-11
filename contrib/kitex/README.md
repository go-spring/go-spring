# kitex (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` example that
starts from the scaffold `kitex` generates and is then refactored to boot and
be configured the Go-Spring way: `gs.Run()` drives the lifecycle, the handler
is an IoC bean, and the bind address comes from `conf/app.properties` instead
of hard-coded `main()` wiring.

This is a runnable example, **not** a reusable starter module.

## Layout

```
contrib/kitex/
├── idl/echo.thrift        # Thrift IDL
├── kitex_gen/echo/...     # Kitex-generated code (DO NOT EDIT)
├── kitex_info.yaml        # metadata for re-generation
├── handler.go             # EchoServiceImpl, exported as an echo.EchoService bean
├── server.go              # KitexServer adapter (gs.Server) + Config
├── main.go                # gs.Run() + a self-test client
└── conf/app.properties    # configuration
```

## How it was generated

```bash
# tools (once)
go install github.com/cloudwego/thriftgo@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest

# scaffold from the IDL
kitex -module go-spring.org/kitex -service echo idl/echo.thrift
```

The scaffold produces `kitex_gen/`, a bare `handler.go`, and a `main.go` that
calls `svr.Run()` directly. Re-running the command regenerates `kitex_gen/`
without touching the refactored `handler.go` / `server.go` / `main.go`.

## The refactor: native Kitex → Go-Spring

| Concern        | Kitex scaffold                         | Go-Spring version                                                        |
| -------------- | -------------------------------------- | ------------------------------------------------------------------------ |
| Startup        | `svr.Run()` blocks in `main()`         | `KitexServer` implements `gs.Server`; `gs.Run()` drives Run/Stop         |
| Handler wiring | `new(EchoServiceImpl)` passed manually | `gs.Provide(&EchoServiceImpl{}).Export(gs.As[echo.EchoService]())`       |
| Server enable  | always on                              | conditional on an `echo.EchoService` bean via `gs.OnBean`                |
| Address        | hard-coded default                     | `${spring.kitex.server.addr}` from `conf/app.properties`                 |
| Shutdown       | process-owned                          | graceful shutdown coordinated by Go-Spring (SIGTERM → `Stop()`)          |

The adapter in `server.go` is the crux: Kitex's `server.Run()` blocks and would
own the process, so it is wrapped to start only after `sig.TriggerAndWait()`
and to expose `Stop()` for Go-Spring's shutdown sequence.

## Configuration

```properties
# Disable the built-in HTTP server; this example exposes only Kitex.
spring.http.server.enabled=false

# Kitex bind address; read via the ${spring.kitex.server} prefix, default :8888.
spring.kitex.server.addr=:8888
```

## Run

```bash
go run .
```

`main.go` launches a client 500ms after startup, calls `Echo("Hello, Kitex!")`,
asserts the echoed body, then sends SIGTERM so Go-Spring shuts down cleanly.
Expected output includes:

```
Response from server: Hello, Kitex!
```
