# go-kratos (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kratos](https://go-kratos.dev/en/) `Greeter` example that starts from code
the `kratos` toolchain scaffolds and is then refactored to boot and be
configured the Go-Spring way: `gs.Run()` drives the lifecycle, every layer is
wired through the Go-Spring IoC container instead of `google/wire`, and the
server bind addresses come from `conf/app.properties` instead of Kratos' YAML
config.

It exposes three kratos transports for the Greeter — the **HTTP** (`:8000`)
and **gRPC** (`:9000`) endpoints the scaffold generates, plus a
**WebSocket** endpoint (`:9002`) from the
[`kratos-transport`](https://github.com/tx7do/kratos-transport) ecosystem —
and wires in an **etcd registry** for real **service registration &
discovery**: on startup the provider registers the `kratos-greeter` app into
etcd; the consumer never learns the provider's host:port and instead resolves
a live endpoint from the same etcd via kratos' `discovery:///` scheme. This is
the microservice governance Kratos advertises — not the earlier registry-less
direct connection.

All three transports live in one kratos.App because they can coexist as
`transport.Server` implementations — see the `internal/server` refactor notes
below. Transports that CANNOT coexist (e.g. an MQTT broker-backed transport
that needs an external mosquitto) would warrant a separate subdirectory; the
kitex example follows that pattern for its incompatible protocol modes.

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
  │ → ws://<host>:9002                       │
┌─┴────────────────┐                  ┌──────┴─────┐
│  provider        │◀───── gRPC ──────│  consumer  │
│  gs.Run()        │  SayHello(name)  │ one-shot   │
│  :8000/:9000/    │                  │ assert+exit│
│  :9002 (ws)      │◀── WebSocket ────│            │
│                  │  {type:1,name}   │            │
└──────────────────┘                  └────────────┘
```

## Layout

```
contrib/go-kratos/
├── api/helloworld/v1/          # protoc-generated gRPC + HTTP stubs (DO NOT EDIT)
├── internal/biz/               # domain usecase (GreeterUsecase + GreeterRepo interface)
├── internal/data/              # data layer (Data, greeterRepo) + shared kratos logger bean
├── internal/service/           # service layer (GreeterService)
├── provider/handler.go         # ServiceRegister bean, binds GreeterService to
│                               #   HTTP, gRPC and WebSocket transports
├── provider/server.go          # KratosServer adapter (gs.Server) + Config, composes
│                               #   kratos.App with all three transports + etcd Registrar
├── provider/main.go            # gs.Run(); long-lived, publishes into etcd
├── consumer/main.go            # discovers the provider via etcd, calls SayHello over
│                               #   gRPC AND dials the WebSocket endpoint, asserts both
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
one Kratos `App` serves those two transports plus the WebSocket transport —
that is why, unlike the kitex example, this project is **not** split into
per-protocol subdirectories. WebSocket carries application-defined framed
messages rather than proto RPCs, so its request/reply shape is hand-defined in
`provider/handler.go` (see `WSHelloRequest` / `WSHelloReply`) and shared with
the consumer as a text envelope; there is no additional codegen step for WS.

## Why WebSocket lives here (and MQTT does not)

The [`kratos-transport`](https://github.com/tx7do/kratos-transport) ecosystem
exposes many transports on top of the Kratos framework — WebSocket, MQTT,
NATS, Kafka, RabbitMQ, and more. Each one implements the same
`transport.Server` interface, so any of them can be added to
`kratos.Server(...)` alongside HTTP+gRPC. The rule this repo follows is:
protocols that CAN coexist go in the same project; only protocols that CANNOT
coexist get split into their own subdirectory.

- **WebSocket** needs no external dependency (only a TCP listener), so it
  runs in the same kratos.App as HTTP+gRPC. That is why it lives in this
  single project rather than a `contrib/go-kratos/websocket/` subdir.
- **MQTT** would need an external broker (e.g. eclipse-mosquitto in
  docker). Adding a broker service to `docker-compose.yml` is technically
  possible, but at that point MQTT stops being an incremental transport
  demo and becomes a pub/sub example with different semantics. It is
  intentionally skipped here; a real MQTT example belongs in a separate
  subdirectory with its own broker container.

## The refactor: native Kratos → Go-Spring + registry

| Concern             | Kratos scaffold                                             | Go-Spring version                                                                        |
| ------------------- | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Startup             | `kratos.New(...).Run()` owns the process                    | `KratosServer` implements `gs.Server`; `gs.Run()` drives Run/Stop                        |
| Dependency wiring   | `google/wire` `ProviderSet` + generated `wire_gen.go`       | `init()` + `gs.Provide` per layer; blank imports in `provider/main.go` trigger registration |
| Handler wiring      | `v1.RegisterGreeterHTTPServer(hs, impl)` in `internal/server` | `ServiceRegister` bean binds `GreeterService` to all three transports (HTTP, gRPC, WS) |
| Server enable       | always on                                                   | `KratosServer` conditional on a `ServiceRegister` bean via `gs.OnBean`                   |
| Config source       | `configs/config.yaml` scanned into `conf.proto` `Bootstrap` | `conf/app.properties` bound via `value:"${spring.kratos.http}"` / `${spring.kratos.grpc}` |
| Registration        | none (direct)                                               | provider `kratos.Registrar(etcd.New(clientv3.New(...)))` into etcd                       |
| Discovery           | consumer `transgrpc.WithEndpoint("host:port")`              | consumer `transgrpc.WithEndpoint("discovery:///<name>") + WithDiscovery(etcd.New(...))`  |
| Shutdown            | `kratos.App` traps SIGTERM itself                           | graceful shutdown by Go-Spring (SIGTERM → `Stop()` → `App.Stop()`, deregisters from etcd) |

The adapter in `provider/server.go` is the crux: Kratos registers services at
the `kratos.App` level (not per-transport), so `khttp.Server`, `kgrpc.Server`
and the kratos-transport `websocket.Server` are all built and then passed
together into `kratos.New(...)` with `kratos.Registrar(etcdRegistry)`.
`App.Run` binds every listener, publishes the service instance into etcd
(all three endpoints, tagged by kratos "kind"), and blocks forever, so it
runs in a goroutine started only after `sig.TriggerAndWait()`, while `Run`
parks on a done channel that `Stop()` closes to hand control back to
Go-Spring's shutdown (which then calls `App.Stop` to deregister and stop each
transport in turn).

The consumer only supplies the etcd address and the service name: the
`discovery:///` scheme wired via `transgrpc.WithDiscovery(r)` lets kratos find a
live provider in etcd and dial it via gRPC. The WebSocket leg dials the
`ws://` endpoint directly (`--ws ws://127.0.0.1:9002/`), because
kratos-transport's WS client has no discovery hook; adding one just to demo an
extra transport would obscure what the transport does. gRPC proves discovery
works; WS proves coexistence.

## WebSocket wire format

kratos-transport WebSocket is a **message-typed** pipe, not RPC-typed. This
example uses `PayloadTypeBinary`, so each frame on the wire is

```
<4-byte little-endian uint32 messageType><JSON-encoded payload bytes>
```

where `messageType` is an application-defined discriminator that routes a
frame to a server-side handler, and the payload is the JSON-encoded
application struct. The Greeter uses `messageType=1` with `{"name":"<x>"}`
request and `{"message":"Hello <x>"}` reply. Because it is not RPC, there is
no proto contract; the constant and the two structs are the whole contract,
shared by provider (`provider/handler.go`) and consumer (`consumer/main.go`).

Binary is chosen over the library's text-mode envelope for a specific reason.
The text-mode server has an **asymmetric** wire format in the pinned version:
it unwraps `{"type","payload"}` on receive but sends replies as raw codec
bytes (no envelope). That would force the consumer to speak two different
formats depending on direction. Binary is symmetric — the server writes the
same 4-byte header on the way out that it expects on the way in — so the
same marshal/unmarshal pair works in both directions. See the comment in
`provider/server.go` for the version pin (`v1.3.1`).

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

# Kratos WebSocket transport (kratos-transport/transport/websocket), read via
# ${spring.kratos.ws}. Runs alongside HTTP+gRPC in the same kratos.App.
spring.kratos.ws.addr=0.0.0.0:9002
spring.kratos.ws.path=/

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
Response from discovered provider (gRPC): Hello Kratos
Response from discovered provider (WebSocket): Hello Kratos-WS
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash check.sh
```
