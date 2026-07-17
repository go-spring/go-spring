# dubbo-go — REST (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` example that speaks the **REST** protocol — HTTP/1.1
transport with per-method (verb, path, param-source) routing served by
go-restful — wired the Go-Spring way via the reusable **starter-dubbo**
module: it supplies the `gs.Server` adapter, `gs.Run()` drives the
lifecycle, the provider is just a `ServiceRegister` bean, and the protocol
and registry come from `conf/app.properties` instead of hard-coded `main()`
wiring.

Unlike the Triple sibling in [`../triple`](../triple), REST has no
protobuf IDL and no code generator; unlike the classic-Dubbo
([`../dubbo`](../dubbo)) and JSON-RPC ([`../jsonrpc`](../jsonrpc)) siblings,
however, REST cannot be driven by method-reflection alone: dubbo-go needs a
`RestServiceConfig` map that pins every Go method to a concrete `(HTTP verb,
URL path, param source)` tuple before Serve is called. That map is
installed by `provider/handler.go` on the server side and by
`consumer/main.go` on the client side — both must agree, and both must be
in place before the process registers/dials.

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
  │ → rest://<host>:20003                    │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀──── REST (HTTP/1) ────│  consumer  │
│ gs.Run()   │  GET /greet?name=...   │ one-shot   │
│ :20003     │──────────────────────▶│ assert+exit│
└────────────┘   echo name (JSON)     └────────────┘
```

## Layout

```
contrib/dubbo-go/rest/
├── idl/greet.go             # the "IDL": interface name, method name, HTTP verb+path+query constants
├── idl/gen-code.sh          # no-op — REST has no IDL codegen
├── provider/handler.go      # GreetProvider (Go struct) + RestServiceConfig + StarterDubbo.ServiceRegister bean (server comes from starter-dubbo)
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── consumer/main.go         # RestServiceConfig registration + discovers, calls, asserts, exits (raw dubbo-go client, not gs.Run)
├── conf/app.properties      # provider-only configuration (server role + registry + observability, metrics :9090)
├── docker/                  # Prometheus & Promtail config for the backend stack
├── docker-compose.yml       # local etcd + Prometheus + Jaeger + Loki + Promtail
└── scripts/smoke-test.sh    # smoke test: bring up backends+provider, run consumer, tear down
```

## How it was generated

Nothing was generated. REST has no protobuf/thrift IDL and no code generator
in dubbo-go v3 — the service surface is a hand-written Go file
(`idl/greet.go`) that pins the Java-style interface name, method name, and
the HTTP verb / path / query-key constants, plus a hand-written provider
struct with the matching method signature and hand-written
`RestServiceConfig` maps on both sides. Running `./idl/gen-code.sh` prints a one-line
"nothing to do" for symmetry with the Triple sibling.

## Choosing this protocol vs. the siblings

| Concern              | Triple (`../triple`)                | Dubbo/Hessian2 (`../dubbo`)               | JSON-RPC (`../jsonrpc`)                                 | REST (this module)                                      |
| -------------------- | ----------------------------------- | ----------------------------------------- | ------------------------------------------------------- | ------------------------------------------------------- |
| Transport            | HTTP/2                              | Raw TCP                                   | HTTP/1.1                                                | HTTP/1.1                                                |
| Payload              | protobuf                            | Hessian2                                  | JSON-RPC 2.0 envelope                                   | Plain JSON, no envelope                                 |
| URL layout           | fixed by protocol                   | fixed by protocol                         | fixed by protocol (`POST /<interface>`)                 | user-defined per method (verb + path + param source)    |
| IDL                  | `.proto` + `protoc-gen-go-triple`   | none — hand-written Go structs            | none — hand-written Go structs                          | none — Go structs + hand-written RestServiceConfig maps |
| Client-side wiring   | Typed stub                          | Interface name only                       | Interface name only                                     | Interface name + method-mapping map                     |
| Cross-language reach | Any gRPC/Triple client              | Java Dubbo (native), Hessian2 runtimes    | Anything speaking HTTP + JSON                           | Anything speaking HTTP (curl, browsers, gateways, ...)  |
| When to pick         | Greenfield Go microservices         | Interop with existing Java Dubbo services | Bare-HTTP clients / lowest common denominator           | REST-style public APIs, gateway-friendly endpoints      |

## Configuration

```properties
# Disable the built-in HTTP server; the provider exposes only the REST endpoint.
spring.http.server.enabled=false

# REST bind port; the key under ${spring.dubbo.server.protocols} is the
# dubbo-go protocol name. REST on 20003 (20000/20001/20002 are reserved for
# the Triple/Dubbo/JSON-RPC siblings so all four can coexist on one host).
spring.dubbo.server.protocols.rest.port=20003

