# kitex (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` example that
starts from the scaffold `kitex` generates and is then refactored to boot and
be configured the Go-Spring way: `gs.Run()` drives the lifecycle, the handler
is an IoC bean, and the bind address comes from `conf/app.properties` instead
of hard-coded `main()` wiring.

It uses Kitex's default TTHeader/Thrift transport and wires in an **etcd
registry** (via `github.com/kitex-contrib/registry-etcd`) for real **service
registration & discovery**: on startup the provider registers the `echo`
service into etcd; the consumer never learns the provider's host:port and
instead resolves a live address from the same etcd. This is what a real Kitex
microservice looks like — not the earlier registry-less direct connection.

This is a runnable example, **not** a reusable starter module.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ service: echo                            │ resolve provider addr
  │ → <host>:8888                            │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      Echo(message)     │ one-shot   │
│ :8888      │──────────────────────▶│ assert+exit│
└────────────┘       echo message     └────────────┘
```

## Layout

```
contrib/kitex/thrift/
├── idl/echo.thrift          # Thrift IDL
├── idl/echo/...          # Kitex-generated code (DO NOT EDIT)
├── kitex_info.yaml          # metadata for re-generation
├── scripts/gen-code.sh      # regenerates idl/echo/ from the IDL
├── provider/handler.go      # EchoServiceImpl, exported as an echo.EchoService bean
├── provider/server.go       # KitexServer adapter (gs.Server) + Config, configures the etcd registry
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties      # provider configuration
├── docker-compose.yml       # local etcd
└── scripts/smoke-test.sh    # smoke test: bring up etcd+provider, run consumer, tear down
```

## How it was generated

```bash
# tools (once)
go install github.com/cloudwego/thriftgo@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest

# scaffold from the IDL (or just run ./scripts/gen-code.sh)
kitex -module go-spring.org/kitex/thrift -service echo idl/echo.thrift
```

The scaffold produces `idl/echo/`, a bare `handler.go`, and a `main.go` that
calls `svr.Run()` directly. `idl/echo/` is shared by both the provider and
the consumer. Re-running `./scripts/gen-code.sh` regenerates `idl/echo/` without touching
the refactored provider/consumer code.

> This is the **Thrift** protocol variant. For the protobuf-based transports
> (KitexProtobuf and gRPC), see the sibling [`../protobuf`](../protobuf) example.

## The refactor: native Kitex → Go-Spring + registry

| Concern         | Kitex scaffold                          | Go-Spring version                                                                    |
| --------------- | --------------------------------------- | ------------------------------------------------------------------------------------ |
| Startup         | `svr.Run()` blocks in `main()`          | `KitexServer` implements `gs.Server`; `gs.Run()` drives Run/Stop                     |
| Handler wiring  | `new(EchoServiceImpl)` passed manually  | `gs.Provide(&EchoServiceImpl{}).Export(gs.As[echo.EchoService]())`                   |
| Server enable   | always on                               | conditional on an `echo.EchoService` bean via `gs.OnBean`                            |
| Address         | hard-coded default                      | `${spring.kitex.server.addr}` from `conf/app.properties`                             |
| Registration    | none (direct)                           | provider `server.WithRegistry(etcd.NewEtcdRegistry(...))` + `WithServerBasicInfo`    |
| Discovery       | consumer `client.WithHostPorts(":8888")`| consumer `client.WithResolver(etcd.NewEtcdResolver(...))`, resolves by service name  |
| Shutdown        | process-owned                           | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, deregisters from etcd)           |

The adapter in `provider/server.go` is the crux: Kitex's `server.Run()` binds
the listener, registers the provider into etcd, and then blocks forever, so it
runs in a goroutine started only after `sig.TriggerAndWait()`, while `Run`
parks on a done channel that `Stop()` closes to hand control back to
Go-Spring's shutdown.

The consumer only supplies the etcd address, never the provider's: it passes
the service name (`echo`) that the provider registered under, and Kitex uses
it to find a live provider in etcd and call it.

## Choosing a registry

This example standardizes on **etcd** for easy cross-comparison with the other
contrib examples. The [kitex-contrib](https://github.com/kitex-contrib) org
also ships adapters for **Nacos**, **Consul**, **ZooKeeper**, and **Polaris**:
swap `registry-etcd` for the corresponding `registry-nacos` /
`registry-consul` / `registry-zookeeper` / `registry-polaris` module and use
its `NewXxxRegistry` / `NewXxxResolver` in place of `etcd.NewEtcdRegistry` /
`etcd.NewEtcdResolver`. With Nacos you can also inspect the registered
services directly in its built-in `:8848/nacos` console.

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only Kitex.
spring.http.server.enabled=false

# Kitex bind address; read via the ${spring.kitex.server} prefix, default :8888.
spring.kitex.server.addr=:8888

# Service name registered into etcd; consumer resolves by the same name.
spring.kitex.server.service.name=echo

# etcd registry address; matches docker-compose.yml.
spring.kitex.server.registry.etcd=127.0.0.1:2379
```

## Observability (log / trace / metric)

The provider is instrumented for the three pillars, all wired inside
`starter-kitex` and driven purely from `provider/conf/app.properties` — the
handler only adds a context-aware `klog.CtxInfof` line. Kitex has no single
"SetUp" like dubbo-go/go-zero, so `starter-kitex` composes its native
[kitex-contrib](https://github.com/kitex-contrib) pieces:

| Pillar | Mechanism | Backend |
| ------ | --------- | ------- |
| Trace  | `obs-opentelemetry` `tracing.NewServerSuite()` → OTLP/gRPC | Jaeger (`:16686`, collector `:4317`) |
| Metric | `monitor-prometheus` server tracer, self-hosted scrape endpoint | Prometheus (`:9099`, scrapes provider `:9090`) |
| Log    | `klog` backed by the `obs-opentelemetry` logrus adapter (JSON + `trace_id`/`span_id`) | file → Promtail → Loki (`:3100`) |

Only the **provider** is instrumented; the consumer stays a bare client. The
OTel meter is disabled so metrics flow solely through Prometheus (no duplicate
pipeline).

`docker-compose.yml` brings up etcd plus Jaeger, Prometheus, Loki and Promtail.
After running the provider + consumer (or the smoke test), verify each pillar:

- **Trace** — Jaeger UI <http://127.0.0.1:16686>, service `echo`, look for the `Echo` span.
- **Metric** — Prometheus UI <http://127.0.0.1:9099> (query e.g. `kitex_server_throughput`), or `curl 127.0.0.1:9090/metrics`.
- **Log** — `logs/provider.log` holds JSON lines carrying `trace_id`/`span_id`; query them via Loki at `127.0.0.1:3100`.

## Run

Bring up the registry and observability backends first:

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
Response from discovered provider: Hello, Kitex!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash scripts/smoke-test.sh
```
