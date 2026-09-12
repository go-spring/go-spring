# starter-echo Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `middleware.go`, `metrics.go`, `tracing.go`,
`recover.go`, `config.go`) and the runnable [example/](example/). **Echo's own semantics
(routing, context, binding, middleware authoring) are [echo's documentation](https://echo.labstack.com/docs)** —
everything below is go-spring's increment.

**Activation**: the server bean exists only when `spring.echo.server.addr` is set — that key is
the on/off switch; there is no `enabled` key. Single-server model: one engine, one port.

---

## 1. Complete worked project

A realistic service exposing echo routes with health probes, metrics, tracing and runtime fault
injection. File tree:

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
    github.com/labstack/echo/v4  latest
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-echo   latest
    go-spring.org/starter-actuator latest   // optional: probes + /metrics
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
    go-spring.org/starter-governance latest // optional: runtime fault injection
)
```

**main.go**:

```go
package main

import (
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-echo"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go** — the application's entire HTTP surface:

```go
package router

import (
    "context"
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterEcho "go-spring.org/starter-echo"
)

func init() {
    // Optional: stamp every business log line with the request id propagated by
    // the starter's RequestID middleware (see §4.1).
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        if id := StarterEcho.RequestIDFromContext(ctx); id != "" {
            return []log.Field{log.String("request_id", id)}
        }
        return nil
    }

    // The application provides exactly ONE RouterRegister bean. The starter
    // owns the *echo.Echo and its HTTP server; you wire routes and custom
    // middleware onto the engine it hands you — AFTER the built-in chain is
    // installed (see §2.2), so your routes run innermost.
    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
                return func(c echo.Context) error {
                    c.Response().Header().Set("X-App", "demo")
                    return next(c)
                }
            })
            e.GET("/echo/:name", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{"msg": "hi " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- echo server -------------------------------------------------------------
# Let the echo server own the port (disable gs's built-in HTTP server).
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# Liveness endpoint served by the starter; auto-skipped by the access log.
spring.echo.server.health.enabled=true
spring.echo.server.health.path=/healthz

# Reject bodies > 1 MiB with 413 (logged like any response).
spring.echo.server.maxBodySize=1048576

# Timeouts (defaults shown; tune per workload).
spring.echo.server.readTimeout=5s
spring.echo.server.writeTimeout=5s
spring.echo.server.idleTimeout=60s

# --- middleware --------------------------------------------------------------
# On by default: loadtest, recovery, requestId, tracing, metrics, accessLog.
# Opt-ins:
spring.echo.server.middleware.secureHeaders.enabled=true

# --- actuator (probes + metrics mount) --------------------------------------
spring.actuator.addr=:9370

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics via actuator only

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
    scope: loadtest       # only traffic marked X-LoadTest is affected
```

**Verify**:

```bash
curl -i :8002/echo/world        # 200, X-App: demo, X-Request-Id: ...
curl -i :9370/healthz           # actuator liveness
curl -s :9370/metrics | grep http_server_request_duration
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-echo
  └─ gs.Provide(NewSimpleGinServer-equivalent for echo)        [condition: addr set]
        │
gs.Run()
  ├─ config bind: ${spring.echo.server} → Config (value tags, expr validation)
  ├─ bean wiring: RouterRegister autowired — the single app seam. echo has no
  │                 EngineMiddleware outer slot (unlike gin); app middleware runs
  │                 inside the built-in chain.
  ├─ engine assembly: applyMiddlewares() installs the built-in chain
  │                    BEFORE your RouterRegister runs (routes innermost)
  ├─ Server.Run(): net.Listen, then serve
  ├─ readiness: sig.TriggerAndWait() → readyz flips UP
  └─ on SIGTERM: PreStop/Stop → http.Server.Shutdown(ctx) drains in-flight
```

If `RouterRegister` is missing the container fails (non-nullable autowire); two
`RouterRegister` beans fail with ambiguity.

### 2.2 Middleware chain — exact order and why

```
LoadTest → Recovery → RequestID(+propagate) → Tracing → Metrics → AccessLog
→ SecureHeaders → CORS → Gzip → [BodyLimit] → admission → fault → health route → app routes
```

Rationale (from the source comments, verified):

- **LoadTest outermost**: the marker lands on the request context before anything else runs, so
  every downstream layer — and every outbound client your handler calls — can branch on
  `traffic.IsLoadTest(ctx)`. Single header lookup; no-op without the marker.
- **Recovery** catches panics from every later layer and reports through the shared goutil panic
  chain (unified panic policy), not echo's stock Recover alone.
- **RequestID before AccessLog**: every access record carries the request id; the id is also
  stored on the request context (`RequestIDFromContext`) for business-log correlation.
- **Tracing wraps Metrics and AccessLog**: the span captures timing and attributes from both.
- **AccessLog wraps the policy middlewares**: short-circuit responses (413 from BodyLimit, 403
  from CORS, 204) are still logged.
- **admission outside fault**: a request admitted (or rejected) by the inbound rate-limit /
  bulkhead / breaker never also gets faulted, and its 429/503 still passes AccessLog/Tracing/
  Metrics. Installed unconditionally — with governance off the executor is a transparent
  pass-through, so it costs a call frame and changes nothing else. The resource label is
  `echo:<address>` (e.g. `echo::8080`), the same one a govern rule uses:
  `govern.rules[N].resources=echo::8080` with the usual `rate-limit` / `max-concurrent` /
  `error-threshold` knobs. Rejections map to **429** (rate limit, bulkhead full) and **503**
  (circuit open); a handler error or a committed 5xx is fed back to the executor as the call's
  failure, so the breaker sees server-side errors. Inbound admission never retries — a handler
  that already produced side effects cannot be replayed — so leave `max-retries` at 0; a reentry
  guard makes a retrying policy harmless anyway (the handler still runs exactly once).
- **fault innermost**: an injected 503 still passes AccessLog/Tracing/Metrics on the way out —
  you can observe the fire you set. Injected errors (and only those — `*fault.InjectedError`)
  render as 503 "service unavailable"; handler errors pass through to echo's HTTPErrorHandler
  untouched.

### 2.3 One request, layer by layer

`GET /echo/world` with header `X-LoadTest: 1`, fault scope engaged:

1. LoadTest tags ctx (`traffic.IsLoadTest(ctx) == true`)
2. Recovery arms
3. RequestID generates/propagates the id → response header
4. Tracing starts the server span `{method} {route}` (no-op without an OTel provider)
5. Metrics increments in-flight, starts the duration observation
6. AccessLog arms (fields captured on the way out)
7. SecureHeaders/CORS/Gzip as enabled; BodyLimit enforces `maxBodySize`
8. fault: `fault.Apply(ctx, InjectorFor(), "echo", handler)` — with `scope: loadtest` and the
   marker present, ~`rate` of requests get the injected error → 503; others pass through
9. your route runs; the response unwinds through 6→5→4→3: access record logged (severity by
   status), duration recorded with `http.request.method`/`http.route`/`http.response.status_code`
   attributes, span ended, id header set.

---

## 3. Per-key behavior reference

### 3.1 Server core

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** Presence registers the server bean. | Missing → whole starter silently inactive (routes configured but never served). |
| `maxBodySize` | int | 0 | `>0` installs BodyLimit middleware; oversized bodies → 413, logged and recovered like any response. | Too low → 413 on legitimate uploads; error surfaces per-request, not at boot. |
| `readTimeout` | duration | 5s | Also bounds header read. | 0/unset → default; too low kills slow clients mid-header. |
| `writeTimeout` / `idleTimeout` | duration | 5s / 60s | Passed to `http.Server`. | keep-alive churn if idleTimeout too low. |
| `health.enabled` / `health.path` | bool / string | false / `/healthz` | Starter-served liveness route; path auto-merged into the access-log skip set (probes never flood the log). | Custom path must be re-listed in skipPaths only if you disable the merge. |
| `tls.enabled` + `cert-file`/`key-file` | — | off | Switches `Serve` to a TLS listener built via `tlsconf.BuildServer()` (same semantics as starter-grpc). `ca-file` **enables mTLS**: ClientCAs + `RequireAndVerifyClientCert` — clients must present a certificate signed by that CA. `server-name`/`insecure-skip-verify` are client-side keys — bound but dead here. | Setting `ca-file` casually → all clients without certs rejected. |

### 3.2 Middleware groups

Every group has `.enabled`; semantics per layer are in §2.2/§2.3.

| Key | Default | Notes |
|-----|---------|-------|
| `middleware.loadtest.enabled` / `.header` | on / `X-LoadTest` | Marker header name; empty falls back to the traffic package default. |
| `middleware.requestId.enabled` / `.header` | on / `X-Request-Id` | Generated when absent, propagated when present. |
| `middleware.tracing.enabled` / `metrics.enabled` | on / on | No-op without starter-otel's OTel globals — nothing warns. |
| `middleware.accessLog.skipPaths` | — | Merged with the health path. |
| `middleware.accessLog.payload.*` | see gin parity | Body capture — check config.go for the echo-specific defaults; captured bodies land in the log field set. |
| `middleware.cors.*` | off | `allowedMethods` empty → code default full verb set; `allowAllOrigins` vs explicit `allowedOrigins` are mutually exclusive postures. |
| `middleware.gzip.enabled` / `.level` | off / 5 | `minLength`-style tuning where present in config.go. |
| `middleware.secureHeaders.*` | off | frameOptions DENY, referrerPolicy no-referrer; `hsts.*` sub-keys (off). |
| `observability.level` / `maxArgBytes` / `skipOps` | brief / 512 / — | Access-log verbosity for the observe layer. |

---

## 4. Verification & fault drills

### 4.1 Request id propagation

```bash
curl -sD- -o/dev/null :8002/echo/a | grep -i x-request-id   # generated
curl -sD- -o/dev/null -H 'X-Request-Id: fixed-42' :8002/echo/a | grep -i x-request-id  # propagated: fixed-42
```

With the `log.FieldsFromContext` hook from §1, every business log line inside the handler also
carries `request_id`.

### 4.2 Observing the middleware chain

- Access log (tag `_app_echo_access`): one structured record per request — route, status,
  duration, request id, trace/span ids when tracing is live. Severity: ≥500 Error, ≥400 Warn.
- Metrics: `http.server.request.duration` histogram with `http.request.method`,
  `http.route` (the *route pattern*, not the raw path), `http.response.status_code`;
  in-flight gauge with the same method/route attributes. Read them:

```bash
curl -s :9370/metrics | grep -E 'http_server_request_duration|in_flight'
```

- Traces: span named `{method} {route}` per request; check your collector (Jaeger UI etc.)
  after generating traffic.

### 4.3 Probing

```bash
curl -i :9370/readyz     # OUT_OF_SERVICE until ready, UP after; 503 while draining
curl -i :9370/healthz    # the echo-side liveness is on :8002/healthz (starter-served)
```

### 4.4 Fault drill (no restart)

1. Start with `govern.yaml` as in §1 (`fault.enabled: false`).
2. Generate baseline traffic: `curl :8002/echo/x` → 200s.
3. Flip `fault.enabled: true` in the file — the governance source hot-reloads.
4. Marked traffic burns, normal traffic unaffected:

```bash
curl -i :8002/echo/x                          # 200 (scope=loadtest, no marker)
curl -i -H 'X-LoadTest: 1' :8002/echo/x       # ~20% → 503 service unavailable
```

5. Watch the fire in the observables: access-log Warn records with status 503, the duration
   histogram's 503 bucket, spans on the faulted requests. Flip back to false to extinguish.

### 4.5 Load-test marking drill

With `scope: real` instead of `loadtest`, the injector affects unmarked traffic — use only in
dedicated environments. `traffic.IsLoadTest(ctx)` in your handler branches on the same marker,
so business code can degrade features under synthetic load.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Server doesn't start, no logs | `spring.echo.server.addr` missing | Set it — the key is the activation switch. |
| Port conflict at boot | gs HTTP server or another starter owns the port | `spring.http.server.enabled=false` or change `addr`. |
| Container fails: RouterRegister missing/ambiguous | app provides zero or 2+ beans | Provide exactly one. |
| Everything works, no traces/metrics | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| 413 on uploads | `maxBodySize` < payload | Raise or unset. |
| Probes flood the access log | custom health path | It is auto-skipped only for the starter-served path; add yours to `skipPaths`. |
| App middleware must run outside the built-in chain | echo has no EngineMiddleware slot (unlike gin) | `RouterRegister` runs innermost; every built-in group is toggled by its own `middleware.<group>.enabled`, there is no master `middleware.enabled` and no exported `ApplyMiddlewares`. |
| Clients rejected at TLS handshake with cert errors | `tls.ca-file` set — that enables **mTLS** (`RequireAndVerifyClientCert`) | Remove it for one-way TLS, or issue client certs. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | ~37 |
| Required | 1 (`addr`) |
| Quickstart external deps | 0 (collector for full observability) |
| "Watch out" entries | 7 |

Design suspects (for the audit ledger): ~~no resilience admission (rate limit/breaker)~~ — now
installed, mirroring gin (see §2 for the label and status mapping); TLS now uses `BuildServer()` so
`ca-file` enables mTLS (fixed — was `Build()`, which
ignored it); no outer `EngineMiddleware` slot (gin has one — echo's app middleware runs inside the
built-in chain); no dedicated request-body-capture config parity documentation vs gin.
