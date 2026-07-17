# dubbo-go — JSON-RPC (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` example that speaks the **JSON-RPC 2.0** protocol — HTTP/1.1
transport with a **JSON** body — wired the Go-Spring way via the reusable
**starter-dubbo** module: it supplies the `gs.Server` adapter, `gs.Run()`
drives the lifecycle, the provider is just a `ServiceRegister` bean, and the
protocol and registry come from `conf/app.properties` instead of hard-coded
`main()` wiring.

Unlike the Triple sibling in [`../triple`](../triple), this protocol has no
protobuf IDL and no code generator in dubbo-go v3: services are plain Go
structs whose exported method signatures are reflected over at registration
time and marshalled with `encoding/json` on the wire. That makes JSON-RPC
the interop path of last resort — anything that can speak HTTP and JSON
(curl, browsers, non-Go languages without a Dubbo SDK) can hit the provider
directly without a client library.

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
  │ → jsonrpc://<host>:20002                 │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀── JSON-RPC (HTTP/1) ──│  consumer  │
│ gs.Run()   │      Greet(name)       │ one-shot   │
│ :20002     │──────────────────────▶│ assert+exit│
└────────────┘       echo name        └────────────┘
```

## Layout

```
contrib/dubbo-go/jsonrpc/
├── idl/greet.go             # the "IDL": interface name + method-name constants
├── idl/gen-code.sh          # no-op — JSON-RPC has no IDL codegen
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean (server comes from starter-dubbo)
├── provider/main.go         # gs.Run(); long-lived, registers into etcd
├── provider/conf/app.properties  # provider config (server role + registry + observability, metrics :9090)
├── consumer/main.go         # discovers the provider via etcd, calls it and asserts, then exits (Go-Spring style: client bean + gs.Run())
├── consumer/conf/app.properties  # consumer config (client role + registry + observability, metrics :9091)
├── docker/                  # Prometheus & Promtail config for the backend stack
├── docker-compose.yml       # local etcd + Prometheus + Jaeger + Loki + Promtail
└── scripts/smoke-test.sh    # smoke test: bring up backends+provider, run consumer, tear down
```

## How it was generated

Nothing was generated. JSON-RPC has no protobuf/thrift IDL and no code
generator in dubbo-go v3 — the service surface is a hand-written Go file
(`idl/greet.go`) that pins the Java-style interface name and method
names, plus a hand-written provider struct with the matching method
signature. Running `./idl/gen-code.sh` prints a one-line "nothing to do" for
symmetry with the Triple sibling.

Any JSON-serializable Go type can be used as a parameter or return; there
is no equivalent to Hessian2's POJO registration table.

## Choosing this protocol vs. Triple / classic-Dubbo

| Concern              | Triple (`../triple`)                | Dubbo/Hessian2 (`../dubbo`)               | JSON-RPC (this module)                                  |
| -------------------- | ----------------------------------- | ----------------------------------------- | ------------------------------------------------------- |
| Transport            | HTTP/2                              | Raw TCP                                   | HTTP/1.1                                                |
| Payload              | protobuf                            | Hessian2                                  | JSON                                                    |
| IDL                  | `.proto` + `protoc-gen-go-triple`   | none — hand-written Go structs            | none — hand-written Go structs                          |
| Cross-language reach | Any gRPC/Triple client              | Java Dubbo (native), Hessian2 runtimes    | Anything speaking HTTP + JSON (curl, browsers, ...)     |
| Client call style    | Typed stub (`svc.Greet(ctx, req)`)  | Reflective (`conn.CallUnary(...)`)        | Reflective (`conn.CallUnary(...)`)                      |
| When to pick         | Greenfield Go microservices         | Interop with existing Java Dubbo services | Debugging / bare-HTTP clients / lowest common denominator |

## Configuration

The provider and consumer each own a `conf/app.properties` under their own
directory (`provider/conf/`, `consumer/conf/`); at startup each process chdirs
into its own directory (see `main.go`) and loads its file. The two share the
same registry, application name, and tracing setup, but differ where they must
not collide — the metrics port and log file — so no runtime env-var overrides
are needed. Both snippets below are drawn from the provider's file.

```properties
# Disable the built-in HTTP server; the provider exposes only JSON-RPC and the
# consumer runs server-less.
spring.http.server.enabled=false

