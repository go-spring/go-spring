# go-zero — zRPC/gRPC (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [go-zero](https://go-zero.dev) `Greet` example whose stubs come from a
**protobuf** IDL via `goctl rpc protoc`, then refactored to boot and be
configured the Go-Spring way: `gs.Run()` drives the lifecycle, the handler is
an IoC bean, and the bind address comes from `conf/app.properties` instead of
hard-coded `main()` wiring.

The service runs on **zRPC** — go-zero's gRPC layer — and wires in an **etcd
registry** for real **service registration & discovery**: on startup the
provider registers the `greet.rpc` key into etcd; the consumer never learns
the provider's host:port and instead resolves a live address from the same
etcd.

This is the RPC half of the go-zero examples. The HTTP/REST half — same
`Greet` service, but generated from a `.api` file with `goctl api go` — lives
next door in [`../greet-api`](../greet-api).

This is a runnable example, **not** a reusable starter module.

## Why zRPC, and why is there no etcd here for the REST sibling?

Unlike the other framework examples (dubbo-go, kitex, kratos, goframe),
go-zero has **no service discovery in its REST server** (`rest.Server`). The
whole registry story exists only in **zRPC**. To demonstrate go-zero's real
service governance we ship this zRPC-based example — a REST version would be
a fake, hard-coded direct call. The sibling `greet-api/` therefore keeps the
same Go-Spring wiring pattern but drops etcd, and calls the provider
directly over HTTP.

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
contrib/go-zero/greet-rpc/
├── idl/greet.proto         # Protobuf IDL
├── idl/gen-code.sh         # regenerates idl/ stubs from idl/greet.proto via goctl
├── idl/greet.pb.go         # protoc-generated messages (DO NOT EDIT)
├── idl/greet_grpc.pb.go    # protoc-generated gRPC stubs (DO NOT EDIT)
├── provider/handler.go     # GreetProvider, exported as a ServiceRegister bean
├── provider/server.go      # ZrpcServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go        # gs.Run(); long-lived, registers into etcd
├── consumer/main.go        # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties     # provider configuration (incl. observability)
├── docker-compose.yml      # etcd + observability backends (prometheus/jaeger/loki/promtail)
├── docker/                 # prometheus.yml + promtail-config.yml
└── scripts/smoke-test.sh   # smoke test: bring up etcd+backends+provider, run consumer, assert, tear down
```

## How it was generated

```bash
# tools (once)
go install github.com/zeromicro/go-zero/tools/goctl@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# scaffold idl/ stubs from the IDL (or just run ./idl/gen-code.sh)
goctl rpc protoc idl/greet.proto --go_out=./idl --go-grpc_out=./idl --zrpc_out=<tmp>
```

`goctl rpc protoc` normally wants to scaffold an `etc/*.yaml` +
`internal/{config,logic,server,svc}` tree under `--zrpc_out`. That tree is
what a stock go-zero project would use for lifecycle and config; here we
throw it away and keep only the `idl/` stubs — Go-Spring owns the lifecycle
and configuration. `idl/gen-code.sh` points `--zrpc_out` at a `mktemp -d` directory
and deletes it, so re-running never touches the hand-written
provider/consumer.

## The refactor: native go-zero → Go-Spring + registry

| Concern         | Stock go-zero zRPC scaffold                | Go-Spring version (zRPC + etcd)                                                     |
| --------------- | ------------------------------------------ | ----------------------------------------------------------------------------------- |
| Startup         | `server.Start()` blocks in `main()`        | `ZrpcServer` implements `gs.Server`; `gs.Run()` drives Run/Stop                     |
| Handler wiring  | `server.RegisterGreetServer(srv, logic)`   | `gs.Provide(func() ServiceRegister { return greet.RegisterGreetServer(...) })`      |
| Server enable   | always on                                  | conditional on a `ServiceRegister` bean via `gs.OnBean`                             |
| Listen addr     | hard-coded YAML                            | `${spring.zrpc.server.listen-on}` from `conf/app.properties`                        |
| Registration    | zrpc.RpcServerConf inline in main          | Config struct bound from `${spring.zrpc.server}` prefix                             |
| Shutdown        | process-owned                              | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, deregisters from etcd)          |

The adapter in `provider/server.go` is the crux: `zrpc.RpcServer.Start()`
binds the listener, registers the provider into etcd, and then blocks
forever, so it runs in a goroutine started only after `sig.TriggerAndWait()`,
while `Run` parks on a done channel that `Stop()` closes to hand control back
to Go-Spring's shutdown.

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only zRPC.
spring.http.server.enabled=false

# zRPC bind address; read via the ${spring.zrpc.server} prefix.
spring.zrpc.server.listen-on=0.0.0.0:8081

# etcd registry address + key; matches docker-compose.yml.
spring.zrpc.server.etcd.addr=127.0.0.1:2379
spring.zrpc.server.etcd.key=greet.rpc

# Observability (provider-only). See the Observability section below.
spring.zrpc.server.tracing.endpoint=127.0.0.1:4317
spring.zrpc.server.metrics.port=6060
spring.zrpc.server.log.mode=file
spring.zrpc.server.log.path=../logs
```

## Observability

Unlike the dubbo-go examples — where `starter-dubbo` hand-wires OTel and
Prometheus — go-zero ships all three pillars natively. `zrpc.MustNewServer`
calls `service.ServiceConf.SetUp()` internally (the same code path
`rest.MustNewServer` uses next door in `greet-api`), which starts the tracing
agent, the metrics DevServer and logx; the zrpc server's default interceptors
(trace / prometheus / stat / log) then instrument every RPC. **We write no
OpenTelemetry/Prometheus code** — `provider/server.go` only populates the
`ServiceConf` fields from `conf/app.properties`.

| Pillar  | go-zero field           | Backend (docker-compose.yml)               |
| ------- | ----------------------- | ------------------------------------------ |
| Tracing | `ServiceConf.Telemetry` | Jaeger via OTLP/gRPC (:4317, UI 16686)     |
| Metrics | `ServiceConf.DevServer` | Prometheus scrapes :6060/metrics (UI 9099) |
| Logging | `ServiceConf.Log` (logx)| JSON files → Promtail → Loki (:3100)       |

Only the **provider** is instrumented; the consumer is a raw zrpc client.
zrpc's prometheus interceptor uses the **`rpc_server_requests_*`** metric
family (not the `http_server_requests_*` family that `rest.Server` exposes in
the sibling `greet-api`). go-zero's logx tags each log line with the active
trace/span, so logs in Loki correlate with spans in Jaeger.

Bring up etcd + the backends and run the instrumented smoke test:

```bash
docker compose up -d
bash scripts/smoke-test.sh   # asserts /metrics serves rpc_server_requests_*
```

Manual verification while the provider is running and after a request:

- **Metrics**: Prometheus UI at http://127.0.0.1:9099 — query `rpc_server_requests_duration_ms_count`.
- **Traces**: Jaeger UI at http://127.0.0.1:16686 — pick service `greet-rpc`.
- **Logs**: `curl -s 'http://127.0.0.1:3100/loki/api/v1/query_range?query=%7Bjob%3D%22greet-rpc%22%7D'`.

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
bash scripts/smoke-test.sh
```
