# starter-goframe Usage — Reference

Detailed usage reference for the umbrella starter covering all four sub-servers (`http/`,
`grpc/`, `tcp/`, `ws/`) plus the shared `internal/logger` bridge. Overview:
[README.md](README.md). All behavior claims are verified against the starter sources
(`http/starter.go`, `grpc/starter.go`, `tcp/starter.go`, `ws/starter.go`,
`internal/logger/logger.go`) and the runnable examples (`http/example`, `http/example-otel`,
`grpc/example` + `idl/`, `grpc/example-otel`, `tcp/example`, `ws/example`).
**goframe's own semantics (`ghttp`, `grpcx`, `gtcp`, routing, middleware, WebSocket) are
[goframe's documentation](https://goframe.org/docs/)** — everything below is go-spring's
increment: lifecycle wiring, activation keys, etcd registration, log bridge, metrics placement.

**Activation**: each sub-server's bean exists only when its `address` key is set
(`spring.goframe.<proto>.server.address`, `gs.OnProperty`) — that key is the on/off switch;
there is no `enabled` key. The application must additionally provide exactly one
`ServiceRegister` bean of the matching shape. Single instance per protocol per process.

---

## 1. Complete worked project

A realistic service running the http sub-server with etcd discovery, native metrics,
starter-otel tracing, and actuator probes. File tree:

```
demo/
├── go.mod
├── main.go
├── router.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/gogf/gf/v2              v2.x
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-goframe      latest   // module root; import sub-packages
    go-spring.org/starter-actuator     latest   // optional: probes
    go-spring.org/starter-otel         latest   // optional: real trace export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/router"

    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-goframe/http"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go** — the application's entire HTTP surface:

```go
package router

import (
    "github.com/gogf/gf/v2/net/ghttp"
    "go-spring.org/spring/gs"

    goframehttp "go-spring.org/starter-goframe/http"
)

func init() {
    // The application provides exactly ONE ServiceRegister bean per activated
    // sub-server. For http it receives the response-wrapping RouterGroup that
    // HTTPServer created — business routes MUST be bound inside it so goframe's
    // MiddlewareHandlerResponse JSON envelope applies (see §2.2).
    gs.Provide(func() goframehttp.ServiceRegister {
        return func(group *ghttp.RouterGroup) {
            group.ALL("/hello", func(r *ghttp.Request) {
                r.Response.Writeln("Hello World!")
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- goframe http server ------------------------------------------------------
# Let the goframe server own the port (disable gs's built-in HTTP server).
spring.http.server.enabled=false

spring.goframe.http.server.name=goframe-http
spring.goframe.http.server.address=:8000

# Publish into etcd for discovery. ⚠ process-global (see §3.1 registry.etcd).
# spring.goframe.http.server.registry.etcd=127.0.0.1:2379

# Native goframe OTel Prometheus endpoint, served by this SAME server.
# On by default; /metrics sits at the server root, outside the response envelope.
spring.goframe.http.server.metrics.enabled=true
spring.goframe.http.server.metrics.path=/metrics

# --- actuator (probes) --------------------------------------------------------
spring.actuator.addr=:9370

# --- observability (starter-otel): tracing only here ---------------------------
# ⚠ leave spring.observability.metrics.* unset while the goframe-native metrics
# endpoint is on — both set the global OTel MeterProvider (see §3.1 metrics note).
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
```

**Verify** (mirrors `http/example/check.sh` and `http/example-otel`):

```bash
curl -i :8000/hello          # 200, body "Hello World!"
curl -s :8000/metrics | head # goframe-native Prometheus exposition (valid text format)
curl -i :9370/healthz        # actuator liveness
```

The grpc/tcp/ws siblings follow the same shape — see the runnable examples:
`grpc/example` (generated `echo.RegisterEchoServiceServer` onto `grpc.ServiceRegistrar`,
proto in `idl/echo.proto`), `tcp/example` (line-echo handler via `s.SetHandler`),
`ws/example` (`r.WebSocket()` upgrade route bound on the raw `*ghttp.Server`).

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-goframe/<proto>
  └─ gs.Provide(New<Proto>Server, TagArg config arg)               [condition: address key set]
        │    .Export(gs.As[gs.Server]())
  └─ blank-import internal/logger: glog → go-spring log bridge self-installs
        in init() — BEFORE any server is built, so the very first framework
        log line (including gsvc registration and startup errors) is bridged.

gs.Run()
  ├─ config bind: ${spring.goframe.<proto>.server} → Config (value tags)
  ├─ bean wiring: your ServiceRegister bean autowired into New<Proto>Server
  ├─ construction:
  │    ├─ http/grpc/ws: if registry.etcd set → gsvc.SetRegistry(etcdreg.New(...))
  │    │    BEFORE g.Server(name) / grpcx.Server.New — ghttp/grpcx snapshot
  │    │    gsvc.GetRegistry() at construction time (ordering is load-bearing;
  │    │    see the comments in NewHTTPServer/NewGRPCServer/NewWSServer)
  │    ├─ http only: initMetrics → Prometheus exporter + otelmetric provider,
  │    │    provider.SetAsGlobal(), handler bound at server root
  │    ├─ http only: svr.Group("/") { MiddlewareHandlerResponse; reg(group) }
  │    ├─ ws: reg(svr) on the raw server (no envelope group, by design)
  │    └─ tcp: gtcp.NewServer(addr, nil); reg(s) attaches handler via SetHandler;
  │         registry pre-created but NOT yet called
  ├─ Run(): <-sig.TriggerAndWait() (start only after Go-Spring signals readiness)
  │    ├─ http/ws: svr.Start() — non-blocking listen (+ etcd register if set)
  │    ├─ grpc: svr.Start() — NOT grpcx's Run(), which installs its own gproc
  │    │    signal handler that would fight Go-Spring's; Start + park-on-done
  │    │    keeps shutdown owned by the Go-Spring lifecycle (source comment)
  │    └─ tcp: svr.Run() in a goroutine (blocking Accept loop, error forwarded
  │         via runErr channel); poll GetListenedPort() up to 5s; then Register
  │         into etcd under advertise.host:port — bind first, register second
  └─ on SIGTERM: Stop() →
       ├─ http: svr.Shutdown() (deregisters from etcd) → metricStop(ctx) flush
       ├─ grpc: svr.Stop() (deregisters + grpc.Server.GracefulStop)
       ├─ ws: svr.Shutdown()
       └─ tcp: Deregister FIRST (no new consumers pick a dying instance up),
            then stopping.Store(true), then svr.Close() — the `stopping` flag
            makes the Run goroutine swallow the expected
            "use of closed network connection" Accept error
```

If your `ServiceRegister` bean is missing, the server bean's second ctor argument fails to
autowire and the container errors at startup — the sub-server cannot start half-wired.

### 2.2 Route placement — why the register beans differ per protocol

| Sub-server | ServiceRegister signature | Placement rationale (from source comments) |
|------------|---------------------------|--------------------------------------------|
| http | `func(group *ghttp.RouterGroup)` | Business controllers must sit inside the `MiddlewareHandlerResponse` group (goframe's JSON response envelope); the starter owns the group so the envelope cannot be forgotten. `/metrics` is deliberately bound at the server root so the Prometheus exposition is not wrapped in the envelope. |
| grpc | `func(s grpc.ServiceRegistrar)` | Wraps the generated `RegisterXxxServiceServer`; the adapter stays service-agnostic. |
| tcp | `func(s *gtcp.Server)` | Handler attached via `s.SetHandler`; gtcp has no routing/middleware notion. |
| ws | `func(s *ghttp.Server)` | **Raw server on purpose**: a WebSocket upgrade route must not sit under the response-wrapping middleware — the 101 Switching Protocols handshake and the frame stream cannot pass through goframe's JSON envelope. |

Note (http): `MiddlewareHandlerResponse` leaves an already-written response buffer
untouched — that is why the example writes with `r.Response.Writeln` and gets a plain
body, not a JSON-wrapped one.

### 2.3 One startup, step by step (http + etcd)

1. Package init: log bridge installed on both `glog.SetDefaultHandler` (fresh
   `glog.New()` loggers) and `g.Log().SetHandlers` (the process-wide singleton —
   per-logger handlers take precedence, so both surfaces must be covered).
2. Config bound; `registry.etcd` non-empty → `gsvc.SetRegistry(etcdreg.New(...))`.
3. `g.Server(name)` constructs the server and **snapshots** the global registry as its
   registrar; `SetAddr(address)`.
4. Metrics: Prometheus exporter → `otelmetric.MustProvider` with built-in metrics →
   `SetAsGlobal()`; `BindHandler("/metrics", otelmetric.PrometheusHandler)` at root.
5. Your `ServiceRegister` runs inside the `Group("/", ...)` closure → routes registered.
6. Readiness signal fires → `svr.Start()`: background listener + etcd Register.
7. `Run` parks on `done` until `Stop`; SIGTERM → `Shutdown()` (deregister) →
   metric provider flush → `close(done)`.

---

## 3. Per-key behavior reference

### 3.1 Keys (all four sub-servers)

`spring.goframe.http.server.*` — activation: `address`

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `address` | string | — | **Activation key.** Presence registers the server bean (OnProperty). | Missing → whole sub-starter silently inactive. |
| `name` | string | `goframe` | ghttp server name / etcd service name. | Same name as another ghttp instance in-process → goframe singleton collision. |
| `registry.etcd` | string | `""` | Empty = no registration. Non-empty calls `gsvc.SetRegistry(etcdreg.New(...))` **process-globally** before construction. ⚠ Two goframe sub-servers with different etcd registries cannot coexist — last construction wins; a registered + an unregistered one also interact (the second with empty etcd does not reset the global). | Cross-server registry bleed: the "unregistered" grpc server may register into the http server's etcd. |
| `metrics.enabled` | bool | `true` | Installs goframe-native OTel Prometheus pull endpoint on this same server. ⚠ `provider.SetAsGlobal()` — cannot be unified with starter-otel's metrics pipeline; if both configure metrics, they fight over the global MeterProvider. | Duplicate/conflicting global meter provider; disable one side. |
| `metrics.path` | string | `/metrics` | Bound at server root, outside the response envelope. | A path colliding with a business route → one of them shadows the other. |

`spring.goframe.grpc.server.*` — activation: `address`: `address` (required), `name`
(default `goframe`), `registry.etcd` (same global-registry semantics; grpcx snapshots at
construction). 3 keys, 1 required.

`spring.goframe.ws.server.*` — activation: `address`: identical key set to grpc; the
register bean takes the raw `*ghttp.Server`. 3 keys, 1 required.

`spring.goframe.tcp.server.*` — activation: `address`

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `address` | string | — | **Activation key**; gtcp bind address. | Missing → sub-starter inactive. |
| `name` | string | `goframe` | etcd service name for the manual registration. | — |
| `advertise.host` | string | `127.0.0.1` | Endpoint published into etcd. ⚠ Only used when `registry.etcd` set; gtcp never detects a public IP, so you must name the dialable address. ⚠ The default silently disagrees with whatever you actually bind. | Consumers dial 127.0.0.1 from another host → connection refused. |
| `advertise.port` | int | `8003` | Same ⚠ as host — must match your real `address` port or clients dial the wrong port. | Wrong-port endpoints in etcd. |
| `registry.etcd` | string | `""` | gtcp has **no** gsvc integration; the starter registers by hand in `Run` (bind-first ordering) and deregisters in `Stop` (deregister-before-close). | — |

No other keys exist: no timeouts, no TLS, no middleware switches — those belong to
goframe's own configuration, which this starter deliberately does not proxy. There are no
`${observability:=}`-style wrapper fields anywhere in this starter.

---

## 4. Verification & fault drills

### 4.1 Basics (mirrors the examples' self-checks)

```bash
# http (http/example)
curl -i http://127.0.0.1:8000/hello            # 200, "Hello World!"

# grpc (grpc/example): use grpcurl / a client against :8001
grpcurl -plaintext -d '{"message":"hello"}' 127.0.0.1:8001 echo.EchoService/Echo

# tcp (tcp/example): line echo
printf 'ping\n' | nc 127.0.0.1 8003            # -> ping

# ws (ws/example)
websocat ws://127.0.0.1:8002/echo              # type ping, get ping back
```

### 4.2 Native metrics endpoint (http)

```bash
curl -s :8000/metrics | head          # valid Prometheus text, NOT JSON-wrapped
curl -s :8000/metrics | grep -c '^#'  # goframe built-in metric families present
```

Because the handler sits at the server root, the exposition is plain Prometheus text —
if you ever see a JSON envelope there, the handler got bound inside the group (a bug,
not a config).

### 4.3 Tracing via starter-otel (http/example-otel, grpc/example-otel)

```bash
cd http/example-otel && docker compose up -d     # Jaeger with OTLP enabled
go run .                                          # sends 20 requests, checks Jaeger
# then: http://localhost:16686 — service "goframe-http-otel-example"
curl -s 'http://127.0.0.1:16686/api/traces?service=goframe-http-otel-example&limit=1'
```

ghttp/grpcx auto-instrument off the global OTel TracerProvider that starter-otel
installs — no per-server key, and no warning if starter-otel is absent (silent no-op).

### 4.4 Log bridge drill

Any goframe framework log — server lifecycle lines, gsvc registration errors, your own
`g.Log().Info(...)` — lands in the go-spring JSON pipeline under tag `_rpc_goframe`
(registered via `log.RegisterRPCTag("goframe", "")`; the rendered tag string carries the
leading underscore, see `log.BuildTag`). Level folding: glog `NOTI`→Info, `CRIT`→Fatal
(`PANI`/`FATA` prefix markers also mapped). Structured extras passed as glog `Values`
are kept whole as one `"values"` field — glog has no k/v contract on that slice.
glog's own stdout/file output is fully suppressed (the handler never calls `Next`).

Verify: start the http example, grep the app's log output for `goframe http server
starting` — it must appear in your go-spring pipeline, not on raw stdout.

### 4.5 Shutdown drill (tcp ordering)

Send SIGTERM to the tcp example with `registry.etcd` set and watch the sequence in the
logs: deregister from etcd → listener close → process exit, with no
"use of closed network connection" error logged (the `stopping` flag filters it).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Server doesn't start, no goframe logs | `spring.goframe.<proto>.server.address` missing | Set it — it is the activation switch. |
| Container fails: ServiceRegister missing/ambiguous | app provides zero or 2+ beans of the sub-server's register type | Provide exactly one. |
| Port already in use | gs built-in HTTP server still owns it | `spring.http.server.enabled=false`. |
| No traces | starter-otel not imported | Blank-import it; instrumentation is a silent no-op without it. |
| `/metrics` returns JSON | handler bound inside the envelope group (custom modification) | Keep it at the server root; re-check metrics.path. |
| Two sub-servers both write to etcd A/B unexpectedly | `gsvc.SetRegistry` is process-global, last construction wins | Use one etcd registry per process, or separate processes. |
| MeterProvider conflict / duplicated metrics | both goframe-native metrics and starter-otel metrics enabled | Disable one: `metrics.enabled=false` or drop `spring.observability.metrics.*`. |
| etcd consumers can't reach the tcp server | `advertise.host/port` left at 127.0.0.1:8003 defaults | Set them to the dialable endpoint matching `address`. |
| "use of closed network connection" spam on shutdown | non-starter gtcp usage, or the stopping flag bypassed | Expected only as a filtered-internal detail; if user-visible, report as a bug. |
| Framework logs appear twice (go-spring + raw stdout) | a package replaced the glog handlers after init | Re-install via the bridge or avoid `glog.Set*` in app code. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 (http) / 3 (grpc) / 3 (ws) / 5 (tcp) |
| Required | 1 per sub-server (`address`) |
| Quickstart external deps | 0 (etcd optional; collector for full observability) |
| "Watch out" entries | 6 |

Design suspects (carried from the previous edition, for the audit ledger):

- ⚠ `gsvc.SetRegistry` is a **process-global mutation** — two goframe sub-servers with
  different etcd registries cannot coexist (last construction wins); even one-registered
  + one-not is not cleanly expressible. Candidate: per-server registrar injection.
- ⚠ http metrics calls `provider.SetAsGlobal()` — collides with starter-otel's metrics
  pipeline if both are configured. Candidate: document-or-detect, or an explicit
  opt-out default.
- tcp `advertise.port` default (8003) silently disagrees with whatever `address` you
  actually bind. Candidate: default to the bind port.
- ~~http metrics "cannot be unified with starter-otel's pipeline"~~ — **FIXED** (documented
  + verified): behavior is now an explicit ⚠ in §3.1/§5 with the disable-one-side remedy;
  the structural unification remains blocked by the otel boundary.
- `http/example/conf/app.properties` still comments "native metrics stay off by default",
  but the code default is `metrics.enabled=true` — stale example comment, candidate fix.
- grpc/tcp/ws have no metrics parity with http (grpcx/gtcp would need their own wiring) —
  asymmetry, candidate for follow-up.
