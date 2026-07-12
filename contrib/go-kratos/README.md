# go-kratos (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kratos](https://go-kratos.dev/en/) `Greeter` example that starts from code
the `kratos` toolchain scaffolds and is then refactored to boot and be
configured the Go-Spring way: `gs.Run()` drives the lifecycle, every layer is
wired through the Go-Spring IoC container instead of `google/wire`, and the
server bind addresses come from `conf/app.properties` instead of Kratos' YAML
config.

It exposes both the **HTTP** (`:8000`) and **gRPC** (`:9000`) Greeter endpoints
the scaffold generates, and wires in an **etcd registry** for real **service
registration & discovery**: on startup the provider registers the
`kratos-greeter` app into etcd; the consumer never learns the provider's
host:port and instead resolves a live endpoint from the same etcd via kratos'
`discovery:///` scheme. This is the microservice governance Kratos advertises —
not the earlier registry-less direct connection.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ kratos-greeter                           │ resolve provider addr
  │ → grpc://<host>:9000                     │
  │ → http://<host>:8000                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      SayHello(name)    │ one-shot   │
│ :8000/:9000│──────────────────────▶│ assert+exit│
└────────────┘       "Hello "+name    └────────────┘
```

## Layout

```
contrib/go-kratos/
├── api/helloworld/v1/          # protoc-generated gRPC + HTTP stubs (DO NOT EDIT)
├── internal/biz/               # domain usecase (GreeterUsecase + GreeterRepo interface)
├── internal/data/              # data layer (Data, greeterRepo) + shared kratos logger bean
├── internal/service/           # service layer (GreeterService)
├── provider/handler.go         # ServiceRegister bean, binds GreeterService to HTTP+gRPC
├── provider/server.go          # KratosServer adapter (gs.Server) + Config, composes
│                               #   kratos.App with the etcd Registrar
├── provider/main.go            # gs.Run(); long-lived, publishes into etcd
├── consumer/main.go            # discovers the provider via etcd, calls SayHello, asserts
├── conf/app.properties         # provider configuration
├── docker-compose.yml          # local etcd
└── check.sh                    # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

```bash
# tool (once)
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

# scaffold the project (clones the kratos-layout template)
kratos new go-kratos
```

The scaffold produces `cmd/` (a `wire` + `kratos.App` bootstrap), `configs/config.yaml`,
`internal/conf/` (a `conf.proto` Bootstrap message), and the layered `internal/`
code. The refactor drops `cmd/`, `configs/`, `internal/conf/`, and the `wire`
files, keeps the generated `api/` stubs untouched, and rewires the rest as a
provider + consumer pair.

The `api/helloworld/v1/*.pb.go`, `*_grpc.pb.go`, and `*_http.pb.go` stubs can be
regenerated from the `.proto` files by running `./gen.sh` (a thin wrapper around
`kratos proto client`). A single `.proto` yields both HTTP and gRPC stubs, and
one Kratos `App` serves both transports — that is why, unlike the kitex example,
this project is **not** split into per-protocol subdirectories.

## The refactor: native Kratos → Go-Spring + registry

| Concern             | Kratos scaffold                                             | Go-Spring version                                                                        |
| ------------------- | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Startup             | `kratos.New(...).Run()` owns the process                    | `KratosServer` implements `gs.Server`; `gs.Run()` drives Run/Stop                        |
| Dependency wiring   | `google/wire` `ProviderSet` + generated `wire_gen.go`       | `init()` + `gs.Provide` per layer; blank imports in `provider/main.go` trigger registration |
| Handler wiring      | `v1.RegisterGreeterHTTPServer(hs, impl)` in `internal/server` | `ServiceRegister` bean binds `GreeterService` to both transports                       |
| Server enable       | always on                                                   | `KratosServer` conditional on a `ServiceRegister` bean via `gs.OnBean`                   |
| Config source       | `configs/config.yaml` scanned into `conf.proto` `Bootstrap` | `conf/app.properties` bound via `value:"${spring.kratos.http}"` / `${spring.kratos.grpc}` |
| Registration        | none (direct)                                               | provider `kratos.Registrar(etcd.New(clientv3.New(...)))` into etcd                       |
| Discovery           | consumer `transgrpc.WithEndpoint("host:port")`              | consumer `transgrpc.WithEndpoint("discovery:///<name>") + WithDiscovery(etcd.New(...))`  |
| Shutdown            | `kratos.App` traps SIGTERM itself                           | graceful shutdown by Go-Spring (SIGTERM → `Stop()` → `App.Stop()`, deregisters from etcd) |

The adapter in `provider/server.go` is the crux: Kratos registers services at
the `kratos.App` level (not per-transport), so both `khttp.Server` and
`kgrpc.Server` are built and then passed together into `kratos.New(...)` with
`kratos.Registrar(etcdRegistry)`. `App.Run` binds the listeners, publishes the
service instance into etcd, and blocks forever, so it runs in a goroutine
started only after `sig.TriggerAndWait()`, while `Run` parks on a done channel
that `Stop()` closes to hand control back to Go-Spring's shutdown (which then
calls `App.Stop` to deregister and stop each transport).

The consumer only supplies the etcd address and the service name: the
`discovery:///` scheme wired via `transgrpc.WithDiscovery(r)` lets kratos find a
live provider in etcd and dial it via gRPC.

## Choosing a registry

This example standardizes on **etcd** for easy cross-comparison with the other
contrib examples. Kratos contrib also ships adapters for **Consul**, **Nacos**,
**ZooKeeper**, and **Polaris** — swap the provider's
`etcd.New(clientv3.New(...))` and the consumer's matching call for
`consul.New(...)` / `nacos.New(...)` / `zookeeper.New(...)` /
`polaris.New(...)`, and adjust the client config. With Nacos you can also
inspect the registered services directly in its built-in `:8848/nacos` console.

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only kratos transports.
spring.http.server.enabled=false

# Application name — the key under which the kratos.App is registered in etcd.
spring.kratos.name=kratos-greeter

# Kratos HTTP transport, read via ${spring.kratos.http}.
spring.kratos.http.addr=0.0.0.0:8000
spring.kratos.http.timeout=1s

# Kratos gRPC transport, read via ${spring.kratos.grpc}.
spring.kratos.grpc.addr=0.0.0.0:9000
spring.kratos.grpc.timeout=1s

# etcd registry address; matches docker-compose.yml.
spring.kratos.registry.etcd=127.0.0.1:2379
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
Response from discovered provider: Hello Kratos
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash check.sh
```
