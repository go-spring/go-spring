# starter-actuator Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`actuator.go`, `probes.go`, `endpoints.go`, `beans.go`) and the
self-asserting [example/](example/) (`example/check.sh`). Kubernetes probe semantics are the
[kubelet's contract](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/);
Spring Boot Actuator naming analogies link to
[its documentation](https://docs.spring.io/spring-boot/reference/actuator/endpoints.html) —
everything below is go-spring's increment.

**Activation**: the actuator server bean exists only when `spring.actuator.addr` is set —
that key is the on/off switch; there is no `enabled` key (actuator.go:93-95, `gs.OnProperty`).
Single management-port model: probes, introspection and contributed endpoints (e.g. `/metrics`)
all share one dedicated port, distinct from your application's HTTP port.

---

## 1. Complete worked project

A realistic service with an echo business server, a togglable dependency health indicator,
actuator probes on a management port, and Prometheus metrics mounted on that same port.
File tree:

```
demo/
├── go.mod
├── main.go
├── router.go
├── health.go
└── conf/
    ├── app.properties
    └── govern.yaml
```

**go.mod** (module deps that matter):

```
require (
    github.com/labstack/echo/v4  latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-echo       latest
    go-spring.org/starter-actuator   latest   // probes + introspection on :9370
    go-spring.org/starter-otel       latest   // optional: real metric export, /metrics mount
    go-spring.org/starter-governance latest   // optional: runtime fault injection
)
```

**main.go**:

```go
package main

import (
    _ "demo/health"
    _ "demo/router"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-echo"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**health.go** — the whole health-contribution story. Any bean exported as
`health.Indicator` is folded into the probes with zero per-component wiring; the interface
lives in `cloud/actuator/health`, so your component never imports this starter:

```go
package health

import (
    "context"
    "errors"
    "sync/atomic"

    "go-spring.org/cloud/actuator/health"
    "go-spring.org/spring/gs"
)

// DepDown stands in for "the dependency is broken" (a dead pool, a dropped
// connection). An admin route flips it so the §4 drills can force DOWN without
// killing anything.
var DepDown atomic.Bool

// Optional-cache example: NonCritical means a DOWN result shows per-component
// but keeps readyz at 200 DEGRADED instead of 503.
var CacheDown atomic.Bool

func init() {
    // Critical (default): readiness+startup groups by default.
    gs.Provide(health.NewIndicator("mysql:orders", func(ctx context.Context) error {
        if DepDown.Load() {
            return errors.New("dependency unavailable")
        }
        return nil // your real probe: e.g. db.PingContext(ctx)
    })).Export(gs.As[health.Indicator]()).Name("mysql-orders-indicator")

    gs.Provide(health.NewIndicator("redis:cache", func(ctx context.Context) error {
        if CacheDown.Load() {
            return errors.New("cache unavailable")
        }
        return nil
    }, health.NonCritical())).Export(gs.As[health.Indicator]()).Name("redis-cache-indicator")
}
```

**router.go** — business routes plus the admin toggles used by the §4 drills:

```go
package router

import (
    "net/http"

    "github.com/labstack/echo/v4"
    "go-spring.org/spring/gs"

    healthpkg "demo/health"
    StarterEcho "go-spring.org/starter-echo"
)

func init() {
    gs.Provide(func() StarterEcho.RouterRegister {
        return func(e *echo.Echo) {
            e.GET("/orders/:id", func(c echo.Context) error {
                return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
            })
            // Drill hooks (§4): flip the demo indicators from outside.
            e.POST("/admin/dep/:state", func(c echo.Context) error {
                healthpkg.DepDown.Store(c.Param("state") == "down")
                return c.NoContent(http.StatusOK)
            })
            e.POST("/admin/cache/:state", func(c echo.Context) error {
                healthpkg.CacheDown.Store(c.Param("state") == "down")
                return c.NoContent(http.StatusOK)
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented actuator-relevant surface:

```properties
# --- echo business server ----------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# --- actuator (this starter) -------------------------------------------------
# Activation key: presence registers the management server. No default — bind
# all interfaces so in-cluster K8s probes can reach it. Layout convention:
# main HTTP :9090, actuator :9370, pprof 127.0.0.1:9981.
spring.actuator.addr=:9370

# Sensitive introspection endpoints (loggers, env, configprops, threaddump,
# beans) are default-off; list them in include to expose them. /info and
# contributed endpoints (e.g. metrics) are default-on; probes are ALWAYS
# registered.
spring.actuator.endpoints.include=loggers,env,configprops,threaddump
spring.actuator.endpoints.exclude=

# Optional auth for the whole management port: bearer token (takes precedence)
# or HTTP Basic. Without either, binding all interfaces logs a WARN.
spring.actuator.token=
spring.actuator.username=
spring.actuator.password=

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics served by actuator only

# --- logging (gives /loggers something to list) ------------------------------
logging.logger.root.type=Logger
logging.logger.root.level=INFO
```

**conf/govern.yaml** (not actuator-specific; included for the full-stack posture):

```yaml
govern:
  enabled: true
  fault:
    enabled: false
    rate: 0.2
    error: timeout
```

**Verify**:

```bash
curl -i :8002/orders/7          # 200 business route
curl -i :9370/healthz           # 200 {"status": "UP"}
curl -i :9370/readyz            # 200 UP (with components, see §2.3)
curl -i :9370/startupz          # 200 after startup completes
curl -i :9370/info              # build/version metadata
curl -i :9370/metrics           # contributed by starter-otel, same port
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-actuator
  └─ gs.Provide(&Server{})  [condition: spring.actuator.addr set]  (actuator.go:93-95)
        │   exports gs.Server under a distinct name — coexists with the app's
        │   main HTTP server, which also exports gs.Server
gs.Run()
  ├─ config bind: addr / endpoints.include / endpoints.exclude /
  │               token / username / password (value tags)
  ├─ bean wiring: []health.Indicator, []endpoint.Endpoint,
  │               *gs.PropertiesRefresher, BeanLister — all autowire:"?" optional
  ├─ Server.Run(): net.Listen, mux registration + httpauth.Guard.Wrap,
  │               WARN when unauthenticated && non-loopback, SERVING IMMEDIATELY
  │     (before app readiness — a readiness probe must observe the
  │      OUT_OF_SERVICE → UP transition; actuator.go:22-25, 265-273)
  ├─ readiness: sig.TriggerAndWait() → goroutine sets s.ready once ALL servers
  │     (including this one) report ready (actuator.go:269-273)
  ├─ on SIGTERM: PreStop(ctx) sets draining=true  (actuator.go:305-307)
  │     → /readyz flips to 503 OUT_OF_SERVICE while the server KEEPS SERVING,
  │       so the endpoint controller drains the pod before Stop()
  └─ StopContext(ctx): http.Server.Shutdown(ctx) rides the shutdown context
```

Why serving precedes readiness (source comment, actuator.go:21-25): "a readiness probe must
be able to reach the endpoint *before* the app is ready so it can observe the
OUT_OF_SERVICE -> UP transition, and a liveness probe must answer throughout a long startup
so the pod is not killed prematurely."

### 2.2 Endpoint registration order

```
probe routes:      /healthz /readyz /startupz (+ /health /readiness /startup aliases)
                   — registered UNCONDITIONALLY (actuator.go:228-235); the source
                   comment at actuator.go:169-171: filtering them "would break the
                   Kubernetes contract"
introspection:     info, loggers, env, configprops, threaddump, beans — each passes
                   the include/exclude filter first (actuator.go:238-243)
contributed:       every []endpoint.Endpoint bean, name = path without "/",
                   same filter (actuator.go:251-258); registered AFTER built-ins so
                   a contributor cannot shadow /health — a duplicate path panics
                   ServeMux at startup, surfacing the misconfiguration fast
```

### 2.3 One readiness request, end to end

`GET /readyz` after startup, both indicators registered, mysql up, redis down:

1. `s.ready.Load()` true, `s.draining.Load()` false — else immediate
   `503 {"status":"OUT_OF_SERVICE"}` (probes.go:120-126).
2. `checkGroup(GroupReadiness)` sweeps every indicator whose groups contain readiness.
   Group membership: explicit `HealthGroups()`, else the default
   **readiness + startup, never liveness** (probes.go:34-39 — so a dependency check can
   never trigger a pod restart).
3. The sweep carries a **3s total timeout** (`checkTimeout`, actuator.go:98-100): one slow
   dependency cannot stall the probe past a typical kubelet timeout; a late indicator sees
   an expired ctx and reports DOWN with the deadline error.
4. Per indicator: `CheckHealth(ctx)` nil → `components[name] = {status: UP}`;
   error → `{status: DOWN, error: err}` (probes.go:74-83).
5. Aggregation (probes.go:55-62, 85-88): every UP → `UP`; only non-critical DOWN →
   `DEGRADED`; any critical DOWN → `DOWN`.
6. `writeProbe`: HTTP 503 **only** when DOWN; DEGRADED keeps 200 — "the app is still
   serving traffic, only a tolerable dependency is failing" (probes.go:91-94).

```json
{
  "status": "DEGRADED",
  "components": {
    "mysql:orders": {"status": "UP"},
    "redis:cache":  {"status": "DOWN", "error": "context deadline exceeded"}
  }
}
```

`/healthz` runs the same sweep over the **liveness** group only — with no indicator
declaring liveness (the normal case) it is trivially `200 {"status":"UP"}` while the
process serves. `/startupz` additionally gates on `s.ready` and is unaffected by drain
(probes.go:131-135: once startup succeeds the kubelet stops polling it).

### 2.4 Introspection handlers (exact shapes)

| Endpoint | Response shape | Source |
|----------|----------------|--------|
| `GET /info` | `{"go","module":{"path","version"},"build":{"revision","time","modified"}}` from `debug.ReadBuildInfo` | actuator.go:312-336 |
| `GET /loggers` | `{"loggers": {"root": {"configuredLevel": "INFO"}, ...}}` — read-only; `POST /loggers/{name}` is intentionally not implemented | endpoints.go:54-62 |
| `GET /env` | `{"propertySources": [{"name", "properties": {key: {"value": masked}}}]}` — sources in priority order (highest first), **unmerged** | endpoints.go:64-84 |
| `GET /configprops` | array of `{"name", "config": <nested tree>}` — same sources, tree view, same masking | endpoints.go:90-100 |
| `GET /threaddump` | `text/plain` goroutine dump at debug detail 2 | endpoints.go:104-108 |
| `GET /beans` | `{"beans": [{"name","type"}]}` or, with no `BeanLister` bean, `{"beans": [], "note": "bean registry not available..."}` | beans.go:51-63 |
| contributed (e.g. `/metrics`) | owned entirely by the contributor (Prometheus text format for starter-otel) | cloud/actuator/endpoint |

Masking (endpoints.go:29-71): a value becomes `"******"` when the **key** matches the
case-insensitive regex `password|passwd|secret|token|credential|api-?key|private-key|access-key`
(substring, so `spring.datasource.password` and `auth.access-key` both hit), or when the key's
final `.`/`_`/`-`-separated segment is exactly `key` / `api-key` / `api_key` / `apikey`
(end-anchored with a leading separator, so `some.key` and `aws.key` hit while `monkey`,
`keyword`, and `keynote` do not), or when the **value** is an `ENC(...)` placeholder from
config-encryption. A value that is a URL with embedded credentials
(`scheme://user:pass@host`, incl. empty-user `scheme://:pass@host`) has only the userinfo
part redacted (`scheme://******@host`), keeping the rest of the URL readable. Everything
else passes through verbatim.

---

## 3. Per-key behavior reference

Complete: these three are every `value` tag in the starter (cross-checked with
`grep -rhoE 'value:"[^"]+"'`).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.actuator.addr` | string | — | **Activation key + bind address.** Presence registers the `gs.Server` bean (actuator.go:93-95). No default by policy (starter server ports must be user-configured); documented layout `:9370`, all interfaces so in-cluster probes reach it. | Missing → whole starter silently inactive (no probes, no `/metrics` mount). Bad address → `Run` fails with `actuator: failed to listen on <addr>`. |
| `spring.actuator.endpoints.include` | string (comma list) | `""` | Non-empty = **whitelist mode**: only named introspection/contributed endpoints register. Names are paths without the leading slash: `info, loggers, env, configprops, threaddump, beans`, plus a contributor's own path (e.g. `metrics`). Matching is exact, case-insensitive, whitespace-trimmed. ⚠ **Sensitive endpoints are default-off**: `loggers, env, configprops, threaddump, beans` register ONLY when explicitly listed here, even when the list is otherwise empty. ⚠ **Probe endpoints are exempt** — `/healthz` etc. register regardless. | Typo'd name → that endpoint silently off (only a Debug log); forgetting `metrics` in the list drops your only metrics port. **Migration (2026-08-28)**: configs that relied on the old default-on introspection must now add e.g. `spring.actuator.endpoints.include=env,configprops,loggers,threaddump,beans`. |
| `spring.actuator.endpoints.exclude` | string (comma list) | `""` | **Blacklist, always applies — including inside a whitelist and even over an explicit include of a sensitive endpoint** (test `TestEndpointFilter_ExcludeBeatsSensitiveInclude`). Same name syntax. | Excluding `metrics`/`info` you actually scrape; silent (Debug log only). |
| `spring.actuator.token` | string | `""` | Bearer-token auth for the WHOLE management port (shared `stdlib/httpauth` guard): every request needs `Authorization: Bearer <token>`. Takes precedence over username/password. Constant-time compare. | Wrong/missing header → 401 on every endpoint incl. probes (K8s probes then need the header too — or keep the actuator loopback-only). |
| `spring.actuator.username` | string | `""` | HTTP Basic auth together with `spring.actuator.password` (both must be set; token wins if also set). Failures get `401` + `WWW-Authenticate: Basic`. | Half-configured pair (only one of the two set) → guard stays disabled; non-loopback listener then logs a WARN. |
| `spring.actuator.password` | string | `""` | See `spring.actuator.username`. | See `spring.actuator.username`. |

No key interacts with the main HTTP server: the actuator always owns its own port. There is
no TLS or path-prefix key — see §5/§6. When no auth scheme is configured and the address is
non-loopback, startup logs a WARN (`listening on %q without authentication`).

---

## 4. Verification & fault drills

All drills assume the §1 project is running (`go run .`).

### 4.1 Baseline probes

```bash
curl -s :9370/readyz | jq .status          # "UP" (or "DEGRADED" if redis is down)
curl -s :9370/healthz                      # {"status": "UP"}
curl -s :9370/startupz | jq .status        # "UP" after startup
```

During startup you can catch `curl -i :9370/readyz` → `503 {"status": "OUT_OF_SERVICE"}`
before the readiness barrier crosses (the example's self-test asserts this sequence).

### 4.2 Critical indicator DOWN → readyz 503, healthz stays 200

```bash
curl -i -X POST :8002/admin/dep/down       # flip the critical mysql:orders indicator
curl -i :9370/readyz                       # 503 {"status":"DOWN","components":{"mysql:orders":{"status":"DOWN","error":"..."}}}
curl -i :9370/healthz                      # 200 {"status":"UP"} — a degraded dependency
                                           # must not trip liveness (probes.go:107-110)
curl -i -X POST :8002/admin/dep/up         # recover → readyz 200 UP again
```

### 4.3 Non-critical indicator DOWN → DEGRADED, still 200

```bash
curl -i -X POST :8002/admin/cache/down     # redis:cache is NonCritical
curl -s :9370/readyz | jq '.status, .components."redis:cache".status'
# "DEGRADED"        <- HTTP 200: still serving (probes.go:91-94)
# "DOWN"
curl -i -X POST :8002/admin/cache/up
```

### 4.4 Drain flip (graceful shutdown)

```bash
kill -TERM <pid>                           # or Ctrl+C
# immediately, repeatedly:
curl -s -o /dev/null -w '%{http_code}\n' :9370/readyz   # 503 OUT_OF_SERVICE
curl -s -o /dev/null -w '%{http_code}\n' :9370/healthz  # 200 — probes stay answerable
                                                          # through the drain window
```

`PreStop` sets `draining` **before** `Stop` shuts the server down (actuator.go:299-307), so
Kubernetes removes the pod from Service endpoints while in-flight requests finish. The
example's `runTest` automates exactly this sequence (example.go, SIGTERM + poll loop).
Note `/startupz` is unaffected by drain by design (probes.go:131-135).

### 4.5 Endpoint filtering

```properties
spring.actuator.endpoints.include=info,env,metrics
spring.actuator.endpoints.exclude=configprops
```

```bash
curl -i :9370/info        # 200 (default-on, also in include)
curl -i :9370/metrics     # 200 (contributed endpoint in whitelist)
curl -i :9370/env         # 200 (sensitive: explicit include is REQUIRED)
curl -i :9370/configprops # 404 (sensitive AND excluded)
curl -i :9370/threaddump  # 404 (sensitive, not included — the default)
curl -i :9370/readyz      # 200 — probes exempt from the filter
```

With no auth configured and a non-loopback addr, boot also logs:
`WARN actuator listening on ":9370" without authentication; set ${spring.actuator.token} ...`.

### 4.6 Auth

```properties
spring.actuator.token=s3cret           # or:
spring.actuator.username=admin
spring.actuator.password=pw
```

```bash
curl -i :9370/healthz                   # 401
curl -i -H 'Authorization: Bearer s3cret' :9370/healthz      # 200
curl -i -u admin:pw :9370/info          # 200 (Basic, when no token set)
```

### 4.7 Secret masking

```bash
curl -s :9370/env | grep -E 'password|token|key|redis'
# "demo.datasource.password": {"value": "******"}
# "demo.api.token":          {"value": "******"}
# "demo.aws.key":            {"value": "******"}   # final segment == "key"
# "demo.redis.url":          {"value": "redis://******@127.0.0.1:6379/0"}
curl -s :9370/configprops | grep ENC      # nested tree, same "******" leaves
```

### 4.8 Slow dependency bound

The whole readiness sweep shares one 3s budget (actuator.go:100). Wire an indicator whose
`CheckHealth` sleeps 5s → `readyz` returns within ~3s with that component
`DOWN "context deadline exceeded"`, not a probe timeout.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No actuator endpoints at all | `spring.actuator.addr` missing | Set it — the key is the activation switch (actuator.go:93-95). |
| Boot fails: `actuator: failed to listen on :9370` | port in use (another actuator, pprof overlap) | Change `addr` or free the port. |
| `/readyz` stuck at 503 OUT_OF_SERVICE after startup | readiness barrier not crossed — some other `gs.Server` never signaled ready | Check which server is blocking readiness; the actuator flips `s.ready` only when ALL servers report ready (actuator.go:269-273). |
| `/readyz` 503 DOWN, `/healthz` 200 | a critical readiness indicator failing — this is correct behavior | Read `components` in the body for the failing dependency and its error string. |
| `/metrics` 404 | starter-otel not imported, or `metrics` not in a non-empty `endpoints.include` | Import starter-otel; when whitelisting, remember contributed endpoints. |
| Introspection endpoint 404, nothing in logs | filtered out — sensitive endpoints are default-off and filtering logs at Debug only | Add the endpoint to `endpoints.include` (case-insensitive exact name). |
| Boot panic: `http: multiple registrations for /...` | a contributed `endpoint.Endpoint` path collides with a built-in or another contributor | Change the contributor's `Path()`; ServeMux panics at registration (actuator.go:249-253) — fail-fast by design. |
| `/env` shows more sources than expected / "wrong" value | `/env` is per-source and **unmerged**, highest priority first | Judge aggregation yourself across sources (endpoints.go:64-67); use `/configprops` for the tree view. |
| Secret-looking value visible in `/env` | key doesn't match the masking rules (`demo.license`, `demo.keystore-id` pass through; URL userinfo IS masked but credentials in query strings are not) | Rename the key or file a design issue — masking is regex-on-key + URL-userinfo-on-value (endpoints.go:29-71). |
| Every endpoint answers 401 | `spring.actuator.token` (or Basic pair) configured — the guard covers probes too | Present the header/token in probes and scrapers, or unset the keys and keep the listener loopback-only. |
| Probe intermittently 503 DOWN with `context deadline exceeded` | one indicator eating the shared 3s sweep budget | Make `CheckHealth` honor ctx / tighten the dependency check. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 |
| Required | 1 (`addr`, doubles as the switch) |
| Quickstart external deps | 0 |
| "Watch out" entries | 7 (probe exemption from filtering AND from auth-off WARN only covering non-loopback; sensitive endpoints default-off; Debug-only filter logs; unmerged /env; /beans needs a contributor; auth covers probes too; shared 3s sweep budget) |

Design suspects (for the audit ledger):

1. ~~Dead key `spring.actuator.enabled`~~ — FIXED 2026-08-27: removed from READMEs, DESIGN,
   and ~20 example configs.
2. ~~Stale `POST /loggers` docs~~ — FIXED 2026-08-27: README_CN/DESIGN now state read-only;
   phantom `"levels"` field removed.
3. ~~`:9370` default claims~~ — FIXED 2026-08-27: docs/comments now say "no default; the key
   activates the starter".
4. ~~`/env` documented as merged~~ — FIXED 2026-08-27: documented as per-source unmerged.
5. ~~No security layer on an all-interfaces management port~~ — FIXED 2026-08-28:
   `spring.actuator.token` / `.username` / `.password` guard the whole port via the shared
   `stdlib/httpauth` guard (same pattern as starter-pprof), and the sensitive introspection
   endpoints are default-off unless explicitly included in `endpoints.include`.
6. ~~Masking regex misses generic secret-ish key names and credentials embedded in
   values~~ — FIXED 2026-08-28: final-segment bare `key`/`api-key` keys are masked
   (boundary-anchored: `aws.key` yes, `monkey`/`keyword` no) and URL-embedded userinfo is
   redacted (`scheme://******@host`).
7. Shared 3s `checkTimeout` for the whole sweep means many indicators shrink each one's
   budget and a slow one starves the rest (no per-indicator timeout). → candidate:
   per-indicator bound.
8. ~~Example conf POST comment~~ — FIXED 2026-08-27.
9. `/beans` can never list out of the box (gs core does not export bean enumeration,
   beans.go:29-43) — the endpoint ships as a boundary note. Acceptable, but users should
   not expect Spring-parity here.
