# goframe — HTTP (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [GoFrame](https://goframe.org) `Hello` service, generated with `gf init` and
then refactored to boot and be configured the Go-Spring way: `gs.Run()` drives
the lifecycle, the goframe `*ghttp.Server` is an IoC bean, and the bind address
comes from `conf/app.properties` instead of `manifest/config/config.yaml`.

On top of that lift it wires in an **etcd registry** for real **service
registration & discovery**: on startup the provider registers `goframe.hello`
into etcd; the consumer never learns the provider's host:port and instead
resolves a live address from the same etcd through goframe's `gclient`
discovery middleware. This is the microservice governance goframe advertises
via its `gsvc` layer — not the earlier direct-connection example.

This is the **HTTP** protocol variant. GoFrame's gRPC support lives in a
different server type (`grpcx.GrpcServer` vs `*ghttp.Server`) with a different
codegen chain (`protoc` vs `gf gen ctrl`), so the two protocols are split into
sibling modules. For the gRPC variant, see [`../grpc`](../grpc).

The server lifecycle, log bridge and optional metrics are **not** hand-rolled
here anymore: they live in the reusable
[`starter-goframe/http`](../../../starter/starter-goframe) module. This example
just imports that starter and supplies a `ServiceRegister` bean; tracing is
deferred to [`starter-otel`](../../../starter/starter-otel).

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ goframe.hello                            │ resolve provider addr
  │ → http://<host>:8000                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── HTTP ────────│  consumer  │
│ gs.Run()   │      GET /hello        │ one-shot   │
│ :8000      │──────────────────────▶│ assert+exit│
└────────────┘     "Hello World!"     └────────────┘
```

## Layout

```
contrib/goframe/http/
├── provider/main.go              # gs.Run() + blank-import starter-otel; long-lived, registers into etcd
├── provider/handler.go           # imports starter-goframe/http, provides its ServiceRegister; hand-written HelloController
├── consumer/main.go              # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties           # provider configuration (${spring.goframe.http.server} + ${spring.observability})
├── scripts/gen-code.sh           # no-op: the handler is hand-written, no IDL codegen
├── docker-compose.yml            # local etcd + observability backends (Prometheus/Jaeger/Loki/Promtail)
├── docker/prometheus.yml         # Prometheus scrape config (targets the host's :8000/metrics)
├── docker/promtail-config.yml    # Promtail config (tails ./logs, pushes to Loki)
└── scripts/smoke-test.sh         # smoke test: bring up backends+provider, run consumer, assert three pillars, tear down
```

The `GoFrameServer` adapter (`gs.Server` + `Config` + etcd registry + metrics)
and the glog bridge that used to sit in `provider/server.go` and
`provider/logbridge.go` now live in `starter-goframe/http`, so those two files
are gone from this example.

## How it was generated

```bash
# tool (once)
go install github.com/gogf/gf/cmd/gf/v2@latest

# scaffold the single-repo template (module renamed to go-spring.org/goframe/http
# when it was split from its gRPC sibling under ../grpc).
gf init goframe -g go-spring.org/goframe/http
```

The original `gf init` scaffold (the `api/`, `internal/`, `manifest/`,
`resource/` trees plus `gf gen ctrl` controllers) has been **dropped** in favour
of the flat `provider/{main,server,handler}.go` layout the sibling protocol
examples use. The Hello handler is now hand-written in `provider/handler.go`;
only *how the service is configured, started and discovered* is the Go-Spring
part.

## The refactor: native goframe → Go-Spring + registry

| Concern         | goframe scaffold                                    | Go-Spring version                                                              |
| --------------- | --------------------------------------------------- | ------------------------------------------------------------------------------ |
| Startup         | `cmd.Main.Run()` → `s.Run()` blocks in `main()`     | starter-goframe/http's server implements `gs.Server`; `gs.Run()` drives Run/Stop |
| Server creation | `g.Server()` inline in `internal/cmd`               | `starter-goframe/http`'s `NewHTTPServer`, a `gs.Server` bean                    |
| Route wiring    | `s.Group(...)` inline in `internal/cmd`             | the app's `ServiceRegister` bean, bound inside the starter's server constructor |
| Config source   | `manifest/config/config.yaml` via `g.Cfg()`         | `conf/app.properties` bound via `value:"${...}"` tags under `${spring.goframe.http.server}` |
| Registration    | none (direct)                                       | starter calls `gsvc.SetRegistry(etcd.New(addr))` before `g.Server(name)` when `registry.etcd` is set |
| Discovery       | consumer hard-coded `http://localhost:8000/hello`   | consumer `g.Client().Discovery(etcd.New(addr)).Get(ctx, "http://<name>/hello")` |
| Shutdown        | `s.Run()`'s own signal handling                     | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, deregisters from etcd)     |

The adapter now lives in `starter-goframe/http`, not this example.
`ghttp.Server` snapshots `gsvc.GetRegistry()` at construction time (see
`ghttp_server.go` `registrar: gsvc.GetRegistry()`), so the starter sets the etcd
registry *before* calling `g.Server(name)`. `s.Start()` is non-blocking, so `Run`
parks on a done channel that `Stop()` closes to hand control back to Go-Spring's
shutdown, which in turn triggers `s.Shutdown()` and the etcd deregister.

The consumer never learns the provider's host:port: it passes the same etcd
address plus the service name (`goframe.hello`, matching
`spring.goframe.http.server.name` in `conf/app.properties`) to `gclient`, whose
internal `Discovery` middleware treats `r.URL.Host` as a service name and
resolves it against etcd.

