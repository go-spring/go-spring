# dubbo-go — REST (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` example that speaks the **REST** protocol — HTTP/1.1
transport with per-method (verb, path, param-source) routing served by
go-restful — refactored to boot and be configured the Go-Spring way:
`gs.Run()` drives the lifecycle, the provider is an IoC bean, and the bind
port comes from `conf/app.properties` instead of hard-coded `main()` wiring.

Unlike the Triple sibling in [`../triple`](../triple), REST has no
protobuf IDL and no code generator; unlike the classic-Dubbo
([`../dubbo`](../dubbo)) and JSON-RPC ([`../jsonrpc`](../jsonrpc)) siblings,
however, REST cannot be driven by method-reflection alone: dubbo-go needs a
`RestServiceConfig` map that pins every Go method to a concrete `(HTTP verb,
URL path, param source)` tuple before Serve is called. That map is
installed by `provider/handler.go` on the server side and by
`consumer/main.go` on the client side — both must agree, and both must be
in place before the process registers/dials.

It wires in an **etcd registry** for real **service registration &
discovery**: on startup the provider registers `com.example.GreetService`
(the Java-style dotted interface name) into etcd; the consumer never learns
the provider's host:port and instead resolves a live address from the same
etcd.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ com.example.GreetService                 │ resolve provider addr
  │ → rest://<host>:20003                    │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀──── REST (HTTP/1) ────│  consumer  │
│ gs.Run()   │  GET /greet?name=...   │ one-shot   │
│ :20003     │──────────────────────▶│ assert+exit│
└────────────┘   echo name (JSON)     └────────────┘
```

## Layout

```
contrib/dubbo-go/rest/
├── proto/greet.go           # the "IDL": interface name, method name, HTTP verb+path+query constants
├── gen.sh                   # no-op — REST has no IDL codegen
├── provider/handler.go      # GreetProvider (Go struct) + RestServiceConfig registration
├── provider/server.go       # DubboServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # RestServiceConfig registration + discovers, calls, asserts, exits
├── conf/app.properties      # provider configuration
├── docker-compose.yml       # local etcd
└── check.sh                 # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

Nothing was generated. REST has no protobuf/thrift IDL and no code generator
in dubbo-go v3 — the service surface is a hand-written Go file
(`proto/greet.go`) that pins the Java-style interface name, method name, and
the HTTP verb / path / query-key constants, plus a hand-written provider
struct with the matching method signature and hand-written
`RestServiceConfig` maps on both sides. Running `./gen.sh` prints a one-line
"nothing to do" for symmetry with the Triple sibling.

## Choosing this protocol vs. the siblings

| Concern              | Triple (`../triple`)                | Dubbo/Hessian2 (`../dubbo`)               | JSON-RPC (`../jsonrpc`)                                 | REST (this module)                                      |
| -------------------- | ----------------------------------- | ----------------------------------------- | ------------------------------------------------------- | ------------------------------------------------------- |
| Transport            | HTTP/2                              | Raw TCP                                   | HTTP/1.1                                                | HTTP/1.1                                                |
| Payload              | protobuf                            | Hessian2                                  | JSON-RPC 2.0 envelope                                   | Plain JSON, no envelope                                 |
| URL layout           | fixed by protocol                   | fixed by protocol                         | fixed by protocol (`POST /<interface>`)                 | user-defined per method (verb + path + param source)    |
| IDL                  | `.proto` + `protoc-gen-go-triple`   | none — hand-written Go structs            | none — hand-written Go structs                          | none — Go structs + hand-written RestServiceConfig maps |
| Client-side wiring   | Typed stub                          | Interface name only                       | Interface name only                                     | Interface name + method-mapping map                     |
| Cross-language reach | Any gRPC/Triple client              | Java Dubbo (native), Hessian2 runtimes    | Anything speaking HTTP + JSON                           | Anything speaking HTTP (curl, browsers, gateways, ...)  |
| When to pick         | Greenfield Go microservices         | Interop with existing Java Dubbo services | Bare-HTTP clients / lowest common denominator           | REST-style public APIs, gateway-friendly endpoints      |

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only the REST endpoint.
spring.http.server.enabled=false

# REST bind port; read via the ${spring.dubbo.server} prefix, default 20003
# (20000/20001/20002 are reserved for the Triple/Dubbo/JSON-RPC siblings so
# all four can coexist on one host).
spring.dubbo.server.port=20003

# etcd registry address; matches docker-compose.yml.
spring.dubbo.server.registry.etcd=127.0.0.1:2379
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

Terminal B — start the consumer (discovers via etcd and calls):

```bash
go run ./consumer
```

Expected consumer output:

```
Response from discovered provider: Hello, Dubbo-Go!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash check.sh
```