# etcd registry, defined once under ${spring.dubbo.registries}: the map key is
# a logical registry ID (type defaults to the key). Roles reference it by ID via
# ${...registry-ids}; with one registry, neither sets it. Matches docker-compose.yml.
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379
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
bash scripts/smoke-test.sh
```

## Observability

starter-dubbo has **metrics and tracing built in and on by default**; this
example turns them on explicitly in `conf/app.properties` and wires a full local
backend stack in `docker-compose.yml` so all three signals — metrics, traces,
logs — are visible end to end. None of this needs any code; it is all
configuration layered on the shared `Instance` bean.

> **Provider-only.** Unlike the Triple/Dubbo/JSON-RPC siblings, starter-dubbo's
> client bean has no REST protocol support, so the consumer here is a standalone
> raw dubbo-go `client.NewClient` main() that never touches Go-Spring wiring.
> It is not instrumented: there is no consumer `conf/`, no consumer metrics
> port, no consumer OTel exporter, no consumer log file. Everything below is
> about the **provider**. The single `conf/app.properties` lives at the module
> root and is loaded only by the provider (its `init()` chdirs to the module
> root before `gs.Run()`).

| Signal  | Produced by                                    | Backend         | Where to look                                        |
| ------- | ---------------------------------------------- | --------------- | ---------------------------------------------------- |
| Metrics | dubbo-go Prometheus exporter, port `:9090`     | Prometheus      | UI http://127.0.0.1:9099 (query `up`, `dubbo_*`)     |
| Traces  | dubbo-go OTel → OTLP/gRPC `127.0.0.1:4317`     | Jaeger          | UI http://127.0.0.1:16686 (service `rest-demo`)      |
| Logs    | go-spring `log` → `logs/provider.log`          | Loki (Promtail) | Loki HTTP API, port `:3100` (query below)            |

### Architecture & how it works

The provider (and the raw consumer) run **on the host** (scripts/smoke-test.sh
builds and runs the provider); every backend runs **in a container**
(docker-compose.yml). The host↔container boundary is why each signal takes a
slightly different path:

```
        HOST (provider + raw consumer)          DOCKER (docker-compose.yml)
  ┌────────────────────────────────┐        ┌──────────────────────────────┐
  │ provider                       │  reg/  │ etcd            :2379         │
  │   REST (HTTP/1)    :20003      │◀─disc─▶│   service registry            │
  │   /metrics (HTTP)  :9090       │        │                               │
  │   OTel SDK ─┐                  │        │ Prometheus      :9099 (UI)    │
  │   log file  │                  │        │ Jaeger    :4317 / :16686 (UI) │
  │ consumer (raw, not instrumented)        │ Loki            :3100         │
  │   one-shot call, then exits    │        │ Promtail (tails /var/log/app) │
  └─────────────┼──────────────────┘        └──────────────────────────────┘
                │
   (1) METRICS — pull:   Prometheus ──GET /metrics every 5s──▶ provider :9090
                         (reaches the host via host.docker.internal)
   (2) TRACES  — push:   OTel SDK ──OTLP/gRPC spans──▶ Jaeger :4317 ─▶ :16686 UI
   (3) LOGS    — tail+push: provider ─write▶ logs/provider.log ◀─bind-mount─ Promtail
                         Promtail ──HTTP push──▶ Loki :3100 ─▶ query API
```

**(1) Metrics — a pull model.** starter-dubbo's built-in Prometheus registry
(enabled by `spring.dubbo.metrics.*`) stands up a plain HTTP endpoint that
renders the current counter/gauge values on demand — it pushes nothing. The
provider serves it on `:9090`. Prometheus is the active party: on its
`scrape_interval` (5s here) it issues `GET /metrics`, parses the text-format
response, and stores each sample with a timestamp. Because Prometheus is in a
container and the target is on the host, `docker/prometheus.yml` targets
`host.docker.internal:9090` (mapped via `extra_hosts` on Linux, native on
macOS/Windows). Metrics are registered lazily, so `dubbo_*` rows only appear
*after* the first RPC — that is why the smoke test calls before asserting.

**(2) Traces — a push model.** The dubbo-go OTel integration (enabled by
`spring.dubbo.tracing.*`) wraps each RPC in a span on the provider side. An
in-process batch span processor buffers spans and exports them over
**OTLP/gRPC** to the endpoint in config (`127.0.0.1:4317`), which is Jaeger's
mapped collector port. Here the *application* is the active party — it pushes
to the collector; Jaeger stores the spans and serves them at the `:16686` UI
under service `rest-demo`. `mode=always`/`ratio=1.0` samples every span, so
even a single call shows up. (The raw consumer contributes no client-side
spans.)

**(3) Logs — tail then push.** No log ever travels over the network from the
application. go-spring's `log` module (a `FileLogger` with a `JSONLayout`)
writes structured JSON lines to `logs/provider.log` on the host — because the
provider chdirs into the module root before `gs.Run()`, this relative `logs/`
resolves to `contrib/dubbo-go/rest/logs/`. That `logs/` directory is
**bind-mounted read-only** into the Promtail container at `/var/log/app`
(docker-compose.yml). Promtail (`docker/promtail-config.yml`) tails `*.log`,
tracks its read offset in a positions file, tags each line with
`job="rest-demo"` plus the source `filename`, and **pushes** batches to Loki's
`:3100` HTTP API.

Everything above is **configuration only** — no application code touches
Prometheus, OTel, or Loki directly. It all layers onto the single shared
`Instance` bean that starter-dubbo builds from `spring.dubbo.*`.

```properties
# metrics (Prometheus) — served independently of the disabled HTTP server
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.metrics.path=/metrics