# Registries are defined once, only here under ${spring.dubbo.registries}. The
# map key is a logical registry ID; the type defaults to the key when no
# `protocol` is given. Roles never define registries inline — they reference
# these by ID via ${...registry-ids}. With one registry defined, neither role
# sets registry-ids, so both the provider (server) and consumer (client) use it
# by default. Matches docker-compose.yml.
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379

# Provider protocol listener; the key under ${spring.dubbo.server.protocols} is
# the dubbo-go protocol name. JSON-RPC on 20002 (20000/20001 are reserved for
# the Triple / classic-Dubbo siblings so all three can coexist on one host).
spring.dubbo.server.protocols.jsonrpc.port=20002
```

The Dubbo **client** is provided by starter-dubbo as a default bean
(`__default__`) built from `${spring.dubbo.client}` plus the top-level
`${spring.dubbo.registries}`; the consumer autowires it and dials the service.
`spring.dubbo.client.protocol=jsonrpc` is what makes `NewClient` apply
`client.WithClientProtocolJsonRPC()` under the hood. Multiple named clients can
be declared under `${spring.dubbo.client.instances}` (bean name = the map key).
To run two registries of the same type, give each a distinct map-key ID and set
`protocol` explicitly, e.g. `spring.dubbo.registries.bj.protocol=etcdv3` /
`...sh.protocol=etcdv3`, then let each role pick with `registry-ids` (e.g.
`spring.dubbo.client.registry-ids=bj`).

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
example turns them on explicitly in each role's `conf/app.properties` and wires
a full local backend stack in `docker-compose.yml` so all three signals —
metrics, traces, logs — are visible end to end. None of this needs any code; it
is all configuration layered on the shared `Instance` bean.

| Signal  | Produced by                                    | Backend         | Where to look                                        |
| ------- | ---------------------------------------------- | --------------- | ---------------------------------------------------- |
| Metrics | dubbo-go Prometheus exporter, port `:9090`     | Prometheus      | UI http://127.0.0.1:9099 (query `up`, `dubbo_*`)     |
| Traces  | dubbo-go OTel → OTLP/gRPC `127.0.0.1:4317`     | Jaeger          | UI http://127.0.0.1:16686 (service `jsonrpc-demo`)   |
| Logs    | go-spring `log` → JSON files under `logs/`     | Loki (Promtail) | Loki HTTP API, port `:3100` (query below)            |

### Architecture & how it works

The provider and consumer run **on the host** (scripts/smoke-test.sh builds and runs them);
every backend runs **in a container** (docker-compose.yml). The host↔container
boundary is why each signal takes a slightly different path:

```
        HOST (provider + consumer)              DOCKER (docker-compose.yml)
  ┌────────────────────────────────┐        ┌──────────────────────────────┐
  │ provider                       │  reg/  │ etcd            :2379         │
  │   JSON-RPC (HTTP/1) :20002     │◀─disc─▶│   service registry            │
  │   /metrics (HTTP)   :9090      │        │                               │
  │   OTel SDK ─┐                  │        │ Prometheus      :9099 (UI)    │
  │   log file  │ ─┐               │        │ Jaeger    :4317 / :16686 (UI) │
  │ consumer    │  │               │        │ Loki            :3100         │
  │   /metrics  │  │    :9091      │        │ Promtail (tails /var/log/app) │
  │   OTel SDK ─┤  │               │        └──────────────────────────────┘
  │   log file  │  │               │
  └─────────────┼──┼───────────────┘
                │  │
   (1) METRICS — pull:   Prometheus ──GET /metrics every 5s──▶ provider :9090
                         (reaches the host via host.docker.internal)
   (2) TRACES  — push:   OTel SDK ──OTLP/gRPC spans──▶ Jaeger :4317 ─▶ :16686 UI
   (3) LOGS    — tail+push: process ─write▶ ../logs/*.log ◀─bind-mount─ Promtail
                         Promtail ──HTTP push──▶ Loki :3100 ─▶ query API
```

**(1) Metrics — a pull model.** starter-dubbo's built-in Prometheus registry
(enabled by `spring.dubbo.metrics.*`) stands up a plain HTTP endpoint that
renders the current counter/gauge values on demand — it pushes nothing. The
provider serves it on `:9090`, the consumer on `:9091`. Prometheus is the active
party: on its `scrape_interval` (5s here) it issues `GET /metrics`, parses the
text-format response, and stores each sample with a timestamp. Because Prometheus
is in a container and the target is on the host, `docker/prometheus.yml`
targets `host.docker.internal:9090` (mapped via `extra_hosts` on Linux, native on
macOS/Windows). Metrics are registered lazily, so `dubbo_*` rows only appear
*after* the first RPC — that is why the smoke test calls before asserting.

**(2) Traces — a push model.** The dubbo-go OTel integration (enabled by
`spring.dubbo.tracing.*`) wraps each RPC in a span. An in-process batch span
processor buffers spans and exports them over **OTLP/gRPC** to the endpoint in
config (`127.0.0.1:4317`), which is Jaeger's mapped collector port. Here the
*application* is the active party — it pushes to the collector; Jaeger stores the
spans and serves them at the `:16686` UI under service `jsonrpc-demo`.
`mode=always`/`ratio=1.0` samples every span, so even a single call shows up.

**(3) Logs — tail then push.** No log ever travels over the network from the
application. go-spring's `log` module (a `FileLogger` with a `JSONLayout`) writes
structured JSON lines to `../logs/<role>.log` on the host. That `logs/` directory
is **bind-mounted read-only** into the Promtail container at `/var/log/app`
(docker-compose.yml). Promtail (`docker/promtail-config.yml`) tails
`*.log`, tracks its read offset in a positions file, tags each line with
`job="jsonrpc-demo"` plus the source `filename`, and **pushes** batches to Loki's
`:3100` HTTP API. Loki indexes only the labels; the JSON body stays queryable via
the label selectors shown in the manual steps.

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
logging.logger.root.dir=../logs
logging.logger.root.file=provider.log
```

**Two processes, two config files.** The provider and consumer read **separate**
`conf/app.properties` files (under `provider/` and `consumer/`), so the values
that would otherwise clash are simply set to different literals: the provider
serves metrics on `:9090` and logs to `provider.log`, the consumer serves
metrics on `:9091` and logs to `consumer.log`. Both write into the same
module-root `logs/` (via `../logs`) that Promtail tails. No env-var overrides are
needed — just run each process:

```bash
go run ./consumer
```

### Manual verification (step by step)

`scripts/smoke-test.sh` asserts only **endpoint liveness** (the provider serves `dubbo_*`
metrics, the RPC round-trips, no backend container crashed); it does **not** wait
for data to actually land in Prometheus/Jaeger/Loki. The steps below let you
confirm each signal by hand and see exactly what to expect at every stage. Run
them **while the stack is up** and **after at least one RPC has been made**.

> All `curl`/`open` targets are on `127.0.0.1`. If your shell has an HTTP proxy
> exported, add `127.0.0.1,localhost` to `no_proxy`/`NO_PROXY` first, otherwise
> the local requests get routed through the proxy and return nothing.

**Step 0 — bring everything up and make the calls.**

```bash
docker compose up -d          # or docker-compose up -d
go run ./provider &           # Terminal A: long-lived
go run ./consumer             # Terminal B: makes 21 Greet calls (1 + a batch of 20)
```

Expected: the consumer prints the canonical line, then a batch summary, then
exits.

```
Response from discovered provider: Hello, Dubbo-Go!
Sent 21 greetings (1 canonical + 20 batch)
```

**Step 1 — the provider exposes `dubbo_*` metrics.** Metrics are registered
lazily, so this only returns rows *after* the calls in step 0.

```bash
curl -s http://127.0.0.1:9090/metrics | grep '^dubbo_provider_requests_total{'
```

Expected — a counter for the `Greet` method at `21` (all calls from step 0):

```
dubbo_provider_requests_total{application_name="jsonrpc-demo",group="",interface="com.example.GreetService",method="Greet",version="",...} 21
```

**Step 2 — Prometheus scraped the provider.** Query Prometheus's HTTP API (UI is
on `:9099`, its container port `9090` is remapped to avoid clashing with the
provider's `:9090`).

```bash
# a) the scrape target is healthy (value "1" = up)
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=up{job="jsonrpc-provider"}'

# b) the dubbo metric made it into Prometheus
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=dubbo_provider_requests_total'
```

Expected — `"status":"success"` and a result whose `"value"` ends in `"21"`
(the `up` query ends in `"1"`, i.e. healthy):

```json
{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up","job":"jsonrpc-provider","instance":"host.docker.internal:9090","role":"provider","service":"jsonrpc-demo"},"value":[...,"1"]}]}}
```

Or open the UI and query `up` / `dubbo_*`:

```bash
open http://127.0.0.1:9099
```

**Step 3 — the trace reached Jaeger.**

```bash
# a) the service registered
curl -s 'http://127.0.0.1:16686/api/services'

# b) several traces now exist, each with a "Greet" span
curl -s 'http://127.0.0.1:16686/api/traces?service=jsonrpc-demo&limit=30'
```

Expected — the service list contains `jsonrpc-demo`, and the traces payload holds
multiple traces (one per RPC), each containing a span whose `operationName` is
`Greet`:

```json
{"data":["jsonrpc-demo"],"total":1,"limit":0,"offset":0,"errors":null}
```

Or open the Jaeger UI, pick service `jsonrpc-demo`, and click *Find Traces*:

```bash
open http://127.0.0.1:16686
```

**Step 4 — the logs reached Loki (via Promtail).**

```bash
# a) Promtail is shipping both files
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/label/filename/values'

# b) query the actual JSON log lines from the last hour
END=$(date +%s)000000000; START=$(($(date +%s)-3600))000000000
curl -s -G 'http://127.0.0.1:3100/loki/api/v1/query_range' \
  --data-urlencode 'query={job="jsonrpc-demo"}' \
  --data-urlencode "start=$START" --data-urlencode "end=$END" \
  --data-urlencode 'limit=5'
```

Expected — (a) lists both files, (b) returns `"status":"success"` with one or
more streams of JSON log lines:

```json
{"status":"success","data":["/var/log/app/consumer.log","/var/log/app/provider.log"]}
```

**Step 5 — the log files exist on disk.** Both processes write into the shared
module-root `logs/` (each via `../logs`); this is the directory Promtail
bind-mounts.

```bash
ls logs/
head -1 logs/provider.log
```

Expected — two files, and structured JSON lines:

```
consumer.log  provider.log
{"level":"info","time":"...","fileLine":"...","tag":"_app_def","msg":"ready",...}
```

**Step 6 — no backend crashed.**

```bash
docker compose ps        # or docker-compose ps
```

Expected — all five containers `Up`:

```
contrib-dubbo-go-jsonrpc-etcd         Up
contrib-dubbo-go-jsonrpc-jaeger       Up
contrib-dubbo-go-jsonrpc-loki         Up
contrib-dubbo-go-jsonrpc-prometheus   Up
contrib-dubbo-go-jsonrpc-promtail     Up
```

## Known upstream issue: Go 1.26 (`jsonv2` experiment) x dubbo-go v3.3.1

Under a Go toolchain built with the `jsonv2` experiment enabled
(`runtime.Version()` carries an `-X:jsonv2` suffix, the default on Go 1.26),
`dubbo.apache.org/dubbo-go/v3/protocol/jsonrpc.(*serverRequest).UnmarshalJSON`
recurses indefinitely: it calls `encoding/json.Unmarshal` on its own receiver
type, and the v2 arshaler treats the method as an override and dispatches
back into `UnmarshalJSON`, exploding the goroutine stack. The provider
process crashes on the first request, so the next consumer dial to the same
port fails with `connect: connection refused`.

The example code itself is correct — this is an upstream defect in dubbo-go
v3.3.1's JSONRPC protocol implementation. Options:

- Run this example on a Go toolchain **without** the `jsonv2` experiment
  (`GOEXPERIMENT=nojsonv2` at Go-build time, or a Go 1.25 toolchain).
- Wait for a dubbo-go release that stops calling `json.Unmarshal` on the
  method receiver from inside its own `UnmarshalJSON`.

`go build ./...` / `go vet ./...` succeed on any toolchain; `scripts/smoke-test.sh` will
fail until the upstream fix is in place.
