# Observability Combinations (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

**One app, many observability back-ends.** This example takes a single,
unchanged dubbo-go **Triple** service (the same `GreetService` as
[`../dubbo-go/triple`](../dubbo-go/triple)) and points it at several
interchangeable **MTL** (Metrics / Traces / Logs) stacks, so you can compare the
popular observability combinations side by side.

The app is **byte-for-byte identical** across every stack. Its three signals are
fixed:

- **Metrics** — the provider serves a Prometheus scrape endpoint on `:9090`
  (consumer on `:9091`), `dubbo_*` series.
- **Traces** — OTel spans exported over **OTLP/gRPC** to `127.0.0.1:4317`.
- **Logs** — structured JSON lines written to `logs/*.log` (with `trace_id` /
  `span_id` fields when a span is active).

The *only* thing that changes between stacks is **who owns `:4317`, who scrapes
`:9090`, and who tails `logs/`** — i.e. the back-end pipeline, not the code. That
is the whole point: **instrumentation stays put, the back-end is pluggable.**

## Are M, T and L separable?

Technically yes — you can ship only metrics and nothing else. But their *value*
is in **correlation**: jumping from a slow trace to its logs (`trace_id` in the
log line), or from a latency histogram to a sample trace (exemplars). So every
stack here carries all three signals, and stack 3 is dedicated to showing them
wired together.

## The stacks

| # | Stack | Pipeline | Metrics | Traces | Logs | Highlight |
|---|-------|----------|---------|--------|------|-----------|
| **1** | [`1-classic`](stacks/1-classic) | app → back-end **directly** (no collector) | Prometheus (scrape) | Jaeger (OTLP) | Loki (Promtail) | The common starting point; each signal has its own back-end |
| **2** | [`2-collector`](stacks/2-collector) | app → **OTel Collector** → fan-out | Prometheus | Jaeger | Loki | Vendor-neutral single pipeline; instrumentation decoupled from back-ends |
| **3** | [`3-lgtm`](stacks/3-lgtm) | app → OTel Collector → **LGTM** | Prometheus (exemplars) | **Tempo** | Loki | **Correlation**: trace↔logs↔metric-exemplar jumps in Grafana |
| **5** | [`5-elastic`](stacks/5-elastic) | app → OTel Collector → **Elasticsearch** | Elasticsearch | Elasticsearch | Elasticsearch | All three signals in one store, viewed in Kibana (OTel-native, no Beats) |

> Numbering follows the survey we did while designing this project (stack 4,
> a single ClickHouse store à la SigNoz/Uptrace, and stack 6, VictoriaMetrics,
> were deliberately left out). Stack 5 (Elastic) is optional — it pulls the
> heaviest images and takes the longest to warm up.

### Other combinations, and why they are not here

The four stacks above span the whole design space along three axes —
**instrumentation** (framework-native vs manual OTel SDK vs eBPF auto), **pipeline**
(direct vs collector), and **storage** (per-signal vs all-in-one). Combinations
we considered but skipped:

- **ClickHouse all-in-one** (SigNoz / Uptrace) — one store, one UI; overlaps
  conceptually with the Elastic stack.
- **VictoriaMetrics + VictoriaLogs** — a lighter Prometheus/Loki alternative.
- **eBPF auto-instrumentation** (Grafana Beyla) — zero-code, a different
  *instrumentation* axis rather than a different back-end.
- **SaaS single-pane** (Datadog / New Relic / Honeycomb / Grafana Cloud) —
  usually also OTLP under the hood; needs an account.

## Layout

```
contrib/observability/
├── proto/                     # shared Triple IDL + generated stubs (DO NOT EDIT the *.go)
├── provider/                  # the GreetService provider (identical to dubbo-go/triple)
│   └── conf/app.properties    # metrics :9090, OTLP → :4317, JSON logs → ../logs
├── consumer/                  # discovers via etcd, calls, asserts, exits
│   └── conf/app.properties    # metrics :9091, JSON logs → ../logs
├── stacks/
│   ├── 1-classic/             # docker-compose + prometheus/promtail/grafana config
│   ├── 2-collector/           # + otel-collector-config.yml
│   ├── 3-lgtm/                # + otel-collector-config.yml, tempo.yml, correlation datasources
│   └── 5-elastic/             # + otel-collector-config.yml (elasticsearch exporter)
└── scripts/
    ├── gen-code.sh            # regenerate proto/*.go from the IDL
    └── smoke-test.sh <stack>  # bring up a stack + app, run consumer, tear down
```

