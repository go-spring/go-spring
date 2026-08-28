# starter-kitex Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `internal/logger/logger.go`, `schema.json`) and the
runnable [example/](example/) / [example-otel/](example-otel/). **Kitex's own semantics**
(thrift/protobuf codegen, `kitex_gen`, middleware suites) are [Kitex's documentation](https://www.cloudwego.io/docs/kitex/)
— everything below is go-spring's increment.

**Activation**: the server bean exists only when `spring.kitex.server.addr` is set — that key
is the on/off switch (no `enabled` key; example/conf's "defaulting to :8888" comment is wrong,
there is no default). The starter inlines what a generated `xxxservice.NewServer` would do —
construct the raw `server.Server`, defer service binding to your `ServiceRegister` bean — so
the server is configured entirely from `conf/app.properties`.

---

## 1. Complete worked project

A thrift echo service with tracing, Prometheus metrics and an etcd-optional deployment
posture. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
├── kitex_gen/echo/…            # generated from echo.thrift (kitex tool)
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/cloudwego/kitex      latest
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-kitex     latest
    go-spring.org/starter-actuator  latest   // optional: probes (this server has none built in)
)
```

Note: starter-otel is not needed for this minimal posture — without it the starter falls back
to its own OTel provider (§4.2). Import it too and kitex automatically attaches to the
unified pipeline instead; the two never double-export (§4.2).

**main.go**:

```go
package main

import (
    _ "demo/service"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-kitex"
)

func main() { gs.Run() }
```

**service.go** — the application's entire RPC surface:

```go
package service

import (
    "context"

    "github.com/cloudwego/kitex/server"
    "go-spring.org/spring/gs"
    StarterKitex "go-spring.org/starter-kitex"
    echo "demo/kitex_gen/echo"
    "demo/kitex_gen/echo/echoservice"
)

func init() {
    // The application provides exactly ONE ServiceRegister bean. The starter
    // owns the raw server.Server and its lifecycle; you bind your generated
    // service onto the server it hands you.
    gs.Provide(func() StarterKitex.ServiceRegister {
        return func(svr server.Server) error {
            return echoservice.RegisterService(svr, &EchoServiceImpl{})
        }
    })
}

type EchoServiceImpl struct{}

