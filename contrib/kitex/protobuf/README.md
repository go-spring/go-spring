# kitex — protobuf (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` example generated
from a **protobuf** IDL and refactored to boot and be configured the Go-Spring
way: `gs.Run()` drives the lifecycle, the handler is an IoC bean, and the bind
address comes from `conf/app.properties` instead of hard-coded `main()` wiring.

Because the service is defined in protobuf, one provider serves **both**
protobuf transports on the same port at once:

- **KitexProtobuf** — Kitex's own protobuf payload over TTHeader (the default).
- **gRPC** — protobuf over HTTP/2.

The server sniffs each incoming connection and dispatches accordingly, so
there is nothing protocol-specific to configure on the provider; the consumer
picks the wire protocol per call via `client.WithTransportProtocol`. This is
the companion to the Thrift variant in [`../thrift`](../thrift).

It wires in an **etcd registry** (via
`github.com/kitex-contrib/registry-etcd`) for real **service registration &
discovery**: on startup the provider registers the `echo` service into etcd;
the consumer never learns the provider's host:port and instead resolves a live
address from the same etcd.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ service: echo                            │ resolve provider addr
  │ → <host>:8888                            │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀── KitexProtobuf ──────│  consumer  │
│ gs.Run()   │◀────── gRPC ───────────│ one-shot   │
│ :8888      │──────────────────────▶│ assert+exit│
└────────────┘       echo message     └────────────┘
```

## Layout

```
contrib/kitex/protobuf/
├── idl/echo.proto           # protobuf IDL
├── kitex_gen/echo/...       # Kitex-generated code (DO NOT EDIT)
├── kitex_info.yaml          # metadata for re-generation
├── scripts/gen-code.sh      # regenerates kitex_gen/ from the IDL
├── provider/handler.go      # EchoServiceImpl, exported as an echo.EchoService bean
├── provider/server.go       # KitexServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # discovers via etcd, calls once over each transport, asserts, exits
├── conf/app.properties      # provider configuration
├── docker-compose.yml       # local etcd
└── scripts/smoke-test.sh    # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

```bash
# tool (once)
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest

# scaffold from the IDL (or just run ./scripts/gen-code.sh)
kitex -module go-spring.org/kitex/protobuf -service echo idl/echo.proto
```

The scaffold produces `kitex_gen/`, a bare `handler.go`, and a `main.go` that
calls `svr.Run()` directly. `kitex_gen/` is shared by both the provider and the
consumer, and it already supports both KitexProtobuf and gRPC — the transport
is a runtime choice, not a codegen one. Re-running `./scripts/gen-code.sh` regenerates
`kitex_gen/` without touching the refactored provider/consumer code.

## Choosing the transport

The provider is transport-agnostic. On the consumer side:

```go
// KitexProtobuf (default): no transport option.
cli, _ := echoservice.NewClient("echo", client.WithResolver(r))

// gRPC: add WithTransportProtocol.
cli, _ := echoservice.NewClient("echo",
    client.WithResolver(r),
    client.WithTransportProtocol(transport.GRPC))
```

`consumer/main.go` calls the discovered provider once over each transport and
asserts both, proving the single provider speaks both protocols.

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only Kitex.
spring.http.server.enabled=false

# Kitex bind address; read via the ${spring.kitex.server} prefix, default :8888.
# This one port serves both KitexProtobuf and gRPC.
spring.kitex.server.addr=:8888

# Service name registered into etcd; consumer resolves by the same name.
spring.kitex.server.service.name=echo

# etcd registry address; matches docker-compose.yml.
spring.kitex.server.registry.etcd=127.0.0.1:2379
```

## Run

Bring up the registry first:

```bash
docker compose up -d      # or docker-compose up -d
```

Terminal A — start the provider (long-lived, registers into etcd):

```bash
go run ./provider
```

Terminal B — start the consumer (discovers via etcd and calls over both transports):

```bash
go run ./consumer
```

Expected consumer output:

```
[KitexProtobuf] response from discovered provider: Hello, Kitex!
[gRPC] response from discovered provider: Hello, Kitex!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash scripts/smoke-test.sh
```
