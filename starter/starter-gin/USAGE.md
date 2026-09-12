# starter-gin Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `middleware.go`, `admission.go`,
`capture.go`, `observe.go`) and the runnable examples: [example/](example/) (self-asserting,
`check.sh`), [example-resilience/](example-resilience/), [example-otel/](example-otel/).
**Gin's own semantics (routing, context, binding, middleware authoring) are
[gin's documentation](https://gin-gonic.com/docs/)** — everything below is go-spring's increment.

**Activation**: the server bean exists only when `spring.gin.server.addr` is set — that key is
the on/off switch; there is no `enabled` key (⚠ the README still mentions one; see §6).
Single-server model: one engine, one port.

---

## 1. Complete worked project

A realistic service exposing gin routes with a health probe, payload-carrying access logs,
tracing/metrics, inbound rate-limit admission and runtime fault injection. File tree:

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
    github.com/gin-gonic/gin    v1.12.0
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-gin   latest
    go-spring.org/starter-governance latest // optional: admission + fault via the governance center
    go-spring.org/starter-otel       latest // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-gin"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**router.go** — the application's entire HTTP surface (mirrors example/example.go):

```go
package router

import (
    "context"
    "net/http"

    "github.com/gin-gonic/gin"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterGin "go-spring.org/starter-gin"
)

func init() {
    // Optional: stamp every business log line with the request id propagated by
    // the starter's RequestID middleware (see §4.1).
    log.FieldsFromContext = func(ctx context.Context) []log.Field {
        if rid := StarterGin.RequestIDFromContext(ctx); rid != "" {
            return []log.Field{log.String("request_id", rid)}
        }
        return nil
    }

    // The application provides exactly ONE RouterRegister bean. The starter owns
    // the *gin.Engine and its HTTP server; you wire routes and app middleware onto
    // the engine it hands you — AFTER the built-in chain is installed (§2.2), so
    // your middleware runs innermost (health route is registered before you).
    gs.Provide(func() StarterGin.RouterRegister {
        return func(e *gin.Engine) {
            e.Use(func(c *gin.Context) { c.Header("X-App", "demo") })
            e.GET("/echo/:name", func(c *gin.Context) {
                c.JSON(http.StatusOK, gin.H{"message": "Hello, " + c.Param("name")})
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented surface actually used above (composed from
the three examples' verified configs):

```properties
# --- gin server ---------------------------------------------------------------
# Let the gin server own the port (disable gs's built-in HTTP server).
spring.http.server.enabled=false
spring.gin.server.addr=:8001

# Liveness endpoint served by the starter itself (registered BEFORE app routes
# so a wildcard route cannot shadow it); path auto-skipped by the access log.
spring.gin.server.health.enabled=true
spring.gin.server.health.path=/healthz

# Timeouts (defaults shown). readTimeout also bounds header read
# (ReadHeaderTimeout reuses it — slowloris protection).
spring.gin.server.readTimeout=5s
spring.gin.server.writeTimeout=5s
spring.gin.server.idleTimeout=60s

# --- middleware ---------------------------------------------------------------
# On by default: LoadTest, RequestID, Observe (Recovery+Tracing+Metrics+AccessLog),
# admission, fault, ResponseCapture. Opt-ins:
spring.gin.server.middleware.secureHeaders.enabled=true

# --- observability (starter-otel) --------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics

# --- governance (inbound admission + fault drills) ----------------------------
# Same keys example-resilience uses: 5 QPS limit → burst shed with 429.
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=5
```

**Verify** (structurally identical to what the examples assert):

```bash
curl -sD- :8001/echo/gin | grep -Ei 'x-app|x-request-id'   # X-App: demo, X-Request-Id: ...
curl -sD- :8001/echo/gin | grep -i x-content-type-options  # nosniff (SecureHeaders)
curl -i :8001/healthz                                     # 200 ok (starter-served)
curl -s :9090/metrics | grep http_server_request_duration
for i in $(seq 1 20); do curl -s -o/dev/null -w '%{http_code}\n' :8001/echo/x; done
# → mix of 200 and 429 once over 5 QPS (admission, §4.4)
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-gin
  └─ init(): gs.Provide(NewSimpleGinServer)  Export gs.As[gs.Server]()
         Condition: gs.OnProperty("spring.gin.server.addr")     [starter.go:34-40]
        │
gs.Run()
  ├─ config bind: ${spring.gin.server} → Config (value tags; gin.SetMode(ReleaseMode) in init)
  ├─ bean wiring: RouterRegister (required) + EngineMiddleware ("?" nullable autowire)
  ├─ NewSimpleGinServer (starter.go:98):
  │    1. outer(e)        — EngineMiddleware hook first → ends up OUTERMOST  [starter.go:104]
  │    2. ApplyMiddlewares — the built-in chain (only if middleware.enabled) [starter.go:109]
  │    3. health route     — registered BEFORE app routes, unshadowable     [starter.go:117]
  │    4. register(e)      — your RouterRegister, innermost                 [starter.go:122]
  │    → CORS misconfig returns error here → container fails fast (no first-request panic)
  ├─ Run(): net.Listen immediately, then <-sig.TriggerAndWait() → serve
  │    (TLS: tls.NewListener with tlsconf.BuildServer — tls.cert-file/key-file pair; tls.ca-file enables mTLS)
  └─ on SIGTERM: Stop → http.Server.Shutdown(ctx) drains in-flight requests
```

Missing `RouterRegister` fails the container (non-nullable); a second one fails with ambiguity.
Two `EngineMiddleware` beans likewise fail (single nullable hook).

### 2.2 Middleware chain — exact order and why

```
[EngineMiddleware] → LoadTest → RequestID → Observe(Recovery+Tracing+Metrics+AccessLog)
→ admission(resilience) → fault → SecureHeaders → CORS → Gzip → ResponseCapture
→ health route → app routes
```

Rationale (from the source comments in `ApplyMiddlewares`, middleware.go:89-174):

- **EngineMiddleware outermost** (optional app hook): auth/trace-context that must precede
  RequestID and Observe without disabling the defaults (starter.go:57-74).
- **LoadTest first of the built-ins**: the marker lands on the request context before anything
  else runs, so every downstream layer — and every outbound client your handler calls — can
  branch on `traffic.IsLoadTest(ctx)`. Single header lookup; no-op without the marker.
- **RequestID before Observe**: the id is on the context from the very start, so Observe reads
  it at any point (not only in its defer); a panic in RequestID itself cannot happen (trivial
  body), handler panics are still recovered by Observe's defer.
- **Observe bundles Recovery+Tracing+Metrics+AccessLog into ONE middleware** so a single
  deferred finalize owns every signal's end-of-request work — on a handler panic the span still
  ends, the in-flight gauge doesn't leak, and one start time/status feeds trace+metric+log
  (observe.go:113-158 documents the two bugs the old separate-middlewares design had).
- **admission inside Observe**: 429/503 rejects are still traced/metered/logged. No retry —
  inbound serving is not idempotent; a `Written()` reentry guard backs that up (admission.go:49-85).
- **fault after admission**: a rate-limited request is not also faulted; injected errors render
  503 and pass Observe on the way out, so you observe the fire you set.
- **SecureHeaders/CORS/Gzip inside Observe**: short-circuit responses (204, 403) are still observed.
- **ResponseCapture innermost**: it wraps the response writer *inside* gzip, so `resp.body` in
  the access log is the UNCOMPRESSED logical body, not compressed wire garbage (capture.go:40-85).

### 2.3 One request, layer by layer

`GET /echo/gin` with header `X-LoadTest: 1`, governance armed with rate-limit + fault:

1. (optional app `EngineMiddleware`, then) LoadTest tags ctx → `traffic.IsLoadTest(ctx)==true`
2. RequestID generates/propagates the id → response header `X-Request-Id`, ctx value for logs
3. Observe: skip-check (path or route pattern), span `{method} {route}` starts, in-flight
   gauge +1 (opt-in), request body tee'd into a bounded buffer (payload capture)
4. admission: `exec.Execute` around the handler — over limit → 429, breaker open → 503;
   handler 5xx feeds the breaker as a failure
5. fault: `fault.Apply(ctx, InjectorFor(), "gin", handler)` — marked traffic at `rate` gets
   the injected error → 503
6. SecureHeaders/CORS/Gzip as enabled; ResponseCapture wraps the writer
7. health route / your route runs; unwind: SSE trailing events finalized, then Observe's defer
   records duration with `http.request.method`/`http.route`/`http.response.status_code`
   (+`http.response.stream=sse` for streams), ends the span, emits the access record
   (severity by status), balances the gauge.

---

## 3. Per-key behavior reference

All keys live under `spring.gin.server.*`. Reconciled against
`grep -rhoE 'value:"[^"]+"' starter-gin --include='*.go'` — no missing, no extra.

### 3.1 Server core

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | **Activation key.** Presence registers the server bean (`OnProperty`). | Missing → whole starter silently inactive. |
| `readTimeout` | duration | 5s | Also reused as `ReadHeaderTimeout` (starter.go:134-136). | Too low kills slow clients mid-header. |
| `writeTimeout` / `idleTimeout` | duration | 5s / 60s | Passed to `http.Server`. | keep-alive churn if idleTimeout too low. |
| `tls.enabled` + `cert-file`/`key-file` | bool/strings | off | Switches `Serve` to a TLS listener built via `tlsconf.BuildServer` (same semantics as starter-grpc). `ca-file` **enables mTLS**: ClientCAs + `RequireAndVerifyClientCert` — clients must present a certificate signed by that CA. `server-name`/`insecure-skip-verify` are client-side keys — bound but dead on a server. | Setting `ca-file` casually → all clients without certs rejected. |
| `health.enabled` / `health.path` | bool / string | false / `/healthz` | Starter-served liveness route, registered before app routes; path auto-appended to access-log skip set (middleware.go:115-117). | Custom path is auto-skipped too — only if health.enabled. |

### 3.2 Middleware groups (`middleware.*`)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `middleware.enabled` | bool | true | Master switch. false = manual mode: starter installs NOTHING (Recovery included) — you call `ApplyMiddlewares`/exported constructors from your RouterRegister. ⚠ an `EngineMiddleware` bean is silently ignored in manual mode. | No recovery → a handler panic crashes the process. |
| `loadtest.enabled` / `.header` | bool / string | true / `X-LoadTest` | Marker header → ctx tag via `traffic.WithLoadTest`. Empty header falls back to the traffic package default. | — |
| `requestId.enabled` / `.header` | bool / string | true / `X-Request-Id` | Self-implemented (uuid v4), NOT gin-contrib/requestid; incoming id honored, echoed on the response, stored on ctx (`RequestIDFromContext`). | — |
| `accessLog.skipPaths` | []string | — | Skips ALL THREE signals (span+metric+log); an entry matches the concrete path OR the gin route pattern (`/users/:id`). A panic overrides the skip. | — |
| `accessLog.payload.enabled` | bool | **true** ⚠ | Captures req body+query+headers and resp body into the access log. On by default — privacy/volume surprise. | Sensitive bodies in logs. |
| `accessLog.payload.limit` | int | 524288 | Per-side cap (req and resp each); record bounded ~2x limit. `expr:"$ > 0"` — must be positive. | 0/negative → startup failure (by design). |
| `accessLog.metrics.sseDistributions` | bool | true | `http.server.sse.event.size` / `.interval` histograms; off = no-op instruments, no per-event branch. | — |
| `accessLog.metrics.activeRequests` | bool | false | `http.server.active_requests` gauge (method+scheme+proto attrs). Opt in for SSE/long-lived streams. | — |
| `cors.enabled` + `allowAllOrigins` / `allowedOrigins` / `allowedMethods` / `allowedHeaders` / `exposeHeaders` / `allowCredentials` / `maxAge` | — | all off / false / — | gin-contrib/cors built at startup with up-front `Validate()` — misconfig fails the boot, not the first request. Empty `allowedMethods` → code default full verb set. ⚠ `allowAllOrigins` vs explicit `allowedOrigins` are mutually exclusive postures. | Invalid combo → container fails fast. |
| `gzip.enabled` / `.level` / `.minLength` | bool/int/int | false / 5 / 0 | compress/gzip semantics (1..9, -1 default); minLength 0 = compress everything. | — |
| `secureHeaders.enabled` | bool | false | When on: `X-Content-Type-Options:nosniff` always; `frameOptions` (DENY), `referrerPolicy` (no-referrer); "" omits. | — |
| `secureHeaders.frameOptions` / `.referrerPolicy` | string | DENY / no-referrer | See above. | — |
| `secureHeaders.hsts.enabled` / `.maxAge` / `.includeSubDomains` / `.preload` | — | off / 0s / false / false | Emitted only when the request is TLS and maxAge>0 (per-request `c.Request.TLS` check). | Set without TLS → header silently absent. |
| `admission` / `fault` | — | — | **No starter keys.** Driven entirely by the governance center (the `govern.*` rules document), resource label `gin::{addr}`, hot-reloaded. | — |

---

## 4. Verification & fault drills

### 4.1 Request id + headers (no external deps — mirrors example/check.sh)

```bash
go run ./example            # self-asserting smoke; or: go run ./example -manual
curl -sD- -o/dev/null :8001/echo/a | grep -i x-request-id    # generated
curl -sD- -o/dev/null -H 'X-Request-Id: fixed-42' :8001/echo/a | grep -i x-request-id  # fixed-42
curl -sD- :8001/echo/a | grep -i x-content-type-options      # nosniff when secureHeaders on
```

With the `log.FieldsFromContext` hook from §1, every business log line carries `request_id`.

### 4.2 Observing the middleware chain

- Access log (tag `_app_gin_access`): one structured record per request —
  `http.request.method`, `url.path`, `http.route`, `http.response.status_code`,
  `http.response.body_size`, `duration_ms`, `request_id`, `trace_id`/`span_id` (when starter-otel
  is live), `req.body`/`req.query`/`req.headers`/`resp.body` (payload capture), `event.count`
  (SSE), `panic`+`stack`. Severity: ≥500 Error, ≥400 Warn, else Info.
- Metrics (Prometheus via starter-otel, example-otel uses :9090):
  `http_server_request_duration_seconds` (buckets 0.005…10s; attrs method/route/status/scheme/
  proto, `error.type`, `http.response.stream=sse`), `http_server_active_requests` (opt-in),
  `http_server_sse_events_total`, `http_server_sse_event_size_bytes`,
  `http_server_sse_event_interval_seconds`:

```bash
curl -s :9090/metrics | grep -E 'http_server_request_duration|http_server_sse'
```

- Traces: server span `{method} {route}` (e.g. `GET /echo/:name`), `sse.event` child spans whose
  duration is the inter-event interval. example-otel verifies end-to-end against Jaeger:

```bash
cd example-otel && docker compose up -d && go run .      # asserts traces in Jaeger API
curl -s '127.0.0.1:16686/api/traces?service=gin-otel-example&limit=1'
```

### 4.3 Probing

```bash
curl -i :8001/healthz    # starter-served liveness, independent of app routes
```

### 4.4 Admission drill (no restart — mirrors example-resilience/check.sh)

1. Start §1's project with `govern.default.rate-limit=5`.
2. Burst: `for i in $(seq 1 20); do curl -s -o/dev/null -w '%{http_code}\n' :8001/echo/x; done`
   → both 200s and 429s (the smoke asserts exactly that). Breaker open → 503
   (`admission.go:79-82`).
3. Tighten at runtime: governance hot-reloads (`ExecutorFor` provider seam) — raise/lower
   `rate-limit` without restart and re-burst.

### 4.5 Fault drill (no restart)

Governance fault keys are hot-toggled (middleware is always installed; `fault.Apply` is a
pass-through when no injector is registered):

```bash
curl -i :8001/echo/x                     # 200 (fault off)
# flip fault on (e.g. rate 0.2) in the governance source
curl -i :8001/echo/x                     # ~20% → 503; access log shows Warn records
```

### 4.6 Load-test marking drill

`curl -H 'X-LoadTest: 1' :8001/echo/x` tags the ctx; handlers branch on
`traffic.IsLoadTest(c.Request.Context())` to degrade features under synthetic load, and
fault/admission can scope to marked traffic.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Server doesn't start, no gin logs | `spring.gin.server.addr` missing | Set it — the key is the activation switch. |
| Port conflict at boot | gs built-in HTTP server owns the port | `spring.http.server.enabled=false` or change `addr`. |
| Container fails: RouterRegister missing/ambiguous | app provides zero or 2+ beans | Provide exactly one. |
| Panic crashes the whole process | `middleware.enabled=false` (manual mode) removed Recovery | Install `Observe` yourself or re-enable the set. |
| Everything works, no traces/metrics | starter-otel not imported | Add it; the OTel hooks are silent no-ops without it. |
| Container fails at boot: "gin: invalid cors config" | mutually exclusive cors posture | Pick `allowAllOrigins` OR explicit `allowedOrigins`. |
| Sensitive request bodies appear in logs | payload capture is ON by default (512 KiB) | `middleware.accessLog.payload.enabled=false`. |
| 429s under modest traffic | governance rate-limit too low for the workload | Raise `govern.default.rate-limit` (hot-reload). |
| Clients rejected at TLS handshake with cert errors | `tls.ca-file` set — that enables **mTLS** (`RequireAndVerifyClientCert`) | Remove it for one-way TLS, or issue client certs. |
| Access log shows garbage `resp.body` under gzip | — should not happen: ResponseCapture sits inside gzip; if you rebuild the chain manually, keep it innermost | In manual mode install `ResponseCapture` innermost, inside any transformer. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 40 starter-local (+7 tls) |
| Required | 1 (`addr`) |
| Quickstart external deps | 0 (collector only for full observability) |
| "Watch out" entries | 6 |

Design suspects (audit ledger; carried over from the previous edition):

1. **Still open** — README inaccuracies: claims a `spring.gin.server.enabled` activation key and
   a default `:8001` (real condition: `addr` presence, no default); stale middleware-order table;
   requestId attribution. Docs-only fix.
2. **Fixed** — the server now uses `tlsconf.BuildServer` (was `ServeTLS` with only
   cert/key files): `ca-file` enables mTLS (`RequireAndVerifyClientCert`), matching
   starter-grpc. `server-name`/`insecure-skip-verify` remain client-side keys with no
   server-side effect.
3. **Still open** — payload capture ON by default (512 KiB bodies into access logs):
   privacy/volume surprise vs. observability convenience.
4. **Still open** — `EngineMiddleware` bean silently ignored under `middleware.enabled=false`.
5. **Still open** — metric instrument creation errors discarded (`_, _ =` in
   `newHTTPMetrics`/`newSSEMetrics`).
6. **Still open** — example-resilience's doc comment and README still cite the nonexistent
   `spring.gin.server.resilience.enabled` key (admission is governance-driven since the
   governance center landed; the conf is correct, the prose is stale).

