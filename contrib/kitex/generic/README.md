# kitex — generic call (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` example where the
**consumer has no generated stubs at all**. Instead it parses the Thrift IDL
at runtime and invokes the service via Kitex's **JSON generic** client:

```go
p, _   := generic.NewThriftFileProvider("idl/echo.thrift") // parse IDL at runtime
g, _   := generic.JSONThriftGeneric(p)                     // JSON <-> Thrift codec
cli, _ := genericclient.NewClient("echo-generic", g,
    client.WithResolver(etcd.NewEtcdResolver(...)))
resp, _ := cli.GenericCall(ctx, "Echo", `{"message":"hi"}`) // resp is a JSON string
```

The provider is deliberately identical to the [`../thrift`](../thrift) sibling
— a normal code-generated Kitex Thrift server. The wire format is the same
TTHeader/Thrift bytes either way, so the generic client dials the ordinary
server without any special "generic" setup on the server side.

## Why this is a separate subproject

| Subproject                       | Client stubs?                    | Wire protocol(s)                                                          |
| -------------------------------- | -------------------------------- | ------------------------------------------------------------------------- |
| [`../thrift`](../thrift)         | typed stubs                      | TTHeader / Thrift                                                         |
| [`../protobuf`](../protobuf)     | typed stubs                      | KitexProtobuf **and** gRPC on one port; selected per client at call time  |
| **`./` (this one)**              | **none — IDL parsed at runtime** | TTHeader / Thrift (same wire as `../thrift`)                              |

The two siblings both call Kitex via typed, code-generated handles. This one
demonstrates the opposite: dynamic invocation by method name with a JSON
payload, driven purely by the IDL file at runtime. That is the real capability
being showcased — not a different transport, but a fundamentally different
**invocation mode**.

Real-world use cases: API gateways proxying REST/JSON to internal Thrift
services, admin/ops tools that must call arbitrary services without a rebuild,
integration test harnesses, and cross-language bridges.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────────────┐
   register     │         etcd         │   discover
  ┌────────────▶│         :2379        │◀───────────┐
  │             └──────────────────────┘            │
  │ service: echo-generic                           │ resolve provider addr
  │ → <host>:8890                                   │
┌─┴──────────┐                              ┌───────┴─────────────────┐
│  provider  │◀──── TTHeader/Thrift ────────│  consumer               │
│ gs.Run()   │      (JSON <-> Thrift        │  generic.JSONThrift     │
│ :8890      │       encoded on client)     │  GenericCall("Echo",    │
│  (typed)   │                              │   `{"message":"hi"}`)   │
└────────────┘                              └─────────────────────────┘
```

## Layout

```
contrib/kitex/generic/
├── idl/echo.thrift          # Thrift IDL — parsed at runtime by the CONSUMER
├── kitex_gen/echo/...       # Kitex-generated code (used by the PROVIDER only)
├── kitex_info.yaml          # metadata for re-generation
├── scripts/gen-code.sh      # regenerates kitex_gen/ from the IDL
├── provider/handler.go      # EchoServiceImpl (identical to ../thrift)
├── provider/server.go       # KitexServer adapter (gs.Server) + Config
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # NO kitex_gen import; JSON generic invocation
├── conf/app.properties      # provider config (port :8890, service `echo-generic`)
├── docker-compose.yml       # local etcd (unique container name)
└── scripts/smoke-test.sh    # smoke test: bring up etcd+provider, run consumer, tear down
```

The IDL file is loaded by the consumer via a **relative** path
(`idl/echo.thrift`), so `go run ./consumer` and the scripts/smoke-test.sh binary invocation
both run from the module root where that path resolves.

## Configuration

```properties
# The built-in HTTP server is disabled so the provider exposes only Kitex.
spring.http.server.enabled=false

# Bind port 8890 (not 8888 used by thrift/protobuf) so all three subprojects
# can run side by side.
spring.kitex.server.addr=:8890

# Service name registered into etcd; consumer resolves by the same name.
# `echo-generic` keeps it distinct from the siblings' `echo` in a shared registry.
spring.kitex.server.service.name=echo-generic

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

Terminal B — start the consumer (parses the IDL, discovers via etcd, invokes
generically):

```bash
go run ./consumer
```

Expected consumer output:

```
Raw JSON response from discovered provider: {"message":"Hello, Kitex!"}
Generic call round-trip OK: Hello, Kitex!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash scripts/smoke-test.sh
```
