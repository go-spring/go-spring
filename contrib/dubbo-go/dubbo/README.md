# dubbo-go — Dubbo/Hessian2 (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` example that speaks the **classic Dubbo protocol** — TCP
transport with **Hessian2** serialization — refactored to boot and be
configured the Go-Spring way: `gs.Run()` drives the lifecycle, the provider
is an IoC bean, and the bind port comes from `conf/app.properties` instead of
hard-coded `main()` wiring.

Unlike the Triple sibling in [`../triple`](../triple), this protocol has no
protobuf IDL and no code generator in dubbo-go v3: services are plain Go
structs whose exported method signatures are reflected over at registration
time and marshalled with Hessian2 on the wire. That makes classic Dubbo the
interop path for calling into Java Dubbo services (which use the same
protocol natively); Triple is the recommended protocol for greenfield Go
microservices.

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
  │ → dubbo://<host>:20001                   │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀─── Dubbo (Hessian2) ──│  consumer  │
│ gs.Run()   │      Greet(name)       │ one-shot   │
│ :20001     │──────────────────────▶│ assert+exit│
└────────────┘       echo name        └────────────┘
```

## Layout

```
contrib/dubbo-go/dubbo/
├── proto/greet.go           # the "IDL": interface name + method-name constants
├── gen.sh                   # no-op — classic Dubbo has no IDL codegen
├── provider/handler.go      # GreetProvider (Go struct, reflected at registration)
├── provider/server.go       # DubboServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties      # provider configuration
├── docker-compose.yml       # local etcd
└── check.sh                 # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

Nothing was generated. Classic Dubbo/Hessian2 has no protobuf/thrift IDL and
no code generator in dubbo-go v3 — the service surface is a hand-written Go
file (`proto/greet.go`) that pins the Java-style interface name and method
names, plus a hand-written provider struct with the matching method
signature. Running `./gen.sh` prints a one-line "nothing to do" for symmetry
with the Triple sibling.

If your service uses non-primitive types, register them with
`hessian.RegisterPOJO(&MyStruct{})` — Hessian2 needs the Go↔Java type map to
be seeded at process start. This example uses only `string`, so no
registration is needed.

## Choosing this protocol vs. Triple

| Concern              | Triple (`../triple`)                  | Dubbo/Hessian2 (this module)                            |
| -------------------- | ------------------------------------- | ------------------------------------------------------- |
| Transport            | HTTP/2                                | Raw TCP                                                 |
| Payload              | protobuf                              | Hessian2                                                |
| IDL                  | `.proto` + `protoc-gen-go-triple`     | none — hand-written Go structs                          |
| Cross-language reach | Any gRPC/Triple client                | Java Dubbo (native), any Hessian2-capable runtime       |
| Client call style    | Typed stub (`svc.Greet(ctx, req)`)    | Reflective (`conn.CallUnary(ctx, args, resp, "Greet")`) |
| When to pick         | Greenfield Go microservices           | Interop with existing Java Dubbo services               |

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only Dubbo.
spring.http.server.enabled=false

# Dubbo bind port; read via the ${spring.dubbo.server} prefix, default 20001
# (20000 is reserved for the Triple sibling so both can coexist on one host).
spring.dubbo.server.port=20001

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
