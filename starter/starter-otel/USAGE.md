# starter-otel Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `config.go`, `exporters.go`, `trace/`, `metric/`) and the runnable
[example/](example/) (`example/check.sh` — verified passing). **OTel concepts (spans, tracer/meter
providers, exporters, sampling, [W3C trace context](https://www.w3.org/TR/trace-context/)) are
OpenTelemetry's own** — see the [OTel Go documentation](https://opentelemetry.io/docs/languages/go/)
and the [Collector docs](https://opentelemetry.io/docs/collector/). Everything below is go-spring's
increment: one flat config prefix, process-global provider installation, the actuator seam, and
shutdown flushing.

**Activation**: importing the starter activates the OTel SDK — there is no `enabled`-to-import key.
The on/off switch inside the import is `spring.observability.enable` (default `true`). Providers are
NOT beans: they are installed as OTel process globals during module setup, before any bean is
constructed (see §2.1).

---

## 1. Complete worked project

A realistic service serving HTTP (starter-echo) with health probes, Prometheus metrics on the
actuator port, and OTLP traces to a collector. File tree:

```
demo/
├── go.mod
├── main.go
├── loghook.go
├── router.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/labstack/echo/v4    latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-echo     latest
    go-spring.org/starter-actuator latest   // probes + /metrics mount
    go-spring.org/starter-otel     latest
    go.opentelemetry.io/otel       v1.45.x  // only if you touch the OTel API directly (loghook.go)
)
```

**main.go**:

```go
package main

import (
    _ "demo/loghook"
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-echo"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**loghook.go** — trace↔log correlation is deliberately app-owned (starter-otel does not ship an
installer; see §6):

```go
package loghook

import (
    "context"

    "go-spring.org/log"
    "go.opentelemetry.io/otel/trace"
)

func init() {
    // log.FieldsFromContext is the single hook go-spring's log calls on every
    // record; lifting trace_id/span_id off the context correlates logs with traces.
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        sc := trace.SpanContextFromContext(ctx)
        if !sc.IsValid() {
            return nil
        }
        return []log.Field{
            log.String("trace_id", sc.TraceID().String()),
            log.String("span_id", sc.SpanID().String()),
        }
    }
}
```

**router.go**:

```go
package router

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/spring/gs"

    StarterEcho "go-spring.org/starter-echo"
)

func init() {
    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            e.GET("/hello/:name", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{"msg": "hi " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented observability surface:

```properties
# --- echo server -------------------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# --- actuator (probes + metrics mount) ---------------------------------------
spring.actuator.addr=:9370

# --- observability (starter-otel) --------------------------------------------
spring.observability.enable=true
# service-name defaults to ${spring.application.name} and falls back to
# go-spring-app — always set one of the two or every backend shows the generic
# name and merges this service's traffic with every other defaulted service.
spring.observability.service-name=demo

# Traces: batched, pushed to a local OTLP collector over plaintext gRPC.
# W3C TraceContext + Baggage propagation is installed as the process global,
# so every instrumented client propagates traceparent on outbound requests.
spring.observability.trace.enable=true
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
spring.observability.trace.propagator=w3c

# Metrics: pull-based Prometheus. port=0 disables the dedicated scrape server,
# so /metrics is served ONLY through the actuator management port — one port
# for probes and metrics (the consolidation the example proves).
spring.observability.metrics.enable=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
spring.observability.metrics.path=/metrics

# Go runtime metrics (GC, heap, goroutines, GOMAXPROCS) into the same
# MeterProvider — scraped alongside the HTTP/echo metrics above.
spring.observability.metrics.runtime.enable=true
```

**Verify**:

```bash
curl -i :8002/hello/world                                  # 200
curl -s :9370/metrics | grep -E 'go_goroutine_count|http_server_request_duration'
curl -i :9370/health                                       # probes alive next to /metrics
```

With zero config beyond the imports, the starter is NOT inert: defaults are `otlp-grpc` trace +
`otlp-grpc` metrics exporters with empty endpoints, which silently aim at `localhost:4317` (see §5,
row 3). Configure or disable explicitly in every environment.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline — and why setup runs BEFORE bean construction

```
import starter-otel
  └─ init(): blank-imports register exporters (exporters.go:27-33)
  └─ gs.Module(nil, setup) registered                     [starter.go:62]
gs.Run()
  ├─ config load + .env (pre-config)
  ├─ RefreshPrepare → applyModules → setup()              [starter.go:69-93]
  │     ├─ bind ${spring.observability} → Config
  │     ├─ enable=false → log + return (globals stay OTel no-ops)   [starter.go:74-77]
  │     ├─ trace.NewResource(service-name)                [trace/provider.go:32-36]
  │     ├─ setupTrace: TracerProvider + propagator → otel.Set*Globals
  │     │     └─ gs.RegisterStopper("otel-trace", tp.Shutdown)      [starter.go:121]
  │     └─ setupMetrics: MeterProvider → otel.SetMeterProvider
  │        ├─ runtime metrics (once per process)                     [starter.go:184-189]
  │        ├─ prometheus: RegisterStopper("otel-metrics-scrape-server", ...)
  │        └─ Provide(metric.NewEndpoint).Export(As[endpoint.Endpoint])
  ├─ bean construction (gorm client, echo engine, http transports, ...)
  │     — every component reading otel.GetTracerProvider()/GetMeterProvider()
  │       sees the live globals
  ├─ servers Run → readiness
  └─ on SIGTERM: servers stop → container closes → runStoppers flush  [gs/stopper.go:86-100]
```

The ordering is load-bearing, and the source says why (starter.go:56-61):

> This must be a `gs.Module`, not a plain bean: its body executes during applyModules in the
> RefreshPrepare phase, i.e. BEFORE any bean is instantiated. Setting the OTel globals here
> therefore guarantees they are live before component beans (e.g. a gorm client calling `db.Use`)
> are constructed. Building the providers lazily inside a bean constructor would break that
> ordering.

Consequence for users: there is nothing to autowire and no init-order pitfall — but also nothing to
override at runtime. If `enable=false`, the globals remain the SDK's no-op providers, so an
imported-but-disabled starter has no effect (starter.go:66-68).

### 2.2 Exporter registry

Both pillars use the same driver-registry idiom (`trace/registry.go`, `metric/registry.go` over the
shared generic `internal/registry`). Built-ins self-register at init via the blank imports in
`exporters.go`:

| Pillar | Registry name | Kind | Endpoint default | Notes |
|--------|--------------|------|------------------|-------|
| trace | `otlp-grpc` | push (batcher) | `localhost:4317` | default exporter |
| trace | `otlp-http` | push (batcher) | `localhost:4318` | |
| trace | `stdout` | push | — | local debugging |
| trace | `none` | — | — | pillar fully skipped (starter.go:103) |
| metrics | `otlp-grpc` | push (PeriodicReader, `interval`) | `localhost:4317` | default exporter |
| metrics | `otlp-http` | push (PeriodicReader, `interval`) | `localhost:4318` | |
| metrics | `prometheus` | **pull** | serves `path` on `port` / actuator | `otelprom.New` is itself the Reader; handler renders a dedicated registry (`metric/prometheus/exporter.go`) |
| metrics | `stdout` | push (PeriodicReader) | — | |
| metrics | `none` | — | — | pillar skipped (starter.go:134) |

An unknown name fails setup loudly with a self-diagnosing error listing the registered exporters
(`unknownExporterErr`, trace/registry.go:58-61). Applications add backends via
`trace.RegisterSpanExporter(name, factory)` / `metric.RegisterMeterExporter(name, factory)` — same
path the built-ins use; duplicate/nil registration panics at init.

### 2.3 Shutdown: stopper flush ordering

On SIGTERM, after all servers have stopped and the IoC container has closed, `runStoppers` invokes
every registered stopper (gs/stopper.go:26-30, 86-100). Registered here: `otel-trace`
(tp.Shutdown), `otel-metrics` (mp.Shutdown), and — prometheus with port>0 only —
`otel-metrics-scrape-server` (srv.Shutdown).

- **There is no defined flush order.** Stoppers run in map-iteration order and are contractually
  independent (gs/stopper.go:51-55): "Stoppers must be independent: like servers, they run in no
  defined order and must not rely on one another's cleanup having run." So do not reason "trace
  flushes before metrics" — either may go first.
- **No timeout is applied.** The context passed to stoppers is `context.WithoutCancel(ctx)`
  (gs/stopper.go:92) — flush waits for the exporter's own retry/timeout behavior. A wedged
  collector connection can hold shutdown open until the OTLP exporter's own backoff gives up.
- **What is lost if skipped**: the trace side uses `sdktrace.WithBatcher` (trace/provider.go:55), so
  un-exported buffered spans exist between batch ticks; `tp.Shutdown` is precisely what flushes
  them. A `kill -9` (or skipping the stopper) drops every span still in the batch queue. The
  metrics PeriodicReader likewise flushes its last collection on `mp.Shutdown`. Prometheus is
  pull-based — nothing to flush; its stopper only closes the scrape server.
- A failing stopper is logged and does not block the rest (gs/stopper.go:94-97); the registry is
  drained so a second run is a no-op.

### 2.4 One traced request, end to end

`GET /hello/world` against the §1 project:

1. echo's Tracing middleware extracts the inbound `traceparent` (or starts a fresh trace) using the
   global propagator installed by setupTrace — `propagation.TraceContext{}` + `Baggage{}` for
   `w3c` (trace/provider.go:80-84).
2. The sampler decides: `sampler-ratio` maps to `ParentBased(Always|Ratio|Never)` (§3), so with an
   inbound sampled trace the decision follows the parent; otherwise the ratio applies.
3. Server span `{method} {route}` runs; the request-scoped context carries the SpanContext.
4. Your handler logs — the `log.FieldsFromContext` hook lifts `trace_id`/`span_id` onto the record.
5. Any instrumented outbound call (http-client transport, gorm bridge, ...) starts a child span from
   the same context and injects `traceparent` via the same global propagator.
6. Span ends → the SDK batcher queues it; export happens on the batcher's cadence, not per request.
7. Metrics (duration histogram, in-flight gauge) record through the global MeterProvider; the
   Prometheus reader serves them on the next scrape of `:9370/metrics`.
8. On SIGTERM: servers drain → container closes → `otel-trace`/`otel-metrics` stoppers flush the
   last batch to the collector (§2.3).

---

## 3. Per-key behavior reference

All under `spring.observability.*`. 17 keys total (verified against `grep -rhoE 'value:"[^"]+"'`).
`trace.*` and `metrics.*` are struct-bound sub-configs (`${trace}` / `${metrics}` in config.go:32-33).

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `enable` | bool | true | Master switch inside setup; false leaves OTel globals as SDK no-ops (starter.go:74-77). | false + instrumented components → everything runs, nothing is exported, no warning. |
| `service-name` | string | `${spring.application.name:=go-spring-app}` | Becomes the sole resource attribute `service.name` (schemaless resource, trace/provider.go:32-36). | Unset → silent `go-spring-app`; all defaulted services merge in every backend. |
| `trace.enable` | bool | true | false (or `exporter=none`) skips the TracerProvider only — the **propagator is still installed** (starter.go setupTrace): context/baggage relay keeps working even with no span export (2026-08 fix; the key used to be ignored entirely when tracing was off). | To drop propagation too, set `trace.propagator=none`. |
| `trace.exporter` | string | otlp-grpc | Registry lookup (§2.2): otlp-grpc \| otlp-http \| stdout \| none. | Unknown name → setup fails at boot with the registered-names error. |
| `trace.endpoint` | string | "" | Host:port for otlp exporters; empty falls back to SDK default localhost:4317/:4318 (trace/otlp/exporter.go). Ignored by stdout/none. | Wrong endpoint → one startup WARN from the connectivity probe (internal/probe, 3s TCP dial, never fatal — the collector may start later), then spans silently queue and drop until it is reachable. |
| `trace.insecure` | bool | true | Plaintext OTLP (WithInsecure) — the norm for a local/sidecar collector. | true against a TLS collector → export fails at runtime only. false against plaintext → same, other direction. |
| `trace.sampler-ratio` | float | 1.0 | `ParentBased` mapping: ≥1 AlwaysSample, (0,1) TraceIDRatioBased (trace/provider.go). | **≤0 is rejected at startup** (2026-08): non-positive ratios used to mean NeverSample — ALL spans silently dropped while everything looked healthy. To disable tracing use `trace.exporter=none` / `trace.enable=false` (propagation still works). |
| `trace.propagator` | string | w3c | Comma list of registered propagators composed in order; built-ins `tracecontext`+`baggage`, `w3c`=both; `none` leaves the process default untouched (trace/provider.go). A company adds its own (e.g. a named-header propagator) via `RegisterPropagator` and names it, e.g. `w3c,luohua`. ⚠ Ignored entirely when trace pillar is off. | Unknown name → setup error listing registered propagators. |
| `metrics.enable` | bool | true | false (or `exporter=none`) skips the metrics pillar (starter.go:134). | Same silent-nothing-exported posture as trace.enable. |
| `metrics.exporter` | string | otlp-grpc | otlp-grpc \| otlp-http \| prometheus \| stdout \| none (§2.2). | Unknown name → setup fails listing valid names. |
| `metrics.endpoint` | string | "" | otlp only; same SDK-default fallback as trace. Dead key for prometheus/stdout. | Same lazy-failure mode as trace.endpoint, plus the same one-shot startup probe WARN (internal/probe). |
| `metrics.insecure` | bool | true | otlp only. | Dead key for prometheus/stdout — set with no effect. |
| `metrics.port` | int | **9090** | prometheus only: >0 starts a **dedicated second HTTP server** bound synchronously at setup (metric/prometheus/exporter.go `serveMetrics`); 0 = scrape handler served solely via the actuator mount. | ⚠ Default 9090 + actuator → `/metrics` on BOTH ports (the endpoint bean is contributed regardless). 0 + no actuator in the process → startup WARN naming the remediation (2026-08, endpoint.IsServing() detection); port in use → boot fails loudly. |
| `metrics.path` | string | /metrics | prometheus only; used by BOTH the standalone server and the actuator mount (starter.go:169). | Custom path changes both surfaces — point your Prometheus scrape config at it. |
| `metrics.interval` | duration | 10s | Push cadence for otlp/stdout PeriodicReader (metric/provider.go:101-107). Dead for prometheus/none. | 0/negative keeps the reader's own default (not "as fast as possible"). |
| `metrics.runtime.enable` | bool | true | Feeds Go runtime metrics (GC, heap, goroutines, GOMAXPROCS) via OTel contrib, started exactly once per process (starter.go:184-189). | Disable loses `go_goroutine_count` etc. — the example smoke asserts on those. |
| `metrics.runtime.min-read-mem-stats-interval` | duration | 15s | Caps `runtime.ReadMemStats` (stop-the-world) frequency; 0 = instrumentation default (metric/provider.go:57-62). | Too low → measurable STW overhead at high cardinality of collection. |

⚠ **Per-exporter dead keys** (no warning at any phase): `endpoint`/`insecure` are dead for
stdout/prometheus/none; `port`/`path`/`interval` are dead for otlp/stdout/none.

---

## 4. Verification & fault drills

### 4.1 Self-contained smoke (no external services)

The committed example proves both core outcomes in-process:

```bash
cd starter/starter-otel/example && ./check.sh
# expects stdout spans printed, "log correlation OK: trace_id=... span_id=...",
# "actuator /metrics OK: runtime metrics exposed on :9370", "actuator /health OK"
```

### 4.2 Verify spans land in a collector (real export drill)

Run an OTLP-collector that prints to console, point the app's trace exporter at it, generate
traffic, observe spans:

```bash
# 1. Collector with a console exporter (save as otel-config.yaml):
# receivers:
#   otlp:
#     protocols:
#       grpc: { endpoint: 0.0.0.0:4317 }
#       http: { endpoint: 0.0.0.0:4318 }
# processors: { batch: {} }
# exporters:
#   debug: { verbosity: detailed }
# service:
#   pipelines:
#     traces:  { receivers: [otlp], processors: [batch], exporters: [debug] }
#     metrics: { receivers: [otlp], processors: [batch], exporters: [debug] }
docker run --rm -p 4317:4317 -p 4318:4318 \
  -v "$PWD/otel-config.yaml:/etc/otelcol/config.yaml" otel/opentelemetry-collector

# 2. Point the app at it (§1 config already does: trace.endpoint=127.0.0.1:4317,
#    switch metrics exporter to otlp-grpc to exercise the metrics pipeline too):
#    spring.observability.metrics.exporter=otlp-grpc
#    spring.observability.metrics.endpoint=127.0.0.1:4317

# 3. Generate traffic:
curl -s :8002/hello/world >/dev/null; curl -s :8002/hello/again >/dev/null

# 4. Watch the collector console: detailed spans named "GET /hello/:name" with
#    resource attribute service.name=demo; the metrics pipeline prints
#    go_goroutine_count / http server histograms every interval (10s).
# 5. Send SIGTERM (Ctrl-C) and confirm the final flush emits the last spans —
#    that is the otel-trace stopper (§2.3).
```

For a real backend swap the exporter to `otlp/jaeger`-style (per current Collector docs) and open
the Jaeger UI at :16686 — search by service `demo`. Sampler drill: set
`spring.observability.trace.sampler-ratio=0.05`, generate 100 requests, count spans in the
collector output — roughly 5 traces survive; `0` kills them all.

### 4.3 port=0 + actuator mount drill (metrics only via actuator)

```bash
# with the §1 config (metrics.exporter=prometheus, metrics.port=0):
curl -s :9370/metrics | grep go_goroutine_count        # served by actuator
curl -s --max-time 2 :9090/metrics                     # connection refused — no second server
curl -i :9370/health                                   # probes coexist on the same port
```

Flip to `metrics.port=9090` and restart: `/metrics` now answers on BOTH `:9090` (dedicated server,
log line `prometheus scrape server listening`) and `:9370` (actuator mount) — the endpoint bean is
contributed regardless of port (starter.go:164-172). Then remove the starter-actuator import with
`port=0`: `/metrics` is nowhere — but since 2026-08 this combo WARNs at startup with remediation.

### 4.4 Log↔trace correlation drill

With the §1 `loghook.go` installed:

```bash
curl -s :8002/hello/world >/dev/null
# the handler's business log line carries trace_id/span_id matching the span
# the collector printed for the same request — paste the id into Jaeger's search.
```

### 4.5 Runtime metrics + propagation drills

- Runtime: `curl -s :9370/metrics | grep -E '^go_'` → GC, heap, goroutine, GOMAXPROCS series
  (continuous — complementing starter-pprof's on-demand profiles).
- Propagation: `curl -H 'traceparent: 00-<32-hex-traceid>-<16-hex-spanid>-01' :8002/hello/world`
  → the collector shows the server span parented under that trace id (ParentBased honors the
  upstream sampled flag).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Everything runs, no traces/metrics anywhere | `enable=false`, pillar `enable=false`, or `exporter=none` | Check the three switches; all default to active — someone set them. |
| No spans, one startup WARN `OTLP endpoint ... is not reachable`, collector logs empty | OTLP exporters connect lazily; the startup probe (internal/probe) WARNs once per (signal, endpoint) when its 3s TCP dial fails — never fatal, since the collector may legitimately start later | Verify endpoint:port (4317 grpc vs 4318 http) and `insecure` vs TLS; the exporter keeps retrying in the background. |
| Spans appear for other services but never for this one | a very low `sampler-ratio` | Raise the ratio; ≤0 was rejected at startup since 2026-08 (to drop tracing entirely use exporter=none/enable=false). |
| Boot fails: `unknown exporter` / `unknown propagator` | Typo in `trace.exporter` / `metrics.exporter` / `trace.propagator` | The error lists registered exporters; use one of those names. |
| Boot fails: prometheus scrape server bind error | `metrics.port` > 0 and the port is in use (bind is synchronous by design — metric/prometheus/exporter.go) | Free the port, change it, or set 0 and mount via actuator. |
| `/metrics` answers on :9090 too, wanted actuator-only | Default `metrics.port=9090` starts the dedicated server even when the actuator mount exists | Set `spring.observability.metrics.port=0`. |
| `/metrics` is nowhere + startup WARN `prometheus exporter has no place to serve` | `port=0` but starter-actuator not linked into the process (endpoint.IsServing()==false) — detected at startup since 2026-08 | Import starter-actuator (and set `spring.actuator.addr`) or use a positive `metrics.port`. |
| Cross-service traces break at this hop, logs lose trace_id | trace pillar off (`trace.enable=false`/`none`) also skips propagator installation and you have no valid span context | Keep tracing enabled (propagator is cross-cutting — §6 suspect 5) and install the log.FieldsFromContext hook (§1). |
| Shutdown hangs after SIGTERM | Stopper context is `WithoutCancel` (gs/stopper.go:92); a wedged collector holds the flush | Fix collector reachability/TLS; export timeouts are the OTLP exporter's own. |
| All services named `go-spring-app` in the backend | Neither `service-name` nor `spring.application.name` set | Set one; the fallback is silent. |

---

## 6. Design health + suspects

| Metric | Value |
|--------|-------|
| Config keys | 17 |
| Required | 0 |
| Quickstart external deps | 0 (stdout) / 1 (collector for real export) |
| "Watch out" entries (§3 ⚠ + §5) | 8 |

Design suspects (for the audit ledger):

1. ~~README says providers are "beans with destroy hooks" and calls `endpoint` "required for otlp"~~
   **FIXED** — README now documents process-global stoppers and optional endpoint with SDK-default
   fallback (README.md "Graceful Shutdown", exporter tables).
2. ~~Silent dead-end: prometheus + `port=0` + no actuator → `/metrics` nowhere, no log.~~
   Fixed 2026-08: detected via endpoint.IsServing() and WARNed with remediation.
3. Per-exporter dead keys with no warning (`endpoint`/`insecure` vs `port`/`path`/`interval`).
4. `insecure=true` default; `service-name` silent fallback to `go-spring-app`.
5. Propagator key is inside the trace pillar — `trace.enable=false` silently disables cross-service
   context propagation, which is cross-cutting.
6. ~~Committed Mach-O binary in example/~~ **FIXED** — the binary is git-ignored (untracked local
   build artifact); `git ls-files` shows only sources in example/.
7. (New, from this audit) stopper flush has no timeout wrapper — a wedged exporter endpoint holds
   process shutdown open indefinitely (`context.WithoutCancel`, gs/stopper.go:92).