func (s *EchoServiceImpl) Echo(ctx context.Context, req *echo.EchoRequest) (*echo.EchoResponse, error) {
    return &echo.EchoResponse{Message: req.Message}, nil
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- kitex server -------------------------------------------------------------
# Disable gs's built-in HTTP server; this process exposes only the Kitex port.
spring.http.server.enabled=false

# Activation key; resolved as a TCP addr, passed via server.WithServiceAddr.
spring.kitex.server.addr=:8888

# Service name advertised in EndpointBasicInfo (and etcd when registry is on).
spring.kitex.server.service.name=echo

# Required for thrift-generated services (see §3): kitex's thrift codegen adds
# WithCompatibleMiddlewareForUnary in its generated NewServer; this starter
# builds the server itself, so enable it explicitly. Leave off for protobuf.
spring.kitex.server.compatible-unary-middleware=true

# Etcd registry — opt-in. Empty (unset) runs a registry-free server clients
# reach directly by host:port; set it to publish the service for discovery.
# spring.kitex.server.registry.etcd=127.0.0.1:2379

# --- kitex observability (global-first, see §4.2) --------------------------------
# Both default on. With starter-otel imported, the tracing suite attaches to
# the global OTel pipeline (endpoint/insecure keys are ignored). Without it,
# the starter builds its own provider exporting OTLP/gRPC to tracing.endpoint.
spring.kitex.server.tracing.enable=true
spring.kitex.server.tracing.endpoint=127.0.0.1:4317
spring.kitex.server.tracing.insecure=true
spring.kitex.server.metrics.enable=true
# A dedicated kitex Prometheus listener starts ONLY on an explicitly set port
# (no default). Without starter-otel, set it to get a kitex /metrics endpoint;
# with starter-otel, kitex RPC metrics ride the global pipeline instead.
# spring.kitex.server.metrics.port=9090
spring.kitex.server.metrics.path=/metrics
```

**Verify** (mirrors example/check.sh, which is self-asserting):

```bash
go run . -manual           # "kitex server starting on :8888" appears
./example/check.sh         # dials :8888, asserts the echo round-trip, self-SIGTERMs
```

The example's client dials direct (`echoservice.NewClient("echo", client.WithHostPorts(":8888"))`);
kitex client semantics (resolvers, retries) are kitex's own.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-kitex
  └─ (side-effect import) internal/logger init(): klog.SetLogger(bridge)   [process-wide,
      before any kitex component captures the default logger]
  └─ gs.Provide(NewSimpleKitexServer, IndexArg(0, TagArg("${spring.kitex.server}")))
        .Export(gs.As[gs.Server]())
        .Condition(gs.OnProperty("spring.kitex.server.addr"))
        │
gs.Run()
  ├─ config bind: ${spring.kitex.server} → Config (value tags)
  ├─ bean wiring: SimpleKitexServer ← the app's ServiceRegister bean (non-nullable)
  ├─ Server.Run():
  │    ├─ net.ResolveTCPAddr(addr)
  │    ├─ server options: WithServiceAddr, WithServerBasicInfo{ServiceName}
  │    ├─ registry.etcd non-empty → etcd.NewEtcdRegistry + WithRegistry
  │    ├─ compatible-unary-middleware → WithCompatibleMiddlewareForUnary
  │    ├─ tracing.enable → globalTracingActive() probe:
  │    │    global pipeline (starter-otel) → WithSuite(tracing.NewServerSuite())
  │    │      reading the OTel globals - NO kitex provider created
  │    │    no global → provider.NewOpenTelemetryProvider(WithServiceName,
  │    │      WithExportEndpoint, WithEnableMetrics(false)) [stored for Stop]
  │    │      + WithSuite(tracing.NewServerSuite())
  │    ├─ metrics.enable → metrics.port>0 → WithTracer(prometheus.
  │    │    NewServerTracer(":port", path)); port unset → global pipeline rides
  │    │    (or dormant with an INFO hint when there is no global either)
  │    ├─ server.NewServer(opts...) ; reg(svr) binds the generated service
  │    └─ <-sig.TriggerAndWait()        # park until gs signals readiness
  │         go svr.Run()                # binds listener, registers into etcd, blocks
  │         select { Run-err | <-done }
  ├─ readiness: TriggerAndWait returns → Run starts
  └─ on SIGTERM: gs calls Stop → svr.Stop() (deregisters from etcd, graceful)
      → otelProvider.Shutdown(ctx) (flush spans) → close(done) → Run returns
```

If `ServiceRegister` is missing the container fails (non-nullable autowire); two
`ServiceRegister` beans fail with ambiguity.

### 2.2 What the starter installs vs what it doesn't

The starter composes kitex's native pieces — one tracing **suite**, one Prometheus **tracer**,
optional etcd registry, optional compatible-unary middleware — layered on last (source comment:
"so a provider lights up metrics and tracing purely from conf/app.properties"). It installs
**no middleware chain of its own**: kitex middleware is composed inside your `ServiceRegister`
(or via suites you attach in your codegen). Consequences vs the sibling RPC starters:

- no loadtest identification filter (no `traffic.IsLoadTest` tagging on this hop)
- no fault-injection filter (no `fault.InjectorFor` wiring; governance cannot "set fire" here)
- no resilience admission (rate-limit/breaker)

### 2.3 One call, layer by layer

Client `Echo("Hello")` with tracing+metrics on (defaults), no registry:

1. kitex transport accepts the connection on the service addr
2. the tracing suite (installed via `WithSuite(tracing.NewServerSuite())`) extracts propagation
   headers and opens the server span — exported through whichever pipeline won the §4.2
   decision: starter-otel's global exporter, or the kitex-owned fallback provider
3. the suite also records the `kitex.server.duration` histogram on the OTel meter; with a
   global pipeline present that metric rides it. On an explicitly configured `metrics.port`
   the Prometheus server tracer additionally records kitex's RPC stats on its dedicated
   listener (kitex-contrib/monitor-prometheus metric names — see its docs)
4. compatible-unary middleware (if enabled) adapts the thrift unary call shape
5. your handler runs; the response unwinds: span ended (flushed on Stop),
   stats recorded, error (if any) reflected in both signals

---

## 3. Per-key behavior reference

All keys under `spring.kitex.server.*` (9 total). Only `addr` is required.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** Resolved with `net.ResolveTCPAddr("tcp", addr)`, passed via `server.WithServiceAddr`. | Missing → starter silently inactive. Unresolvable → startup error "failed to resolve addr". |
| `service.name` | string | `kitex` | Advertised in `EndpointBasicInfo`; also the etcd registration name and the OTel `service.name` of the kitex provider. | Wrong name → discovery misses; traces attributed to the wrong service. |
| `registry.etcd` | string | — (empty) | Non-empty → `etcd.NewEtcdRegistry([]string{addr})` + `WithRegistry`; provider publishes on Run, deregisters on `svr.Stop()`. Empty = registry-free direct dial. ⚠ Single address string, not a list. | Set with etcd down → startup error "failed to create etcd registry". |
| `compatible-unary-middleware` | bool | false | Adds `server.WithCompatibleMiddlewareForUnary`. ⚔ Thrift-generated services need `true` — kitex's thrift codegen adds it in its generated NewServer, protobuf codegen does not; this starter builds the server itself so it must be re-enabled here. | thrift + false → unary calls misbehave (kitex compatibility semantics). |
| `tracing.enable` | bool | true | Attaches the kitex tracing suite. ⚠ The key is `enable`, **not** `enabled` (dead key — the example-otel typo is fixed, don't reintroduce it). With a global pipeline (starter-otel) present the suite rides it and no kitex provider is created; without one the starter-owned fallback provider is built. | `tracing.enabled=…` → silently ignored, tracing stays on. `false` → no suite at all — kitex RPC spans/metrics opt out entirely. |
| `tracing.endpoint` | string | 127.0.0.1:4317 | OTLP/gRPC export endpoint of the kitex-owned **fallback** provider. Ignored when a global pipeline is present. | No collector there (fallback posture) → export errors / dropped spans at runtime. |
| `tracing.insecure` | bool | true | Adds `provider.WithInsecure()` (plaintext OTLP) on the fallback provider. Ignored when a global pipeline is present. | false against a plaintext collector → export fails. |
| `metrics.enable` | bool | true | Gates kitex metrics. Same `enable`-not-`enabled` caveat. | `metrics.enabled=…` → silently ignored. |
| `metrics.port` / `metrics.path` | int / string | 0 (unset) / /metrics | A dedicated monitor-prometheus HTTP listener starts **only when port is explicitly configured** (server ports are never defaulted here). With port unset and a global pipeline present, kitex RPC metrics ride it via the tracing suite's `kitex.server.duration` histogram; with neither, metrics stay dormant (INFO log explains how to enable). | Port already bound → the library's `log.Fatal` kills the process at first traffic. |

### 3.1 Dead keys formerly found in example-otel (fixed)

`example-otel/conf/app.properties` used to set `tracing.enabled` / `metrics.enabled` — bound
by nothing (the code binds `enable`). Fixed 2026-08-28: the file now sets the live
`…enable` keys and documents the global-path semantics. Do not reintroduce the `enabled`
spellings.

### 3.2 schema.json divergence (fixed)

`schema.json` used to declare `tracing.enable` / `metrics.enable` default **false** and
`metrics.port` default **9090**, contradicting the code (true / true / 0). Fixed 2026-08-28;
the schema now matches `starter.go` and carries the global-first description.

---

## 4. Verification & fault drills

### 4.1 Log bridge

Importing the starter is enough: `internal/logger`'s `init()` calls `klog.SetLogger` before
any kitex component captures the default logger, so kitex's server wiring, etcd resolver
events, transport errors and your handler's `klog` calls all flow into go-spring's log
pipeline.

- Every bridged line is tagged with the RPC log tag `kitex` (`log.RegisterRPCTag("kitex", "")`)
  — route it with the usual `logger.<tag>` bindings.
- Prefer `klog.CtxInfof(ctx, …)` in handlers: the Ctx path threads your ctx through, so
  go-spring's `FieldsFromContext` hook can lift trace_id/span_id off the request. The plain
  `Info`/`Infof` paths have no ctx and lose trace correlation.
- kitex's `Notice` level folds into go-spring Info; `SetLevel`/`SetOutput` are no-ops —
  go-spring owns filtering and sinks.

Verify: run the example with no `${logging.logger}` sink — kitex framework lines appear on
go-spring's default console.

### 4.2 Tracing and metrics — global-first, one pipeline per process

Observability is **global-first**: the starter probes whether a real OTel TracerProvider is
installed as the process global (`globalTracingActive()` — a probe span's `IsRecording()`;
starter-otel installs the globals in its setup phase, before any bean runs). The decision
matrix:

| Posture | Tracing | Metrics |
|---------|---------|---------|
| starter-otel imported (global pipeline live) | suite attaches to the **global** provider/propagator; `tracing.endpoint`/`insecure` ignored; **no kitex provider created** — no duplicate spans, no second OTLP connection | `kitex.server.duration` rides the global metrics exporter via the suite's otel meter; a dedicated kitex listener starts only on an explicit `metrics.port` |
| kitex only (no global pipeline) | fallback: starter builds its own `OpenTelemetryProvider` exporting OTLP/gRPC to `tracing.endpoint` (zero-config spans; INFO log points at starter-otel) | dedicated Prometheus listener on an explicitly set `metrics.port`; unset → dormant with an INFO hint (no default port is bound) |
| `tracing.enable=false` | no suite at all — the explicit opt-out that drops kitex-side spans/metrics | metrics.enable alone still honors an explicit `metrics.port` |

Migration (2026-08-28): previously the starter always built its own provider and bound
`:9090` by default, so importing starter-otel produced duplicate spans and a port collision.
If you relied on the kitex-owned `:9090`, set `spring.kitex.server.metrics.port=9090`
explicitly (or move scraping to `spring.observability.metrics.port`).

Caveat: a global provider configured with an always-off sampler answers the probe with
"no global pipeline", so the fallback provider would still be built — but that configuration
drops every span by design, so nothing is double-exported.

Verified drill (example-otel demonstrates the global path — starter-otel configured, kitex
attaching to it):

```bash
docker compose -f example-otel/docker-compose.yml up -d   # Jaeger :16686 / OTLP :4317
cd example-otel && go run .                               # 20 RPCs, asserts traces in Jaeger
curl -s :9090/metrics | grep kitex                        # starter-otel's prometheus exporter
```

### 4.3 Registry drill (etcd)

```bash
docker run -d -p 2379:2379 quay.io/coreos/etcd …
# set spring.kitex.server.registry.etcd=127.0.0.1:2379, start with -manual
etcdctl get --prefix "" | grep -A1 echo                   # service key published under service.name
# consumers resolve by the same name; on SIGTERM the entry deregisters (svr.Stop)
```

### 4.4 Lifecycle verification

```bash
go run . -manual &     # log: "kitex server starting on :8888"
kill -TERM %1          # log: "kitex server shutting down on :8888"; etcd entry gone; exit 0
```

`Stop` is graceful: `svr.Stop()` deregisters and drains per kitex semantics, then
`otelProvider.Shutdown(ctx)` flushes pending spans, then `close(done)` unblocks Run.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Server doesn't start, no kitex logs | `spring.kitex.server.addr` missing | Set it — the key is the activation switch (no default :8888 despite the example comment). |
| Startup error "failed to resolve addr" | addr not a resolvable TCP address | Use `:8888` / `host:port` forms `net.ResolveTCPAddr` accepts. |
| Container fails: ServiceRegister missing/ambiguous | app provides zero or 2+ beans | Provide exactly one. |
| Thrift unary calls misbehave | `compatible-unary-middleware=false` | Set true for thrift services. |
| Startup error "failed to create etcd registry" | `registry.etcd` set but etcd unreachable | Start etcd or clear the key (registry-free). |
| `tracing.enabled` / `metrics.enabled` do nothing | dead keys — live keys are `…enable` (§3.1, fixed in the example) | Rename the keys. |
| Duplicate spans in the collector | two SDK providers in one process (e.g. a global provider built outside starter-otel AND the kitex fallback) | Ensure exactly one pipeline owner; with starter-otel present the starter converges automatically (§4.2). |
| :9090 already in use | collision on the explicitly configured kitex `metrics.port` or starter-otel's exporter port | Pick distinct ports / drop the kitex `metrics.port` and ride the global pipeline. |
| No kitex metrics endpoint anymore after upgrade | `metrics.port` no longer defaults to 9090 (§4.2 migration) | Set `spring.kitex.server.metrics.port` explicitly, or scrape `spring.observability.metrics.port`. |
| No traces at all | tracing off, no global pipeline and collector not at `tracing.endpoint` | Check the key name (`enable`), the endpoint, `insecure`. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 9 |
| Required | 1 (`addr`) |
| Quickstart external deps | 0 (etcd only if `registry.etcd` set) |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger) — resolved entries kept for traceability:

1. `tracing.enable`/`metrics.enable` (not `enabled`) — inconsistent with every sibling starter;
   the `…enabled` spellings silently bind nothing (**example-otel typo fixed 2026-08-28**).
2. ~~Default-on `:9090` metrics listener~~ — **fixed 2026-08-28**: `metrics.port` now defaults
   to 0 (unset); the dedicated listener requires an explicit port.
3. ~~Own OTel provider instead of riding starter-otel globals — double pipeline, no dedup
   path~~ — **fixed 2026-08-28 (USAGE_FINDINGS P2 #18)**: global-first convergence; the starter
   attaches to the global pipeline when present and only falls back to its own provider
   otherwise (§4.2). `tracing.enable=false` is the explicit opt-out.
4. example/conf comment claims a ":8888 default" — there is none.
5. No fault/admission/loadtest parity with starter-grpc / starter-trpc.
6. ~~schema.json default divergence~~ — **fixed 2026-08-28**: schema now matches the code
   (enable defaults true, metrics.port default 0) and describes the global-first semantics.
