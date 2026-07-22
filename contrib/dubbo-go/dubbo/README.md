# dubbo-go — Dubbo/Hessian2 (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` example that speaks the **classic Dubbo protocol** — TCP
transport with **Hessian2** serialization — wired the Go-Spring way via the
reusable **starter-dubbo** module: it supplies the `gs.Server` adapter,
`gs.Run()` drives the lifecycle, the provider is just a `ServiceRegister`
bean, and the protocol and registry come from `conf/app.properties` instead
of hard-coded `main()` wiring.

Unlike the Triple sibling in [`../triple`](../triple), this protocol has no
protobuf IDL and no code generator in dubbo-go v3: services are plain Go
structs whose exported method signatures are reflected over at registration
time and marshalled with Hessian2 on the wire. That makes classic Dubbo the
interop path for calling into Java Dubbo services (which use the same
protocol natively); Triple is the recommended protocol for greenfield Go
microservices.

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
  │ → dubbo://<host>:20001                   │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀─── Dubbo (Hessian2) ──│  consumer  │
│ gs.Run()   │      Greet(name)       │ one-shot   │
│ :20001     │──────────────────────▶│ assert+exit│
└────────────┘       echo name        └────────────┘
```

## Layout

```
contrib/dubbo-go/dubbo/
├── idl/greet.go             # the "IDL": interface name + method-name constants
├── idl/gen-code.sh          # no-op — classic Dubbo has no IDL codegen
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

Nothing was generated. Classic Dubbo/Hessian2 has no protobuf/thrift IDL and
no code generator in dubbo-go v3 — the service surface is a hand-written Go
file (`idl/greet.go`) that pins the Java-style interface name and method
names, plus a hand-written provider struct with the matching method
signature. Running `./idl/gen-code.sh` prints a one-line "nothing to do" for symmetry
with the Triple sibling.

If your service uses non-primitive types, register them with
`hessian.RegisterPOJO(&MyStruct{})` — Hessian2 needs the Go↔Java type map to
be seeded at process start. This example uses only `string`, so no
registration is needed.

## Choosing this protocol vs. Triple

| Concern              | Triple (`../triple`)                  | Dubbo/Hessian2 (this module)                            |
| -------------------- | ------------------------------------- | ------------------------------------------------------- |
| Transport            | HTTP/2                                | Raw TCP                                                 |
| Payload              | protobuf                              | Hessian2                                                |
| IDL                  | `.proto` + `protoc-gen-go-triple`     | none — hand-written Go structs                          |
| Cross-language reach | Any gRPC/Triple client                | Java Dubbo (native), any Hessian2-capable runtime       |
| Client call style    | Typed stub (`svc.Greet(ctx, req)`)    | Reflective (`conn.CallUnary(ctx, args, resp, "Greet")`) |
| When to pick         | Greenfield Go microservices           | Interop with existing Java Dubbo services               |

## Configuration

The provider and consumer each own a `conf/app.properties` under their own
directory (`provider/conf/`, `consumer/conf/`); at startup each process chdirs
into its own directory (see `main.go`) and loads its file. The two share the
same registry, application name, and tracing setup, but differ where they must
not collide — the metrics port and log file — so no runtime env-var overrides
are needed. Both snippets below are drawn from the provider's file.

```properties
# Disable the built-in HTTP server; the provider exposes only Dubbo and the
# consumer runs server-less.
spring.http.server.enabled=false

# Registries are defined once, only here under ${spring.dubbo.registries}. The
# map key is a logical registry ID; the type defaults to the key when no
# `protocol` is given. Roles never define registries inline — they reference
# these by ID via ${...registry-ids}. With one registry defined, neither role
# sets registry-ids, so both the provider (server) and consumer (client) use it
# by default. Matches docker-compose.yml.
spring.dubbo.registries.etcdv3.address=127.0.0.1:2379

# Provider protocol listener; the key under ${spring.dubbo.protocols} is
# the dubbo-go protocol name. Classic Dubbo on 20001 (20000 is reserved for the
# Triple sibling so both can coexist on one host).
spring.dubbo.protocols.dubbo.port=20001
```

The Dubbo **client** is provided by starter-dubbo as a single process-wide bean
(built from `${spring.dubbo.client}` on top of the shared
`${spring.dubbo.registries}`); the consumer autowires it by type and dials the
service. To run two registries of the same type, give each a distinct map-key ID
and set `protocol` explicitly, e.g. `spring.dubbo.registries.bj.protocol=etcdv3`
/ `...sh.protocol=etcdv3`, then let the client (or a per-reference entry) pick
with `registry-ids` (e.g. `spring.dubbo.client.registry-ids=bj`).

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
| Traces  | dubbo-go OTel → OTLP/HTTP `127.0.0.1:4318`     | Jaeger          | UI http://127.0.0.1:16686 (service `dubbo-demo`)     |
| Logs    | go-spring `log` → JSON files under `logs/`     | Loki (Promtail) | Loki HTTP API, port `:3100` (query below)            |

