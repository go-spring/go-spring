# starter-trpc Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `filter.go`, `fault.go`, `loadtest.go`,
`internal/logger/logger.go`) and the runnable [example/](example/) / [example-otel/](example-otel/).
**tRPC-Go's own semantics** (filters, service config, `trpc_go.yaml` conventions, generated stubs)
are [tRPC-Go's documentation](https://trpc.group/trpc-go/trpc-go) — everything below is
go-spring's increment.

**Activation**: the server bean exists only when `spring.trpc.server.addr` is set — that key is
the on/off switch (no `enabled` key). Unlike stock tRPC-Go there is **no `trpc_go.yaml`**: the
starter builds a `*trpc.Config` programmatically from these properties and calls
`trpc.NewServerWithConfig` (see `SimpleTrpcServer.Run`).

---

## 1. Complete worked project

A realistic tRPC greet service with tracing, metrics and runtime fault injection. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
├── idl/                      # generated: greet.pb.go / greet.trpc.go (trpc-go codegen)
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod** (module deps that matter):

```
require (
    trpc.group/trpc-go/trpc-go   latest
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-trpc   latest
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
    go-spring.org/starter-governance latest // optional: runtime fault injection
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/service"

    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-trpc"
)

func main() { gs.Run() }
```

**service.go** — the application's entire RPC surface:

```go
package service

import (
    "context"

    "go-spring.org/spring/gs"
    StarterTrpc "go-spring.org/starter-trpc"
    greet "demo/idl"
    "trpc.group/trpc-go/trpc-go/server"
)

func init() {
    // The application provides exactly ONE ServiceRegister bean. The starter
    // owns the *server.Server and its lifecycle; you bind your generated
    // service onto the server it hands you.
    gs.Provide(func() StarterTrpc.ServiceRegister {
        return func(s *server.Server) {
            greet.RegisterGreetServiceService(s, &GreetServiceImpl{})
        }
    })
}

type GreetServiceImpl struct{}

func (s *GreetServiceImpl) Greet(ctx context.Context, req *greet.GreetRequest) (*greet.GreetResponse, error) {
    // trpclog calls flow through the go-spring log bridge (see §4.1).
    return &greet.GreetResponse{Greeting: "Hello, " + req.Name + "!"}, nil
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- trpc server -------------------------------------------------------------
# Disable gs's built-in HTTP server; this process exposes only the tRPC endpoint.
spring.http.server.enabled=false

# Activation key: host:port, split into tRPC's IP/Port fields.
spring.trpc.server.addr=127.0.0.1:8000

# Fully-qualified tRPC service name (trpc.app.server.service). Must match the
# callee name baked into the generated client stub.
spring.trpc.server.service.name=trpc.demo.greet.GreetService

# Wire protocol / network (defaults shown).
spring.trpc.server.network=tcp
spring.trpc.server.protocol=trpc

# Starter-owned server filters; all default-on (no-ops without starter-otel).
spring.trpc.server.observer.tracing.enabled=true
spring.trpc.server.observer.metrics.enabled=true
spring.trpc.server.loadtest.enabled=true

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- governance (runtime fault injection) ------------------------------------
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml** (fault drills in §4.4 use this):

```yaml
govern:
  enabled: true
  fault:
    enabled: false        # flip to true to "set fire" without restart
    rate: 0.2
    error: timeout
    scope: loadtest       # only traffic marked x-loadtest is affected
```

**Verify** (mirrors example/check.sh, which is self-asserting):

```bash
go run . -manual          # then from another terminal:
grep -c "trpc server starting" <stdout>   # lifecycle log appeared
# example's built-in check dials ip://127.0.0.1:8000 and asserts the round-trip
./example/check.sh
```

The example's client dials direct-connect (`client.WithTarget("ip://127.0.0.1:8000")`); tRPC
client semantics (target schemes, invocation) are tRPC-Go's own.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-trpc
  └─ (side-effect import) internal/logger init(): trpclog.SetLogger(bridge)   [process-wide,
      before any tRPC component captures the default logger]
  └─ gs.Provide(NewSimpleTrpcServer, IndexArg(0, TagArg("${spring.trpc.server}")))
        .Export(gs.As[gs.Server]())
        .Condition(gs.OnProperty("spring.trpc.server.addr"))
        │
gs.Run()
  ├─ config bind: ${spring.trpc.server} → Config (value tags)
  ├─ bean wiring: SimpleTrpcServer ← the app's ServiceRegister bean (non-nullable)
  ├─ Server.Run():
  │    ├─ SplitHostPort(addr) → IP/Port (both parts required, see §3)
  │    ├─ build *trpc.Config in code (single ServiceConfig; no trpc_go.yaml,
  │    │  no naming/registry plugins — direct-connect starter)
  │    ├─ trpc.NewServerWithConfig(cfg)
  │    ├─ filter.Register: "tracing"/"metrics"/"loadtest" (each gated by config,
  │    │  all default-on) and "fault" (always)
  │    ├─ reg(svr): the app's ServiceRegister binds the generated service
  │    └─ <-sig.TriggerAndWait()        # park until gs signals readiness
  │         go svr.Serve()              # binds listener, blocks
  │         select { Serve-err | <-done }
  ├─ readiness: TriggerAndWait returns → Serve starts
  └─ on SIGTERM: gs calls Stop → svr.Close(nil) unblocks Serve → close(done) → Run returns
```

If `ServiceRegister` is missing the container fails (non-nullable autowire); two
`ServiceRegister` beans fail with ambiguity.

**Signals** (source note on `SimpleTrpcServer`): tRPC's `Serve()` installs its own
SIGINT/SIGTERM/SIGSEGV/SIGUSR2 handlers. They co-exist with gs's lifecycle — gs shutdown calls
`Stop()` → `server.Close(nil)`; a direct SIGTERM is caught by both, harmlessly: whichever fires
first tears the server down.

### 2.2 Server filters — what runs, and in what order

The starter does **not** install a fixed chain. It registers named filters into tRPC's global
filter registry (`filter.Register(name, fn, nil)`); whether they execute — and their order —
follows tRPC's own filter-composition rules, i.e. the filter names referenced by the generated
service code / service config. Per-filter behavior:

| Filter | Registered when | What it does |
|--------|-----------------|--------------|
| `loadtest` | `loadtest.enabled` (default true) | Reads inbound server metadata key `x-loadtest`; if affirmative, tags ctx via `traffic.WithLoadTest(ctx, "trpc-metadata")` so every downstream layer (and your handler, via `traffic.IsLoadTest(ctx)`) can branch. No-op without the marker. Put it **first** so the marker lands before tracing/metrics/handler. |
| `tracing` | `observer.tracing.enabled` (default true) | Starts an OTel server span named `{calleeService}/{method}` with `rpc.system=trpc`, `rpc.service`, `rpc.method` attributes; errors set span status + RecordError. Rides the OTel globals — no-op without starter-otel. |
| `metrics` | `observer.metrics.enabled` (default true) | `rpc.server.request_count` counter, `rpc.server.request.duration` histogram (seconds, explicit buckets 5ms..10s), `rpc.server.active_requests` UpDownCounter, all attributed `rpc.method`. Rides the OTel globals. |
| `fault` | always | `fault.Apply(ctx, fault.InjectorFor(), "trpc", next)` — injects latency/errors per governance rules; transparent pass-through when unconfigured. |

Recommended chain (what the starter's comments prescribe): `loadtest` first, then `tracing`,
`metrics`, `fault`, then the handler — same rationale as the HTTP starters: the marker must be
on the ctx before anything branches on it, and fault sits closest to the handler so injected
errors still flow out through metrics/tracing and remain observable.

### 2.3 One call, layer by layer

Client `Greet("world")` with metadata `x-loadtest: 1`, fault scope `loadtest` engaged:

1. `loadtest` reads `ServerMetaData()["x-loadtest"]` → ctx tagged `traffic.IsLoadTest()==true`
2. `tracing` starts the server span `trpc.demo.greet.GreetService/Greet`
3. `metrics`: active_requests +1, duration observation starts
4. `fault`: `fault.Apply` consults the governance injector; with `scope: loadtest` and the
   marker present, ~`rate` of calls get the injected latency/error; the rest pass through
5. your handler runs; the response unwinds 3→2: counters/histogram recorded with
   `rpc.method`, span ended (status Error + recorded error if the call failed)

---

## 3. Per-key behavior reference

All keys under `spring.trpc.server.*` (8 total). Only `addr` is required.

### 3.1 Server core

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** Presence registers the server bean. Split with `net.SplitHostPort`; host must be present (empty-host `:8000` form fails the split → Run errors). | Missing → whole starter silently inactive. Bare `:port` → startup error "failed to parse addr". |
| `service.name` | string | `trpc.helloworld.greet.GreetService` | Fully-qualified tRPC service name (`trpc.app.server.service` convention). ⚠ The default is the **demo** service from tRPC's helloworld sample — always set it explicitly. | Mismatch with the client stub's callee → client can't route/deserialize (direct-connect dials by address, so it may surface as a protocol/callee mismatch, not a bind error). |
| `network` | string | tcp | Passed to tRPC ServiceConfig.Network. | Non-tcp needs matching transport support in tRPC. |
| `protocol` | string | trpc | Passed to tRPC ServiceConfig.Protocol (wire protocol, not transport). | Client must use the same protocol. |

### 3.2 Filters

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `observer.tracing.enabled` | bool | true | Registers the `tracing` server filter. No-op without starter-otel's globals — nothing warns. | false (or no starter-otel) → no spans, silently. |
| `observer.metrics.enabled` | bool | true | Registers the `metrics` server filter. Same no-op caveat. | false/no starter-otel → no RPC metrics, silently. |
| `loadtest.enabled` | bool | true | Registers the `loadtest` server filter. | false → load-test marker no longer rides this hop; downstream `traffic.IsLoadTest` stays false. |

⚠ **Registration ≠ execution**: filters go into tRPC's global name registry; they run only if
the generated service config references those names (tRPC filter-composition rules). If your
codegen emits its own filter list, add `loadtest`/`tracing`/`metrics`/`fault` to it.

### 3.3 Dead `interceptor.*` keys (fixed)

`example-otel/conf/app.properties` originally set
`spring.trpc.server.interceptor.tracing.enabled` / `...interceptor.metrics.enabled` — a stale
copy from a sibling starter. Nothing in the starter binds any `interceptor.*` prefix (grep over
the module: the only occurrence of "interceptor" is a comment). The example worked only because
the live keys default to true. **Fixed**: the config now uses the live keys
`spring.trpc.server.observer.tracing.enabled` / `spring.trpc.server.observer.metrics.enabled`
(§3.2), and `schema.json` now describes the `observer.*` / `loadtest.*` sub-trees. Do not copy
`interceptor.*` into new configs — it is bound by nothing.

---

## 4. Verification & fault drills

### 4.1 Log bridge

Importing the starter is enough: `internal/logger`'s `init()` calls `trpclog.SetLogger` before
any tRPC component captures the default logger, so tRPC's server wiring, transport errors and
your handler's `trpclog.Infof` calls all flow into go-spring's log pipeline.

- Every bridged line is tagged with the RPC log tag `trpc` (registered via
  `log.RegisterRPCTag("trpc", "")`) — route it to a dedicated logger with the usual
  `logger.<tag>` bindings.
- tRPC's base Logger carries no ctx: bridged lines use `context.Background()` and carry no
  trace_id/span_id on this path (same limitation as the other framework bridges).
- tRPC's `SetLevel`/`With` become no-ops — go-spring owns level filtering and fields.

Verify (example/conf relies on exactly this): with no `${logging.logger}` sink configured, run
the example — tRPC's own log lines appear on go-spring's default console.

```bash
./example/check.sh 2>&1 | grep -i "trpc"   # bridged framework lines on stdout
```

### 4.2 Traces and metrics

With starter-otel imported (example-otel):

```bash
docker compose -f example-otel/docker-compose.yml up -d   # Jaeger :16686 / OTLP :4317
cd example-otel && go run .                               # sends 20 RPCs, self-verifies
open http://localhost:16686    # service "trpc-otel-example": one span per RPC, named
                               # {service}/{method}, attributes rpc.system=trpc
```

Metrics (when exported via starter-otel's Prometheus):

```bash
curl -s :9090/metrics | grep -E 'rpc_server_request_duration|rpc_server_request_count|rpc_server_active_requests'
```

### 4.3 Lifecycle verification

```bash
go run . -manual -conf=conf &   # log: "trpc server starting on 127.0.0.1:8000"
kill -TERM %1                   # log: "trpc server shutting down ..." then process exits 0
```

`Stop` calls `svr.Close(nil)`; tRPC closes its internal closeCh, unblocking Serve. In-flight
requests are **not drained** — see §6.

### 4.4 Fault drill (no restart)

1. Start with `govern.yaml` as in §1 (`fault.enabled: false`), `fault` in the filter chain.
2. Generate baseline traffic → all calls succeed.
3. Flip `fault.enabled: true` in the file — the governance source hot-reloads; the filter
   resolves `fault.InjectorFor()` per call, so no restart is needed.
4. Marked traffic burns (metadata `x-loadtest: 1` on the client call), normal traffic is
   unaffected: ~20% of marked calls get the injected `timeout`.
5. Watch the fire: `rpc.server.request.duration` buckets shift, spans on faulted calls carry
   Error status, `rpc.server.request_count` keeps counting. Flip back to extinguish.

### 4.5 Load-test marking drill

`scope: real` inverts the filter — unmarked traffic burns; use only in dedicated environments.
In your handler, `traffic.IsLoadTest(ctx)` branches on the same marker (set by the `loadtest`
filter from inbound metadata), so business code can degrade features under synthetic load.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Server doesn't start, no trpc logs | `spring.trpc.server.addr` missing | Set it — the key is the activation switch. |
| Startup error "failed to parse addr" | bare `:port` form | Use `host:port` (e.g. `127.0.0.1:8000`); SplitHostPort requires the host field. |
| Container fails: ServiceRegister missing/ambiguous | app provides zero or 2+ beans | Provide exactly one `StarterTrpc.ServiceRegister`. |
| Everything works, no traces/metrics | starter-otel not imported, or `observer.*.enabled=false` | Import starter-otel; the filters ride its OTel globals silently. |
| Filters registered but never run | generated service config doesn't reference the names | Add `tracing`/`metrics`/`loadtest`/`fault` to the service's filter chain (§3.2 ⚠). |
| Client can't call despite direct address | `service.name` mismatch with the stub's callee | Align both sides; don't rely on the demo default. |
| Shutdown hangs or drops in-flight calls | expected: `Close(nil)` without drain | Accept for now; see design suspects §6. |
| Set `spring.trpc.server.interceptor.*` — nothing happens | dead prefix (§3.3) | Use `spring.trpc.server.observer.*`. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 8 |
| Required | 1 (`addr`) |
| Quickstart external deps | 0 (collector for full observability) |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger):

1. `service.name` default is a *sample* service name (`trpc.helloworld.greet.GreetService`) —
   a borrowed demo value, not a sane default for real apps.
2. Filters registered globally by name may not run unless referenced by the generated service
   config — activation semantics are implicit.
3. `addr` must be `host:port`; the bare `:port` form common in the HTTP starters fails.
4. No graceful drain on stop (`svr.Close(nil)`); no resilience admission (rate-limit/breaker),
   unlike starter-grpc.
5. ~~Dead keys: `example-otel/conf`'s `spring.trpc.server.interceptor.*` prefix~~ — FIXED:
   the config now uses the live `spring.trpc.server.observer.*` keys (§3.3).
6. ~~`schema.json` omits the `observer.*` / `loadtest.*` sub-trees~~ — FIXED: both sub-trees
   are now described with their `enabled: true` defaults.
