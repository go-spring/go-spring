# starter-hertz Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `middleware.go`, `metrics.go`, `tracing.go`,
`recover.go`, `config.go`) and the runnable [example/](example/) (self-asserting `check.sh`,
no external services) plus [example-otel/](example-otel/) (docker-compose Jaeger).
**Hertz's own semantics (routing, `app.RequestContext`, hertz-contrib) are
[Hertz's documentation](https://www.cloudwego.io/docs/hertz/)** — everything below is
go-spring's increment.

**Activation**: the server bean exists only when `spring.hertz.server.addr` is set — that key
is the on/off switch; there is no `enabled` key. Single-server model. Unlike gin/echo, Hertz
owns its listener: the starter passes the address via `WithHostPorts`, and the read/write/idle
timeouts plus `maxBodySize` via engine options (`server.New(opts...)`, not a standard
`http.Server`, not a middleware — `NewSimpleHertzServer`).

---

## 1. Complete worked project

A realistic service exposing hertz routes with a health endpoint, metrics, tracing and runtime
fault injection. File tree:

```
demo/
├── go.mod
├── main.go
├── router.go
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod** (module deps that matter):

```
require (
    github.com/cloudwego/hertz   v0.10.x
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-hertz  latest
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
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-hertz"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go** — the application's entire HTTP surface (handler signature is hertz's
`func(ctx context.Context, c *app.RequestContext)`):

```go
package router

import (
    "context"
    "net/http"

    "github.com/cloudwego/hertz/pkg/app"
    "github.com/cloudwego/hertz/pkg/app/server"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterHertz "go-spring.org/starter-hertz"
)

func init() {
    // Optional: stamp every business log line with the request id propagated by
    // the starter's RequestID middleware (see §4.1, propagateRequestID).
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        if rid := StarterHertz.RequestIDFromContext(ctx); rid != "" {
            return []log.Field{log.String("request_id", rid)}
        }
        return nil
    }

    // The application provides exactly ONE RouterRegister bean. The starter
    // owns the *server.Hertz and its listener; you wire routes and app-specific
    // middleware onto the engine it hands you — AFTER the built-in chain is
    // installed (see §2.2), so your middleware runs innermost.
    gs.Provide(func() StarterHertz.RouterRegister {
        return func(h *server.Hertz) {
            h.Use(func(ctx context.Context, c *app.RequestContext) {
                c.Response.Header.Set("X-App", "demo")
                c.Next(ctx)
            })
            h.GET("/echo/:name", func(ctx context.Context, c *app.RequestContext) {
                c.JSON(http.StatusOK, map[string]string{"message": "hi " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- hertz server -------------------------------------------------------------
# Hertz drives its own listener: disable gs's built-in HTTP server first.
spring.http.server.enabled=false
spring.hertz.server.addr=127.0.0.1:8003

# Starter-served liveness endpoint; auto-skipped by the access log.
spring.hertz.server.health.enabled=true
spring.hertz.server.health.path=/healthz

# Cap request bodies at 1 MiB via WithMaxRequestBodySize (413 from the engine).
spring.hertz.server.maxBodySize=1048576

# Timeouts (defaults shown; engine options, tune per workload).
spring.hertz.server.readTimeout=5s
spring.hertz.server.writeTimeout=5s
spring.hertz.server.idleTimeout=60s

# --- middleware --------------------------------------------------------------
# On by default: loadtest, recovery, requestId, tracing, metrics, accessLog.
# (No master middleware.enabled switch here — one key per group.)
# Opt-ins:
spring.hertz.server.middleware.secureHeaders.enabled=true

# --- actuator (probes + metrics mount) --------------------------------------
spring.actuator.addr=:9370

# --- observability (starter-otel) -------------------------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
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
    scope: loadtest       # only traffic marked X-LoadTest is affected
```

**Verify** (structurally identical to what `example/check.sh` asserts — X-App header,
X-Request-Id header, JSON body, /healthz "ok"):

```bash
cd example && ./check.sh                          # self-asserting smoke, exit 0
curl -i 127.0.0.1:8003/echo/world                 # 200, X-App: demo, X-Request-Id: ...
curl -i 127.0.0.1:8003/healthz                    # 200 "ok" (starter-served)
curl -s 127.0.0.1:9090/metrics | grep http_server_request_duration
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-hertz
  └─ gs.Provide(NewSimpleHertzServer, IndexArg(1, TagArg(${spring.hertz.server})))
        │  .Export(gs.As[gs.Server]()), Condition(OnProperty("spring.hertz.server.addr"))
gs.Run()
  ├─ config bind: ${spring.hertz.server} → Config (value tags)
  ├─ bean wiring: the app's single RouterRegister autowired (non-nullable)
  ├─ NewSimpleHertzServer:
  │    engine opts (WithHostPorts/Read/Write/Idle/MaxRequestBodySize[/WithTLS])
  │    → server.New(opts...)           // NOT server.Default: Recovery stays configurable
  │    → applyMiddlewares(h, cfg)      // built-in chain installed here
  │    → health route (before register, so a wildcard route can't shadow it)
  │    → register(h)                   // your routes, innermost
  ├─ SimpleHertzServer.Run(ctx, sig): <-sig.TriggerAndWait() → h.Run() blocks
  ├─ readiness: TriggerAndWait returns → readyz flips UP → engine starts serving
  └─ SIGTERM: Stop/StopContext → h.Shutdown(ctx) drains in-flight requests
```

If `RouterRegister` is missing the container fails (non-nullable autowire); two
`RouterRegister` beans fail with ambiguity. Note the engine only starts serving **after** the
readiness signal — `Run` blocks on `sig.TriggerAndWait()` first.

### 2.2 Middleware chain — exact order and why

```
LoadTest → Recovery → RequestID(+propagate) → Tracing → Metrics → AccessLog
→ SecureHeaders → CORS → Gzip → fault → health route → app routes
```

Rationale (from the `applyMiddlewares` comment, verified):

- **LoadTest outermost**: the marker lands on the request context before anything else runs,
  so every downstream layer — and every outbound client your handler calls — can branch on
  `traffic.IsLoadTest(ctx)`. Single header lookup (`Header.Peek`); no-op without the marker.
- **Recovery** (starter-owned `Recover()`, not hertz's contrib middleware) catches panics from
  every later layer and reports through the shared goutil panic chain (unified panic policy),
  then aborts with 500.
- **RequestID before AccessLog**: every access record carries the request id; the id is also
  stored on the request context (`propagateRequestID` + `RequestIDFromContext`) for
  business-log correlation.
- **Tracing wraps Metrics and AccessLog**: the span captures timing and attributes from both.
- **AccessLog wraps the policy middlewares**: short-circuit responses (CORS 403, 204) are
  still logged. Body limiting is the engine option `WithMaxRequestBodySize`, so an over-limit
  413 is likewise logged.
- **fault innermost, always installed**: an injected 503 still passes
  AccessLog/Tracing/Metrics on the way out — you can observe the fire you set. In `buildFault`
  the 503 is only written when the response is untouched (`!IsBodyStream() && StatusCode()==0`);
  handler-produced responses pass through unchanged.

### 2.3 One request, layer by layer

`GET /echo/world` with header `X-LoadTest: 1`, fault scope engaged:

1. LoadTest tags ctx (`traffic.WithLoadTest(ctx, "http-header")`)
2. Recovery arms (`defer`/`recover`)
3. RequestID: hertz-contrib/requestid generates/propagates the id → response header;
   `propagateRequestID` copies it onto the request ctx
4. Tracing extracts caller context, starts the server span `HTTP <method>` (no-op without an
   OTel provider)
5. Metrics increments the in-flight gauge, starts the duration observation
6. AccessLog arms (fields captured on the way out)
7. SecureHeaders/CORS/Gzip as enabled (engine enforces `maxBodySize` independently)
8. fault: `fault.Apply(ctx, fault.InjectorFor(), "hertz", handler)` — with `scope: loadtest`
   and the marker present, ~`rate` of requests get the injected error → 503; others pass
9. your route runs; the response unwinds through 6→5→4→3: access record logged (severity by
   status), duration/count recorded, in-flight decremented, span ended and marked errored on
   5xx, id header set.

---

## 3. Per-key behavior reference

All keys under `spring.hertz.server.*` (37 leaves incl. tls). Reconciled against
`grep -rhoE 'value:"[^"]+"' --include='*.go'`.

### 3.1 Server core

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** `OnProperty("spring.hertz.server.addr")` registers the server bean; passed via `WithHostPorts`. | Missing → whole starter silently inactive. |
| `readTimeout` | duration | 5s | Engine option `WithReadTimeout`. | Too low kills slow clients mid-request. |
| `writeTimeout` / `idleTimeout` | duration | 5s / 60s | `WithWriteTimeout` / `WithIdleTimeout`. | keep-alive churn if idleTimeout too low. |
| `maxBodySize` | int | 0 | `>0` → `WithMaxRequestBodySize`; over-limit bodies rejected by the engine with 413, logged like any response. 0 = Hertz default. | Too low → 413 on legitimate uploads; per-request, not at boot. |
| `health.enabled` | `health.path` | bool / string | false / `/healthz` | Starter-served liveness `GET path → "ok"`; registered **before** app routes so a wildcard route cannot shadow it; path auto-merged into the access-log skip set (`accessLogSkipSet`). | Custom path is skipped only if health.enabled stays true. |
| `tls.enabled` + `tls.cert-file`/`key-file` | — | off | `cfg.TLS.BuildServer()` → `WithTLS` (same semantics as starter-grpc). `tls.ca-file` **enables mTLS**: ClientCAs + `RequireAndVerifyClientCert` — clients must present a certificate signed by that CA. `server-name`/`insecure-skip-verify` are client-side keys — bound but **dead here**. | Setting `ca-file` casually → all clients without certs rejected. |

### 3.2 Middleware groups

No master `middleware.enabled` switch — each group has its own `.enabled` (unlike echo/gin).
Semantics per layer are in §2.2/§2.3.

| Key | Default | Notes |
|-----|---------|-------|
| `middleware.loadtest.enabled` / `.header` | on / `X-LoadTest` | Marker header name; empty falls back to `traffic.HeaderLoadTest`. |
| `middleware.recovery.enabled` | on | Off = a request-goroutine panic crashes the whole process (hertz core behavior). |
| `middleware.requestId.enabled` | on | hertz-contrib/requestid default header `X-Request-Id`; generated when absent, propagated when present. ⚠ no configurable header key (unlike loadtest). |
| `middleware.tracing.enabled` / `metrics.enabled` | on / on | No-op without starter-otel's OTel globals — nothing warns. |
| `middleware.accessLog.enabled` / `.skipPaths` | on / — | skip list merged with the health path. |
| `middleware.cors.enabled` + 7 sub-keys (`allowAllOrigins`, `allowedOrigins`, `allowedMethods`, `allowedHeaders`, `exposeHeaders`, `allowCredentials`, `maxAge`) | all off/false/empty | `allowedMethods` empty → code default full verb set (`corsMiddleware`); config validated at startup via `c.Validate()` — a bad policy fails boot with `hertz: invalid cors config` instead of panicking on first request. `allowAllOrigins` and explicit `allowedOrigins` are mutually exclusive postures. |
| `middleware.gzip.enabled` / `.level` | off / 5 | Level follows compress/gzip semantics: 1=BestSpeed … 9=BestCompression, -1=DefaultCompression. ⚠ no minLength tuning key. |
| `middleware.secureHeaders.enabled` | off | Stamps X-Content-Type-Options: nosniff, X-Frame-Options: DENY, Referrer-Policy: no-referrer (self-implemented; hertz-contrib/secure defaults — 10-year HSTS + SSL redirect — deliberately avoided). |
| `middleware.secureHeaders.hsts.enabled`/`.maxAge`/`.includeSubDomains`/`.preload` | off / 0s / off / off | Header emitted **only when** hsts on AND tls.enabled AND maxAge>0 (`secureHeaders` fn). |

---

## 4. Verification & fault drills

### 4.1 Request id propagation

```bash
curl -sD- -o/dev/null 127.0.0.1:8003/echo/a | grep -i x-request-id   # generated
curl -sD- -o/dev/null -H 'X-Request-Id: fixed-42' 127.0.0.1:8003/echo/a | grep -i x-request-id  # propagated: fixed-42
```

With the `log.FieldsFromContext` hook from §1, every business log line inside the handler also
carries `request_id` (example-otel's handler demonstrates the pattern).

### 4.2 Observing the middleware chain

- Access log (tag `_app_hertz_access`, registered via `log.RegisterAppTag("hertz","access")`):
  one structured record per request — `method`, `path`, `status`, `size`, `ip`, `latency`,
  `request_id`. Severity: ≥500 Error, ≥400 Warn, else Info.
- Metrics (meter `go-spring.org/starter-hertz`):
  - counter `http.server.request_count`
  - histogram `http.server.request_duration` (seconds; OTel HTTP semconv buckets
    0.005…10) — attributes `http.request.method`, `http.route`, `http.response.status_code`
  - updown gauge `http.server.active_requests` — attributes method + `http.route`.
  ⚠ `http.route` here is the **raw request path** (`c.Request.URI().Path()`), not the route
  pattern — high-cardinality, diverges from starter-echo/gin. Read them:

```bash
curl -s 127.0.0.1:9090/metrics | grep -E 'http_server_request_(count|duration)|active_requests'
```

- Traces (tracer `go-spring.org/starter-hertz`): span named **`HTTP <method>`** (e.g.
  `HTTP GET`) per request — not `{method} {route}` like echo. Attributes:
  `http.request.method`, `url.path`, `server.address`, `http.response.status_code`; ≥500 sets
  span status Error. example-otel verifies end-to-end against Jaeger's API:

```bash
cd example-otel && docker compose up -d && go run .
# asserts: traces found in Jaeger for service 'hertz-otel-example'
```

### 4.3 Probing

```bash
curl -i 127.0.0.1:8003/healthz   # starter-served liveness "ok", auto-skipped by access log
curl -i 127.0.0.1:9370/readyz    # actuator readiness (add starter-actuator)
```

### 4.4 Fault drill (no restart)

1. Start with `govern.yaml` as in §1 (`fault.enabled: false`).
2. Generate baseline traffic: `curl 127.0.0.1:8003/echo/x` → 200s.
3. Flip `fault.enabled: true` in the file — the governance source hot-reloads.
4. Marked traffic burns, normal traffic unaffected:

```bash
curl -i 127.0.0.1:8003/echo/x                          # 200 (scope=loadtest, no marker)
curl -i -H 'X-LoadTest: 1' 127.0.0.1:8003/echo/x       # ~20% → 503 service unavailable
```

5. Watch the fire in the observables: access-log Error records with status 503, the duration
   histogram's 503 bucket, errored spans (`HTTP GET`). Flip back to false to extinguish.

### 4.5 Load-test marking drill

With `scope: real` instead of `loadtest`, the injector affects unmarked traffic — use only in
dedicated environments. `traffic.IsLoadTest(ctx)` in your handler branches on the same marker,
so business code can degrade features under synthetic load.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Server doesn't start, no logs | `spring.hertz.server.addr` missing | Set it — the key is the activation switch. |
| Port conflict at boot | gs HTTP server still on | `spring.http.server.enabled=false` (Hertz owns its listener). |
| Container fails: RouterRegister missing/ambiguous | app provides zero or 2+ beans | Provide exactly one. |
| Server never serves, readyz stuck | `Run` blocks on readiness signal by design | Check other beans' readiness; engine starts only after `TriggerAndWait`. |
| Everything works, no traces/metrics | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| Boot fails with `hertz: invalid cors config` | incompatible cors sub-keys (e.g. allowAllOrigins + allowedOrigins/credentials) | Fix the policy — validation is at startup by design (`corsMiddleware`). |
| Uploads 413 | `maxBodySize` < payload | Raise or unset (engine option, not middleware). |
| Clients rejected at TLS handshake with cert errors | `tls.ca-file` set — that enables **mTLS** (`RequireAndVerifyClientCert`) | Remove it for one-way TLS, or issue client certs. |
| Metrics cardinally exploding by path | `http.route` attr is the raw path | ⚠ known divergence from echo/gin; raise as a design issue. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 37 (incl. 5 dead tls keys) |
| Required | 1 (`addr`) |
| Quickstart external deps | 0 (collector for full observability) |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger):

1. **README stale claim (partially fixed)** — the middleware table now lists
   loadtest/tracing/metrics/fault, but the trailing note "Metrics and tracing are not built in
   either - use starter-actuator and starter-otel for those" is wrong: both ship as default-on
   middlewares (`metrics.go`, `tracing.go`). Still unfixed at doc time.
2. `http.route` metric attribute is the raw request path, not the route pattern — unbounded
   cardinality and inconsistent with starter-echo/gin (`metrics.go` `metricsMiddleware`).
3. Tracing span name `HTTP <method>` carries no route — spans of different endpoints are
   indistinguishable (`tracing.go` `tracingMiddleware`).
4. Fixed: the server now uses `tlsconf.BuildServer()` (was `Build()`, client semantics) —
   `ca-file` enables mTLS (`RequireAndVerifyClientCert`), matching starter-grpc;
   `server-name`/`insecure-skip-verify` remain client-side keys with no server effect.
5. No resilience admission (rate limit/breaker) on the inbound path — asymmetric with
   starter-gin; fault injection is wired, protection is not.
6. `requestId` group has no configurable header key (loadtest does) — minor asymmetry.
7. No master `middleware.enabled` switch, unlike echo/gin — per-key toggles only (arguably
   safer; recorded for cross-family consistency review).