### Architecture & how it works

The provider and consumer run **on the host** (scripts/smoke-test.sh builds and runs them);
every backend runs **in a container** (docker-compose.yml). The host↔container
boundary is why each signal takes a slightly different path:

```
        HOST (provider + consumer)              DOCKER (docker-compose.yml)
  ┌────────────────────────────────┐        ┌──────────────────────────────┐
  │ provider                       │  reg/  │ etcd            :2379         │
  │   dubbo/Hessian2   :20001      │◀─disc─▶│   service registry            │
  │   /metrics (HTTP)  :9090       │        │                               │
  │   OTel SDK ─┐                  │        │ Prometheus      :9099 (UI)    │
  │   log file  │ ─┐               │        │ Jaeger    :4318 / :16686 (UI) │
  │ consumer    │  │               │        │ Loki            :3100         │
  │   /metrics  │  │    :9091      │        │ Promtail (tails /var/log/app) │
  │   OTel SDK ─┤  │               │        └──────────────────────────────┘
  │   log file  │  │               │
  └─────────────┼──┼───────────────┘
                │  │
   (1) METRICS — pull:   Prometheus ──GET /metrics every 5s──▶ provider :9090
                         (reaches the host via host.docker.internal)
   (2) TRACES  — push:   OTel SDK ──OTLP/HTTP spans──▶ Jaeger :4318 ─▶ :16686 UI
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
processor buffers spans and exports them over **OTLP/HTTP** to the endpoint in
config (`127.0.0.1:4318`), which is Jaeger's mapped OTLP/HTTP collector port. Here the
*application* is the active party — it pushes to the collector; Jaeger stores the
spans and serves them at the `:16686` UI under service `dubbo-demo`.
`mode=always`/`ratio=1.0` samples every span, so even a single call shows up.

> **Why OTLP/HTTP instead of OTLP/gRPC?** dubbo-go v3.3.1's otlp-grpc exporter
> (`newGrpcExporter` in `otel/trace/otlp/exporter.go`) **ignores the `insecure`
> flag** and always dials TLS, which the plaintext Jaeger all-in-one collector
> rejects ("bogus greeting"), so spans never arrive. The otlp-http exporter
> honors `insecure`, so this example uses OTLP/HTTP (`:4318`). Also note that if
> your shell exports `OTEL_EXPORTER_OTLP_ENDPOINT`, the OTel SDK folds it into the
> export URL (you may see a 404); `unset` it before running.

**(3) Logs — tail then push.** No log ever travels over the network from the
application. go-spring's `log` module (a `FileLogger` with a `JSONLayout`) writes
structured JSON lines to `../logs/<role>.log` on the host. That `logs/` directory
is **bind-mounted read-only** into the Promtail container at `/var/log/app`
(docker-compose.yml). Promtail (`docker/promtail-config.yml`) tails
`*.log`, tracks its read offset in a positions file, tags each line with
`job="dubbo-demo"` plus the source `filename`, and **pushes** batches to Loki's
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

# tracing (OTel → Jaeger over OTLP/HTTP); mode=always so even a single call is sampled
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=otlp-http
spring.dubbo.tracing.endpoint=127.0.0.1:4318
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
dubbo_provider_requests_total{application_name="dubbo-demo",group="",interface="com.example.GreetService",method="Greet",version="",...} 21
```

**Step 2 — Prometheus scraped the provider.** Query Prometheus's HTTP API (UI is
on `:9099`, its container port `9090` is remapped to avoid clashing with the
provider's `:9090`).

```bash
# a) the scrape target is healthy (value "1" = up)
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=up{job="dubbo-provider"}'

# b) the dubbo metric made it into Prometheus
curl -s -G 'http://127.0.0.1:9099/api/v1/query' \
  --data-urlencode 'query=dubbo_provider_requests_total'
```

Expected — `"status":"success"` and a result whose `"value"` ends in `"21"`
(the `up` query ends in `"1"`, i.e. healthy):

```json
{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up","job":"dubbo-provider","instance":"host.docker.internal:9090","role":"provider","service":"dubbo-demo"},"value":[...,"1"]}]}}
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
curl -s 'http://127.0.0.1:16686/api/traces?service=dubbo-demo&limit=30'
```

Expected — the service list contains `dubbo-demo`, and the traces payload holds
multiple traces (one per RPC), each containing a span whose `operationName` is
`Greet`:

```json
{"data":["dubbo-demo"],"total":1,"limit":0,"offset":0,"errors":null}
```

Or open the Jaeger UI, pick service `dubbo-demo`, and click *Find Traces*:

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
  --data-urlencode 'query={job="dubbo-demo"}' \
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
contrib-dubbo-go-dubbo-etcd         Up
contrib-dubbo-go-dubbo-jaeger       Up
contrib-dubbo-go-dubbo-loki         Up
contrib-dubbo-go-dubbo-prometheus   Up
contrib-dubbo-go-dubbo-promtail     Up
```
