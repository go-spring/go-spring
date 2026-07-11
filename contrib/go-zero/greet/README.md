# go-zero (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [go-zero](https://go-zero.dev) `Greet` example that starts from code the
go-zero toolchain generates and is then refactored to boot and be configured
the Go-Spring way: `gs.Run()` drives the lifecycle, the provider is an IoC
bean, and the bind address comes from `conf/app.properties` instead of
hard-coded `main()` wiring.

It runs a **zrpc (gRPC) service** and wires in an **etcd registry** for real
**service registration & discovery**: on startup the provider registers the
`greet.rpc` key into etcd; the consumer never learns the provider's host:port
and instead resolves a live address from the same etcd.

This is a runnable example, **not** a reusable starter module.

## Why zrpc instead of REST — the important difference

Unlike the other four contrib examples (dubbo-go, kitex, kratos, goframe),
go-zero has **no service discovery in its REST server** (`rest.Server`). The
whole registry story only exists in **zrpc** (go-zero's gRPC layer). So to
demonstrate go-zero's real service governance we must ship a zrpc-based
example — a REST version would be a fake, hard-coded direct call.

That is why this example diverges from a stock go-zero REST tutorial: the IDL
is a protobuf, the provider is a zrpc server, and the consumer is a zrpc
client. `spring.http.server.enabled=false`.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │  greet.rpc  └──────────────┘  greet.rpc  │
  │             (key)              (key)     │ resolve provider addr
  │ → grpc://<host>:8081                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      Greet(name)       │ one-shot   │
│ :8081      │──────────────────────▶│ assert+exit│
└────────────┘       echo name        └────────────┘
```

## Layout

```
contrib/go-zero/greet/
├── greet.proto             # Protobuf IDL
├── pb/greet.pb.go          # protoc-generated messages (DO NOT EDIT)
├── pb/greet_grpc.pb.go     # protoc-generated gRPC stubs (DO NOT EDIT)
├── provider/handler.go     # GreetProvider, exported as a ServiceRegister bean
├── provider/server.go      # ZrpcServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go        # gs.Run(); long-lived, registers into etcd
├── consumer/main.go        # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties     # provider configuration
├── docker-compose.yml      # local etcd
└── check.sh                # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

Two paths work. Both produce standard `pb.RegisterGreetServer` /
`pb.NewGreetClient` stubs that the provider and consumer consume directly.

### Path A: `protoc` (used here)

```bash
# tools (once)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# generate messages + gRPC stubs from the IDL
protoc --proto_path=. \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  greet.proto
mv greet.pb.go greet_grpc.pb.go pb/
```

### Path B: `goctl rpc protoc`

```bash
# tools (once)
go install github.com/zeromicro/go-zero/tools/goctl@latest

goctl rpc protoc greet.proto \
  --go_out=./pb --go-grpc_out=./pb --zrpc_out=.
```

`goctl` additionally scaffolds an `etc/*.yaml` + `internal/{config,logic,server,svc}`
tree. This example intentionally does not use that tree — Go-Spring owns the
lifecycle and configuration, so we keep only the `pb/` output.

## The refactor: native go-zero → Go-Spring + registry

| Concern         | Stock go-zero (REST scaffold)              | Go-Spring version (zrpc + etcd)                                                     |
| --------------- | ------------------------------------------ | ----------------------------------------------------------------------------------- |
| Transport       | `rest.Server` (HTTP)                       | `zrpc.RpcServer` (gRPC) — required for discovery                                    |
| IDL             | `greet.api`                                | `greet.proto` + `pb/*.pb.go` / `pb/*_grpc.pb.go`                                    |
| Startup         | `server.Start()` blocks in `main()`        | `ZrpcServer` implements `gs.Server`; `gs.Run()` drives Run/Stop                     |
| Handler wiring  | `handler.RegisterHandlers(server, svcCtx)` | `gs.Provide(func() ServiceRegister { return pb.RegisterGreetServer(...) })`         |
| Server enable   | always on                                  | conditional on a `ServiceRegister` bean via `gs.OnBean`                             |
| Listen addr     | hard-coded YAML                            | `${spring.zrpc.server.listen-on}` from `conf/app.properties`                        |
| Registration    | none (REST has no discovery)               | provider `zrpc.RpcServerConf{Etcd: discov.EtcdConf{Hosts, Key}}` registers to etcd  |
| Discovery       | none                                       | consumer `zrpc.RpcClientConf{Etcd: discov.EtcdConf{Hosts, Key}}` resolves from etcd |
| Shutdown        | process-owned                              | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, deregisters from etcd)          |

The adapter in `provider/server.go` is the crux: `zrpc.RpcServer.Start()` binds
the listener, registers the provider into etcd, and then blocks forever, so it
runs in a goroutine started only after `sig.TriggerAndWait()`, while `Run`
parks on a done channel that `Stop()` closes to hand control back to
Go-Spring's shutdown (which then calls `zrpc.RpcServer.Stop()`).

The consumer only supplies the etcd address + key, never the provider's: zrpc
resolves that key to a live provider address in etcd and calls it.

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only zrpc.
spring.http.server.enabled=false

# zrpc bind address; read via the ${spring.zrpc.server} prefix.
spring.zrpc.server.listen-on=0.0.0.0:8081

# etcd registry address + key; matches docker-compose.yml.
spring.zrpc.server.etcd.addr=127.0.0.1:2379
spring.zrpc.server.etcd.key=greet.rpc
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
Response from discovered provider: Hello, go-zero!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash check.sh
```
