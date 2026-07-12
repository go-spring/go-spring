# dubbo-go — JSON-RPC (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` example that speaks the **JSON-RPC 2.0** protocol — HTTP/1.1
transport with a **JSON** body — wired the Go-Spring way via the reusable
**starter-dubbo** module: it supplies the `gs.Server` adapter, `gs.Run()`
drives the lifecycle, the provider is just a `ServiceRegister` bean, and the
protocol and registry come from `conf/app.properties` instead of hard-coded
`main()` wiring.

Unlike the Triple sibling in [`../triple`](../triple), this protocol has no
protobuf IDL and no code generator in dubbo-go v3: services are plain Go
structs whose exported method signatures are reflected over at registration
time and marshalled with `encoding/json` on the wire. That makes JSON-RPC
the interop path of last resort — anything that can speak HTTP and JSON
(curl, browsers, non-Go languages without a Dubbo SDK) can hit the provider
directly without a client library.

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
  │ → jsonrpc://<host>:20002                 │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀── JSON-RPC (HTTP/1) ──│  consumer  │
│ gs.Run()   │      Greet(name)       │ one-shot   │
│ :20002     │──────────────────────▶│ assert+exit│
└────────────┘       echo name        └────────────┘
```

## Layout

```
contrib/dubbo-go/jsonrpc/
├── proto/greet.go           # the "IDL": interface name + method-name constants
├── gen.sh                   # no-op — JSON-RPC has no IDL codegen
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean (server comes from starter-dubbo)
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties      # provider configuration
├── docker-compose.yml       # local etcd
└── check.sh                 # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

Nothing was generated. JSON-RPC has no protobuf/thrift IDL and no code
generator in dubbo-go v3 — the service surface is a hand-written Go file
(`proto/greet.go`) that pins the Java-style interface name and method
names, plus a hand-written provider struct with the matching method
signature. Running `./gen.sh` prints a one-line "nothing to do" for
symmetry with the Triple sibling.

Any JSON-serializable Go type can be used as a parameter or return; there
is no equivalent to Hessian2's POJO registration table.

## Choosing this protocol vs. Triple / classic-Dubbo

| Concern              | Triple (`../triple`)                | Dubbo/Hessian2 (`../dubbo`)               | JSON-RPC (this module)                                  |
| -------------------- | ----------------------------------- | ----------------------------------------- | ------------------------------------------------------- |
| Transport            | HTTP/2                              | Raw TCP                                   | HTTP/1.1                                                |
| Payload              | protobuf                            | Hessian2                                  | JSON                                                    |
| IDL                  | `.proto` + `protoc-gen-go-triple`   | none — hand-written Go structs            | none — hand-written Go structs                          |
| Cross-language reach | Any gRPC/Triple client              | Java Dubbo (native), Hessian2 runtimes    | Anything speaking HTTP + JSON (curl, browsers, ...)     |
| Client call style    | Typed stub (`svc.Greet(ctx, req)`)  | Reflective (`conn.CallUnary(...)`)        | Reflective (`conn.CallUnary(...)`)                      |
| When to pick         | Greenfield Go microservices         | Interop with existing Java Dubbo services | Debugging / bare-HTTP clients / lowest common denominator |

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only JSON-RPC.
spring.http.server.enabled=false

# JSON-RPC bind port; the key under ${spring.dubbo.server.protocols} is the
# dubbo-go protocol name. JSON-RPC on 20002 (20000/20001 are reserved for the
# Triple/Dubbo siblings so all three can coexist on one host).
spring.dubbo.server.protocols.jsonrpc.port=20002

# etcd registry, map-driven: the key under ${spring.dubbo.server.registries}
# is the dubbo-go registry name. Matches docker-compose.yml.
spring.dubbo.server.registries.etcdv3.address=127.0.0.1:2379
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

## Known upstream issue: Go 1.26 (`jsonv2` experiment) x dubbo-go v3.3.1

Under a Go toolchain built with the `jsonv2` experiment enabled
(`runtime.Version()` carries an `-X:jsonv2` suffix, the default on Go 1.26),
`dubbo.apache.org/dubbo-go/v3/protocol/jsonrpc.(*serverRequest).UnmarshalJSON`
recurses indefinitely: it calls `encoding/json.Unmarshal` on its own receiver
type, and the v2 arshaler treats the method as an override and dispatches
back into `UnmarshalJSON`, exploding the goroutine stack. The provider
process crashes on the first request, so the next consumer dial to the same
port fails with `connect: connection refused`.

The example code itself is correct — this is an upstream defect in dubbo-go
v3.3.1's JSONRPC protocol implementation. Options:

- Run this example on a Go toolchain **without** the `jsonv2` experiment
  (`GOEXPERIMENT=nojsonv2` at Go-build time, or a Go 1.25 toolchain).
- Wait for a dubbo-go release that stops calling `json.Unmarshal` on the
  method receiver from inside its own `UnmarshalJSON`.

`go build ./...` / `go vet ./...` succeed on any toolchain; `check.sh` will
fail until the upstream fix is in place.
