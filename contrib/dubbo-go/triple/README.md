# dubbo-go — Triple (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/) `GreetService`
example generated from a **protobuf** IDL via `protoc-gen-go-triple`, then
refactored to boot and be configured the Go-Spring way: `gs.Run()` drives the
lifecycle, the provider is an IoC bean, and the bind port comes from
`conf/app.properties` instead of hard-coded `main()` wiring.

It uses the **Triple** protocol — Dubbo's flagship protobuf-over-HTTP/2
transport that is wire-compatible with gRPC — and wires in an **etcd registry**
for real **service registration & discovery**: on startup the provider
registers `greet.GreetService` into etcd; the consumer never learns the
provider's host:port and instead resolves a live address from the same etcd.
This is the microservice governance Dubbo advertises — not the earlier
registry-less direct connection.

This is the companion to the classic Dubbo/Hessian2 variant in
[`../dubbo`](../dubbo). Triple is the recommended protocol in dubbo-go v3;
Hessian2 is kept for interop with Java Dubbo services.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ greet.GreetService                       │ resolve provider addr
  │ → tri://<host>:20000                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      Greet(name)       │ one-shot   │
│ :20000     │──────────────────────▶│ assert+exit│
└────────────┘       echo name        └────────────┘
```

## Layout

```
contrib/dubbo-go/triple/
├── proto/greet.proto        # Protobuf IDL
├── proto/greet.pb.go        # protoc-generated messages (DO NOT EDIT)
├── proto/greet.triple.go    # Triple-generated stubs (DO NOT EDIT)
├── gen.sh                   # regenerates proto/*.go from the IDL
├── provider/handler.go      # GreetProvider, wired via a ServiceRegister bean
├── provider/server.go       # DubboServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties      # provider configuration
├── docker-compose.yml       # local etcd
└── check.sh                 # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

```bash
# tools (once)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install github.com/dubbogo/protoc-gen-go-triple/v3@latest

# generate messages + Triple stubs from the IDL (or just run ./gen.sh)
protoc --proto_path=proto \
  --go_out=paths=source_relative:./proto \
  --go-triple_out=paths=source_relative:./proto \
  proto/greet.proto
```

The generator produces `greet.pb.go` and `greet.triple.go` in `proto/`, which
is shared by both the provider and the consumer. Re-running `./gen.sh`
regenerates only those files without touching the refactored business code.

> Note: on a go1.26 toolchain whose `runtime.Version()` carries an experiment
> suffix (e.g. `go1.26.1-X:jsonv2`), `protoc-gen-go-triple` v3.0.3 panics while
> parsing the version. Rebuild it from source with the version string
> truncated to its numeric part.

## The refactor: native Dubbo-go → Go-Spring + registry

| Concern         | Dubbo-go scaffold                          | Go-Spring version                                                              |
| --------------- | ------------------------------------------ | ------------------------------------------------------------------------------ |
| Startup         | `srv.Serve()` blocks in `main()`           | `DubboServer` implements `gs.Server`; `gs.Run()` drives Run/Stop               |
| Handler wiring  | `RegisterGreetServiceHandler(srv, &impl)`  | `gs.Provide(func() ServiceRegister { ... })` binds a service-agnostic register |
| Server enable   | always on                                  | conditional on a `ServiceRegister` bean via `gs.OnBean`                        |
| Port            | hard-coded default                         | `${spring.dubbo.server.port}` from `conf/app.properties`                       |
| Registration    | none (direct)                              | provider `server.WithServerRegistry(registry.WithEtcdV3(), ...)` into etcd     |
| Discovery       | consumer `WithClientURL("host:port")`      | consumer `client.WithClientRegistry(...)`, resolves by interface name from etcd |
| Shutdown        | process-owned                              | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, deregisters from etcd)     |

The adapter in `provider/server.go` is the crux: Dubbo-go's `Serve()` binds the
listener, registers the provider into etcd, and then blocks forever, so it
runs in a goroutine started only after `sig.TriggerAndWait()`, while `Run`
parks on a done channel that `Stop()` closes to hand control back to
Go-Spring's shutdown.

The consumer only supplies the etcd address, never the provider's: the
interface name `greet.GreetService` is baked into the generated stub, and
Dubbo uses it to find a live provider in etcd and call it.

## Choosing a registry

This example standardizes on **etcd** for easy cross-comparison with the other
contrib examples. Dubbo-go natively supports **Nacos**, **ZooKeeper**, and
**Polaris** as well: swap the provider's `registry.WithEtcdV3()` and the
consumer's matching option for `registry.WithNacos()` / `registry.WithZookeeper()`
and adjust `registry.WithAddress(...)`. With Nacos you can also inspect the
registered services directly in its built-in `:8848/nacos` console.

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only Dubbo.
spring.http.server.enabled=false

# Dubbo Triple bind port; read via the ${spring.dubbo.server} prefix, default 20000.
spring.dubbo.server.port=20000

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
