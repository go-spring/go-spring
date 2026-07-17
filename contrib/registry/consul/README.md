# Consul registry (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

Service registration & discovery through **Consul**, using a
[Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` generated from a
**protobuf** IDL. On startup the provider registers the `echo` service into
Consul; the consumer resolves a live provider address from that same Consul
instead of dialing a hard-coded `host:port`.

This is the odd one out among the five sibling examples under [`..`](..): the
other four use dubbo-go, but dubbo-go has **no Consul registry extension**, so
Consul is demonstrated with Kitex via
[`github.com/kitex-contrib/registry-consul`](https://github.com/kitex-contrib/registry-consul).
See the top-level [README](../README.md) for the registry overview.

Because the service is defined in protobuf, one provider serves **both**
protobuf transports on the same port: **KitexProtobuf** (Kitex's protobuf over
TTHeader, the default) and **gRPC** (protobuf over HTTP/2). The server sniffs
each connection and dispatches accordingly; the consumer picks the wire protocol
per call via `client.WithTransportProtocol`.

## Layout

```
consul/
├── idl/echo.proto           # protobuf IDL
├── idl/echo/...             # Kitex-generated code (DO NOT EDIT)
├── idl/kitex_info.yaml      # metadata for re-generation
├── idl/gen-code.sh          # regenerates idl/echo/ from the IDL
├── provider/handler.go      # EchoServiceImpl (the business logic)
├── provider/server.go       # KitexServer (gs.Server) — wires the Consul registry
├── provider/main.go         # gs.Run(); long-lived, registers into Consul
├── provider/conf/app.properties  # provider config (bind addr + service + registry)
├── consumer/main.go         # discovers via Consul, calls over each transport, asserts, exits
├── consumer/conf/app.properties  # consumer config (registry + service name)
├── docker-compose.yml       # local Consul (agent -dev)
└── scripts/smoke-test.sh    # smoke test: up consul+provider, run consumer, tear down
```

## Why the registry is wired in code (no starter)

`starter-kitex` only knows how to build an **etcd** registry, so this example
does the Consul wiring itself. `provider/server.go` is a `gs.Server` adapter
that builds a Kitex server with `server.WithRegistry(consul.NewConsulRegister(...))`
and registers `EchoServiceImpl`. The rest of the app is still Go-Spring: the
server is an IoC bean and `gs.Run()` drives its lifecycle.

The consumer resolves through `consul.NewConsulResolver(addr)` fed into
`client.WithResolver(...)`, then calls once per transport and asserts both.

## The registry configuration

```properties
# Provider: bind to a CONCRETE host:port (not a wildcard). Consul registers
# exactly this address and health-checks it over TCP, so 0.0.0.0 would never
# pass the check.
spring.kitex.server.addr=127.0.0.1:8888
spring.kitex.server.service.name=echo
spring.kitex.server.registry.consul=127.0.0.1:8500

# Consumer: same Consul agent, same service name.
spring.kitex.consumer.registry.consul=127.0.0.1:8500
spring.kitex.consumer.service.name=echo
```

## Run

```bash
docker compose up -d          # or docker-compose up -d
go run ./provider &           # long-lived, registers into Consul
go run ./consumer             # discovers via Consul, calls over both transports
```

Expected consumer output:

```
[KitexProtobuf] response from discovered provider: Hello, Kitex!
[gRPC] response from discovered provider: Hello, Kitex!
```

Or the one-shot smoke test:

```bash
bash scripts/smoke-test.sh
```
