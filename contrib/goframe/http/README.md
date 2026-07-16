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

This is a runnable example, **not** a reusable starter module.

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
├── provider/main.go              # gs.Run(); long-lived, registers into etcd
├── provider/server.go            # GoFrameServer adapter (gs.Server) + Config + etcd registry + observability wiring
├── provider/handler.go           # hand-written HelloController (g.Meta route + response), logs through the request ctx
├── consumer/main.go              # discovers the provider via etcd, calls it and asserts, then exits
├── conf/app.properties           # provider configuration
├── scripts/gen-code.sh           # no-op: the handler is hand-written, no IDL codegen
├── docker-compose.yml            # local etcd + observability backends (Prometheus/Jaeger/Loki/Promtail)
├── docker/prometheus.yml         # Prometheus scrape config (targets the host's :8000/metrics)
├── docker/promtail-config.yml    # Promtail config (tails ./logs, pushes to Loki)
└── scripts/smoke-test.sh         # smoke test: bring up backends+provider, run consumer, assert three pillars, tear down
```

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
| Startup         | `cmd.Main.Run()` → `s.Run()` blocks in `main()`     | `GoFrameServer` implements `gs.Server`; `gs.Run()` drives Run/Stop             |
| Server creation | `g.Server()` inline in `internal/cmd`               | `provider.NewGoFrameServer`, a `gs.Server` bean                                |
| Route wiring    | `s.Group(...)` inline in `internal/cmd`             | done inside the server bean constructor                                        |
| Config source   | `manifest/config/config.yaml` via `g.Cfg()`         | `conf/app.properties` bound via `value:"${...}"` tags under `${goframe}`       |
| Registration    | none (direct)                                       | provider calls `gsvc.SetRegistry(etcd.New(addr))` before `g.Server(name)`      |
| Discovery       | consumer hard-coded `http://localhost:8000/hello`   | consumer `g.Client().Discovery(etcd.New(addr)).Get(ctx, "http://<name>/hello")` |
| Shutdown        | `s.Run()`'s own signal handling                     | graceful shutdown by Go-Spring (SIGTERM → `Stop()`, deregisters from etcd)     |

The adapter in `provider/server.go` is the crux. `ghttp.Server`
snapshots `gsvc.GetRegistry()` at construction time (see `ghttp_server.go`
`registrar: gsvc.GetRegistry()`), so the constructor sets the etcd registry
*before* calling `g.Server(name)`. `s.Start()` is non-blocking, so `Run` parks
on a done channel that `Stop()` closes to hand control back to Go-Spring's
shutdown, which in turn triggers `s.Shutdown()` and the etcd deregister.

The consumer never learns the provider's host:port: it passes the same etcd
address plus the service name (`goframe.hello`, matching `goframe.name` in
`conf/app.properties`) to `gclient`, whose internal `Discovery` middleware
treats `r.URL.Host` as a service name and resolves it against etcd.

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
goframe.address=:8000

# Service name the provider registers under; the consumer resolves this same
# name from etcd.
goframe.name=goframe.hello

# etcd registry address; matches docker-compose.yml.
goframe.registry.etcd=127.0.0.1:2379

# Observability (see the section below). Tracing → Jaeger over OTLP/HTTP,
# metrics → Prometheus scrape on the HTTP port, logging → glog JSON to logs/.
goframe.tracing.endpoint=127.0.0.1:4318
goframe.tracing.path=/v1/traces
goframe.metrics.path=/metrics
goframe.log.dir=../logs
goframe.log.file=provider.log
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

Unlike the dubbo-go example (where observability is built into `starter-dubbo`
and driven by `spring.dubbo.*`), goframe ships **its own OpenTelemetry
integration**, so all three pillars use goframe's native packages — wired in
`provider/server.go`, not go-spring's `log`/`metric`. Only the **provider** is
instrumented; the consumer stays a bare client, matching the dubbo-go / go-zero
examples. The backend stack (Prometheus/Jaeger/Loki/Promtail) is the same shared
one the other contrib examples use, defined in `docker-compose.yml`.

| Signal  | Produced by                                            | Backend         | Where to look                                        |
| ------- | ------------------------------------------------------ | --------------- | ---------------------------------------------------- |
| Metrics | `contrib/metric/otelmetric` → Prometheus exporter      | Prometheus      | UI http://127.0.0.1:9099 (query `target_info`, `otel_scope_info`) |
| Traces  | `contrib/trace/otlphttp` → OTLP/HTTP `127.0.0.1:4318` | Jaeger          | UI http://127.0.0.1:16686 (service `goframe.hello`)  |
| Logs    | `glog` JSON handler → files under `logs/`              | Loki (Promtail) | Loki HTTP API, port `:3100`                          |

### How it works

The provider runs **on the host** (`scripts/smoke-test.sh` builds and runs it);
every backend runs **in a container** (`docker-compose.yml`). Each pillar is set
up once, at server construction, in `NewGoFrameServer` → `initObservability`:

- **Tracing — push.** `otlphttp.Init(name, endpoint, "/v1/traces")` sets the
  global tracer provider and OTLP/HTTP exporter, then returns a shutdown func
  (flushed in `Stop()`). Once the provider is set, **ghttp auto-instruments every
  request** — no middleware needed. OTLP/HTTP (`:4318`) is used because
  `otlphttp` hardcodes `WithInsecure()`, so it talks to the plaintext Jaeger
  all-in-one collector cleanly (the same reason the dubbo-go example avoids
  OTLP/gRPC).
- **Metrics — pull.** A Prometheus (pull) exporter feeds an `otelmetric`
  `MeterProvider` (`WithBuiltInMetrics()` adds Go runtime metrics). The endpoint
  is served by `otelmetric.PrometheusHandler`, bound at the server **root**
  (`s.BindHandler("/metrics", ...)`) rather than inside the `Group("/")` — the
  group's `MiddlewareHandlerResponse` wraps responses in goframe's JSON envelope,
  which would corrupt the Prometheus exposition format. goframe serves `/metrics`
  on the **same HTTP port** (`:8000`); Prometheus scrapes
  `host.docker.internal:8000` (`docker/prometheus.yml`).
- **Logging — tail then push.** `glog`'s built-in JSON handler
  (`glog.HandlerJson`) writes one structured line per event to
  `../logs/provider.log`. glog **auto-injects the request's trace-id** into each
  line logged with the request ctx (see `provider/handler.go`), so logs correlate
  with the spans above. That `logs/` dir is bind-mounted into Promtail, which
  tails it and pushes to Loki `:3100`.

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