## Choosing a registry

This example standardises on **etcd** for easy cross-comparison with the other
contrib examples. `github.com/gogf/gf/contrib/registry/*` also ships
**Nacos**, **ZooKeeper** and **Polaris** adapters that satisfy the same
`gsvc.Registry` interface: swap
`github.com/gogf/gf/contrib/registry/etcd/v2` for
`.../registry/nacos/v2` / `.../registry/zookeeper/v2` / `.../registry/polaris/v2`
and update `goframe.registry.etcd` accordingly. With Nacos you can also inspect
the registered services directly in its built-in `:8848/nacos` console.

## Configuration

```properties
# Disable Go-Spring's built-in HTTP server; the goframe *ghttp.Server owns the port.
spring.http.server.enabled=false

# HTTP bind address for the goframe *ghttp.Server.
spring.goframe.http.server.address=:8000

# Service name the provider registers under; the consumer resolves this same
# name from etcd.
spring.goframe.http.server.name=goframe.hello

# etcd registry address; matches docker-compose.yml. Leave empty for a plain
# server clients dial directly.
spring.goframe.http.server.registry.etcd=127.0.0.1:2379

# Metrics: the starter serves goframe's native Prometheus (pull) endpoint on the
# HTTP port. Tracing is deferred to starter-otel (see the section below).
spring.goframe.http.server.metrics.enabled=true
spring.goframe.http.server.metrics.path=/metrics

# starter-otel: install the global OTel TracerProvider; ghttp auto-instruments
# off it. Metrics are goframe-native above, so starter-otel's metrics stay off.
spring.observability.service-name=goframe.hello
spring.observability.trace.exporter=otlp-http
spring.observability.trace.endpoint=127.0.0.1:4318
spring.observability.metrics.exporter=none
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
Response from discovered provider: Hello World!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash scripts/smoke-test.sh
```

## Observability

Observability is split between two starters: **tracing** rides
[`starter-otel`](../../../starter/starter-otel) (which installs the global OTel
`TracerProvider` that goframe's `ghttp` auto-instruments off), while **metrics**
stay goframe-native, served by `starter-goframe/http` when
`spring.goframe.http.server.metrics.enabled=true`. **Logging** flows through the
starter's glog→go-spring bridge into the single `log` pipeline. Only the
**provider** is instrumented; the consumer stays a bare client, matching the
dubbo-go / go-zero examples. The backend stack (Prometheus/Jaeger/Loki/Promtail)
is the same shared one the other contrib examples use, defined in
`docker-compose.yml`.

| Signal  | Produced by                                            | Backend         | Where to look                                        |
| ------- | ------------------------------------------------------ | --------------- | ---------------------------------------------------- |
| Metrics | `starter-goframe/http` → goframe-native Prometheus exporter | Prometheus | UI http://127.0.0.1:9099 (query `target_info`, `otel_scope_info`) |
| Traces  | `starter-otel` global `TracerProvider` → OTLP/HTTP `127.0.0.1:4318` | Jaeger | UI http://127.0.0.1:16686 (service `goframe.hello`)  |
| Logs    | glog → `starter-goframe` bridge → go-spring `log` FileLogger under `logs/` | Loki (Promtail) | Loki HTTP API, port `:3100`             |

### How it works

The provider runs **on the host** (`scripts/smoke-test.sh` builds and runs it);
every backend runs **in a container** (`docker-compose.yml`).

- **Tracing — push, via starter-otel.** Importing `starter-otel` (blank import in
  `provider/main.go`) installs the global OTel `TracerProvider` from
  `${spring.observability.trace}` — here `otlp-http` to Jaeger `:4318`. Once that
  global is set, **ghttp auto-instruments every request** off it — no per-server
  wiring, replacing the inline `contrib/trace/otlphttp` block the old
  `provider/server.go` carried.
- **Metrics — pull, goframe-native.** `starter-goframe/http` feeds a Prometheus
  (pull) exporter into a goframe `otelmetric` `MeterProvider` and binds
  `otelmetric.PrometheusHandler` at the server **root** (outside the
  response-wrapping group, so the exposition stays valid Prometheus text). goframe
  serves `/metrics` on the **same HTTP port** (`:8000`); Prometheus scrapes
  `host.docker.internal:8000` (`docker/prometheus.yml`). This is a separate
  pipeline from starter-otel's metrics, so `spring.observability.metrics.exporter`
  is left `none` to avoid a second `MeterProvider`.
- **Logging — bridged, then tailed.** The starter routes goframe's `glog` into
  go-spring's `log` module; the root `FileLogger` (JSONLayout, `conf/app.properties`)
  writes one structured line per event to `../logs/provider.log`. That `logs/` dir
  is bind-mounted into Promtail, which tails it and pushes to Loki `:3100`.

Everything is provider-local and deterministic up to the backends; whether the
data is then queryable in Prometheus/Jaeger/Loki is a manual step. With the stack
up and after at least one call:

```bash
# Metrics — the exporter always emits target_info once wired
curl -s http://127.0.0.1:8000/metrics | grep target_info

# Traces — the service appears in Jaeger after the first request
open http://127.0.0.1:16686        # service: goframe.hello

# Logs — each ctx-scoped line carries a non-empty "TraceId"
grep '"TraceId"' logs/provider.log
```

> If your shell exports an HTTP proxy, add `127.0.0.1,localhost` to
> `no_proxy`/`NO_PROXY` first, otherwise the local `curl`s get routed through the
> proxy and return nothing.