Each `stacks/<name>/` is a self-contained docker-compose project (it includes
its own etcd). Only one stack runs at a time, so they all reuse the same host
ports.

## Run

Pick a stack and run its one-shot smoke test — it brings up the back-ends and
the registry, builds and runs the provider on the host, runs the consumer
(1 canonical + 20 batch calls), asserts, and tears everything down:

```bash
bash scripts/smoke-test.sh 1-classic     # or 2-collector | 3-lgtm | 5-elastic
```

Or run a stack manually and keep it up to explore the UIs:

```bash
cd stacks/3-lgtm
docker compose up -d               # or docker-compose up -d
cd ../..
mkdir -p logs
go run ./provider &                # Terminal A: long-lived, registers into etcd
go run ./consumer                  # Terminal B: makes the calls
```

Expected consumer output:

```
Response from discovered provider: Hello, Dubbo-Go!
Sent 21 greetings (1 canonical + 20 batch)
```

> If your shell exports an HTTP proxy, add `127.0.0.1,localhost` to
> `no_proxy`/`NO_PROXY` first, or the local `curl`/UI requests get routed away.

## Where to look (per stack)

All UIs are on `127.0.0.1`.

### 1-classic / 2-collector
| Signal | UI | Try |
|--------|----|-----|
| Metrics | Prometheus http://127.0.0.1:9099 | query `dubbo_provider_requests_total` |
| Traces | Jaeger http://127.0.0.1:16686 | service `obs-demo`, *Find Traces* |
| Logs | Grafana http://127.0.0.1:3000 | Explore → Loki → `{job="obs-demo"}` (stack 1) / `{service_name="obs-demo"}` (stack 2) |

### 3-lgtm (correlation)
Open Grafana http://127.0.0.1:3000 → **Explore**:
- **Tempo**: find a trace → each span links **to Loki** (filtered by `trace_id`)
  and **to Prometheus** (related metrics).
- **Prometheus**: enable *Exemplars* on a panel → click an exemplar dot → jumps
  **to the Tempo trace**.
- **Loki**: `{service_name="obs-demo"} | json` → the `trace_id` field is a
  clickable **derived field** into Tempo.

### 5-elastic
Open **Kibana** http://127.0.0.1:5601 → *Discover* / *Observability*. All three
signals land in Elasticsearch (`logs-obs-demo`, `traces-obs-demo`,
`metrics-obs-demo`) via the collector's `elasticsearch` exporter.

## How the app stays unchanged

The provider/consumer use **starter-dubbo's built-in observability**, driven
entirely by `conf/app.properties` — no observability code in `main.go`:

```properties
# metrics — a Prometheus scrape endpoint, independent of the HTTP server
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090

# tracing — OTel spans over OTLP/gRPC to whoever owns :4317 in the chosen stack
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317

# logging — structured JSON to ../logs, tailed by Promtail (stack 1) or the
# collector's filelog receiver (stacks 2/3/5)
logging.logger.root.type=FileLogger
logging.logger.root.layout.type=JSONLayout
logging.logger.root.dir=../logs
```

Because the tracing endpoint is always `127.0.0.1:4317`, switching stacks is
purely a matter of which container listens there — the app never knows the
difference. This is a runnable example, **not** a reusable starter module.

## Regenerating the proto

Same IDL and toolchain as the Triple example:

```bash
bash scripts/gen-code.sh
```

> On a go1.26 toolchain whose `runtime.Version()` carries an experiment suffix
> (e.g. `go1.26.1-X:jsonv2`), `protoc-gen-go-triple` v3.0.3 panics parsing the
> version — rebuild it from source with the version string truncated to its
> numeric part.
