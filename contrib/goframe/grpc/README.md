# goframe — gRPC (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [GoFrame](https://goframe.org) `EchoService` example generated from a
**protobuf** IDL via `protoc` + the standard Go plugins and served by goframe's
gRPC layer (`github.com/gogf/gf/contrib/rpc/grpcx/v2`). It is refactored to
boot and be configured the Go-Spring way: `gs.Run()` drives the lifecycle, the
`grpcx.GrpcServer` is an IoC bean, the handler is exported as an
`echo.EchoServiceServer` bean, and the bind address comes from
`conf/app.properties` instead of `manifest/config/config.yaml`.

It wires in an **etcd registry** (via
`github.com/gogf/gf/contrib/registry/etcd/v2`) for real **service registration
& discovery**: on startup the provider registers `goframe.grpc.echo` into
etcd; the consumer never learns the provider's host:port and instead resolves
a live address from the same etcd through grpcx's discovery resolver.

This is the **gRPC** protocol variant. For the HTTP variant (goframe
`*ghttp.Server` + `gf gen ctrl` codegen chain), see [`../http`](../http). The
two are split because goframe uses two different server types with two
different codegen pipelines; nothing forces them into a single provider.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ goframe.grpc.echo                        │ resolve provider addr
  │ → <host>:8001                            │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── gRPC ────────│  consumer  │
│ gs.Run()   │      Echo(message)     │ one-shot   │
│ :8001      │──────────────────────▶│ assert+exit│
└────────────┘       echo message     └────────────┘
```

## Layout

```
contrib/goframe/grpc/
├── idl/echo.proto              # protobuf IDL
├── idl/echo/                   # protoc-generated Go stubs (DO NOT EDIT)
├── idl/gen-code.sh             # regenerates idl/echo/ from the IDL
├── provider/handler.go         # EchoServiceImpl, exported as an echo.EchoServiceServer bean
├── provider/server.go          # GoFrameGrpcServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go            # gs.Run(); long-lived, registers into etcd
├── consumer/main.go            # discovers via etcd, calls Echo and asserts, then exits
├── conf/app.properties         # provider configuration
├── docker-compose.yml          # local etcd
└── scripts/smoke-test.sh       # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

```bash
# tools (once)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# regenerate the gRPC stubs from the IDL (or just run ./idl/gen-code.sh)
protoc \
    --proto_path=idl \
    --go_out=. \
    --go_opt=module=go-spring.org/goframe/grpc \
    --go-grpc_out=. \
    --go-grpc_opt=module=go-spring.org/goframe/grpc \
    idl/echo.proto
```

The `option go_package = "go-spring.org/goframe/grpc/idl/echo;echo";` line
in `echo.proto` pins the output package to `idl/echo/` under the module root.
Re-running `./idl/gen-code.sh` regenerates `idl/echo/` without touching the
refactored provider/consumer code.

Unlike goframe's HTTP `gf gen ctrl` chain (which parses `api/*/v*/` types and
emits controllers), gRPC codegen is plain `protoc` + `protoc-gen-go` +
`protoc-gen-go-grpc`. `gf gen pb` is a thin wrapper around the same command
and is not required to keep this example runnable.

## The refactor: native grpcx → Go-Spring + registry

| Concern         | grpcx scaffold                                                             | Go-Spring version                                                                                             |
| --------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Startup         | `s.Run()` blocks in `main()`, installs its own `gproc` signal handler      | `GoFrameGrpcServer` implements `gs.Server`; `gs.Run()` drives Run/Stop; `s.Start()` + park-on-done            |
| Handler wiring  | `echo.RegisterEchoServiceServer(s.Server, &EchoServiceImpl{})` in `main()` | `gs.Provide(&EchoServiceImpl{}).Export(gs.As[echo.EchoServiceServer]())` + `Register…` inside the constructor |
| Server enable   | always on                                                                  | conditional on an `echo.EchoServiceServer` bean via `gs.OnBean`                                               |
| Address         | `grpcx.Server.NewConfig()` reads `grpc.address` from `config.yaml`         | `${goframe.grpc.address}` from `conf/app.properties`                                                          |
| Registration    | none (direct)                                                              | provider `gsvc.SetRegistry(etcd.New(addr))` before `grpcx.Server.New`                                         |
| Discovery       | consumer `grpc.NewClient("host:port")`                                     | consumer `grpcx.Client.MustNewGrpcClientConn(serviceName)` (gsvc scheme resolves via etcd)                    |
| Shutdown        | process-owned via `gproc`                                                  | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, `GracefulStop` + etcd deregister)                         |

The adapter in `provider/server.go` is the crux. `grpcx.GrpcServer` snapshots
`gsvc.GetRegistry()` at construction time (see the grpcx source:
`registrar: gsvc.GetRegistry()`), so the constructor sets the etcd registry
*before* calling `grpcx.Server.New`. `s.Start()` is non-blocking, so `Run`
parks on a done channel that `Stop()` closes to hand control back to
Go-Spring's shutdown, which in turn triggers `s.Stop()` (GracefulStop + etcd
deregister). Using `Start` deliberately avoids `grpcx.Server.Run`, which
installs its own `gproc` signal handler that would fight the Go-Spring
lifecycle.

The consumer only supplies the etcd address, never the provider's: it passes
the service name (`goframe.grpc.echo`, matching `goframe.grpc.name` in
`conf/app.properties`) to `grpcx.Client.MustNewGrpcClientConn`, which builds
a `gsvc://<name>` target and lets grpcx's resolver look it up in etcd.

## Choosing a registry

This example standardises on **etcd** for easy cross-comparison with the other
contrib examples. `github.com/gogf/gf/contrib/registry/*` also ships
**Nacos**, **ZooKeeper** and **Polaris** adapters that satisfy the same
`gsvc.Registry` interface: swap
`github.com/gogf/gf/contrib/registry/etcd/v2` for
`.../registry/nacos/v2` / `.../registry/zookeeper/v2` / `.../registry/polaris/v2`
and update `goframe.grpc.registry.etcd` accordingly.

## Configuration

```properties
# Disable Go-Spring's built-in HTTP server; the provider exposes only gRPC.
spring.http.server.enabled=false

# gRPC bind address for grpcx.GrpcServer.
goframe.grpc.address=:8001

# Service name registered into etcd; consumer resolves by the same name.
goframe.grpc.name=goframe.grpc.echo

# etcd registry address; matches docker-compose.yml.
goframe.grpc.registry.etcd=127.0.0.1:2379
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
Response from discovered provider: Hello, GoFrame gRPC!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash scripts/smoke-test.sh
```