# tracing (OTel → Jaeger over OTLP/gRPC); mode=always so even a single call is sampled
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317
spring.dubbo.tracing.insecure=true

# logging — structured JSON to logs/provider.log, collected by Promtail into Loki
logging.logger.root.type=FileLogger
logging.logger.root.layout.type=JSONLayout
logging.logger.root.dir=logs
logging.logger.root.file=provider.log
```

### Manual verification (step by step)

`scripts/smoke-test.sh` asserts only **endpoint liveness** (the provider serves
`dubbo_*` metrics, the RPC round-trips, no backend container crashed); it does
**not** wait for data to actually land in Prometheus/Jaeger/Loki. The steps
below let you confirm each signal by hand. Run them **while the stack is up**
and **after at least one RPC has been made**.

> All `curl`/`open` targets are on `127.0.0.1`. If your shell has an HTTP proxy
> exported, add `127.0.0.1,localhost` to `no_proxy`/`NO_PROXY` first, otherwise
> the local requests get routed through the proxy and return nothing.

**Step 0 — bring everything up and make the call.**

```bash
docker compose up -d          # or docker-compose up -d
go run ./provider &           # Terminal A: long-lived
go run ./consumer             # Terminal B: makes one Greet call and exits
```

Expected:

```
Response from discovered provider: Hello, Dubbo-Go!
```

**Step 1 — the provider exposes `dubbo_*` metrics.** Metrics are registered
lazily, so this only returns rows *after* the call in step 0.

```bash
curl -s http://127.0.0.1:9090/metrics | grep '^dubbo_provider_requests_total{'
```

**Step 2 — Prometheus scraped the provider.** UI is on `:9099` (its container
port `9090` is remapped to avoid clashing with the provider's `:9090`).

```bash
# the scrape target is healthy (value "1" = up)
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=up{job="rest-provider"}'

# the dubbo metric made it into Prometheus
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=dubbo_provider_requests_total'
```

Or open the UI and query `up` / `dubbo_*`:

```bash
open http://127.0.0.1:9099
```

**Step 3 — the trace reached Jaeger.**

```bash
curl -s 'http://127.0.0.1:16686/api/services'
curl -s 'http://127.0.0.1:16686/api/traces?service=rest-demo&limit=10'
```

Or open the Jaeger UI, pick service `rest-demo`, and click *Find Traces*:

```bash
open http://127.0.0.1:16686
```

**Step 4 — the logs reached Loki (via Promtail).**

```bash
# Promtail is shipping provider.log
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/label/filename/values'

# query the actual JSON log lines from the last hour
END=$(date +%s)000000000; START=$(($(date +%s)-3600))000000000
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={job="rest-demo"}' \
  --data-urlencode "start=$START" --data-urlencode "end=$END" \
  --data-urlencode 'limit=5'
```

Expected — the filename list contains `/var/log/app/provider.log`, and the
query returns `"status":"success"` with one or more streams of JSON log lines.

**Step 5 — the log file exists on disk.**

```bash
ls logs/
head -1 logs/provider.log
```

Expected — one file (`provider.log`; no `consumer.log` because the raw consumer
is not instrumented) and structured JSON lines.

**Step 6 — no backend crashed.**

```bash
docker compose ps        # or docker-compose ps
```

Expected — all five containers `Up`:

```
contrib-dubbo-go-rest-etcd         Up
contrib-dubbo-go-rest-jaeger       Up
contrib-dubbo-go-rest-loki         Up
contrib-dubbo-go-rest-prometheus   Up
contrib-dubbo-go-rest-promtail     Up
```
