# starter-actuator Usage & Design — Reference

The detailed reference. For a first look, read the [README](README.md) first. Everything
here is checked against the source (`actuator.go`, `probes.go`) and the self-asserting
[example/](example/). Kubernetes probe semantics are whatever the
[kubelet](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
does; the Spring Boot Actuator names are just familiar anchors.

**How it turns on**: the server bean is registered only when `spring.actuator.addr` is
set. That key is the switch — there is no `enabled` key. Everything (probes, `/info`,
contributed endpoints like `/metrics`) lives on that one port, separate from your
business port.

---

## 1. A complete worked project

A service with an echo business server, a togglable dependency health indicator, probes
on a management port, and Prometheus metrics on the same port.

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

**go.mod** (the deps that matter):

```
require (
    github.com/labstack/echo/v4  latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-echo       latest
    go-spring.org/starter-actuator   latest   // probes on :9370
    go-spring.org/starter-otel       latest   // optional: /metrics on the same port
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

**health.go** — this is the whole health story. Export a bean as `health.Indicator` and
it lands in the probe aggregate automatically. The interface lives in
`cloud/actuator/health`, so your component never imports this starter:

```go
package health

import (
    "context"
    "errors"
    "sync/atomic"

    "go-spring.org/cloud/actuator/health"
    "go-spring.org/spring/gs"
)

// DepDown fakes "the dependency is broken". An admin route flips it so the §3
// drills can force DOWN without killing anything.
var DepDown atomic.Bool

// CacheDown: a NonCritical indicator — DOWN shows up per-component but readyz
// stays 200 DEGRADED instead of 503.
var CacheDown atomic.Bool

func init() {
    // Critical by default: counts for readiness+startup.
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

**router.go** — business routes plus the admin toggles the §3 drills use:

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
            // Drill hooks (§3): flip the demo indicators from outside.
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

**conf/app.properties** — every actuator-relevant key, commented:

```properties
# --- echo business server ----------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8002

# --- actuator (this starter) -------------------------------------------------
# The switch: setting this key registers the management server. Bind all
# interfaces so in-cluster K8s probes can reach it. Port layout convention:
# main HTTP :9090, actuator :9370, pprof 127.0.0.1:9981.
spring.actuator.addr=:9370

# Optional auth for the whole port: bearer token (wins) or HTTP Basic.
# Neither + non-loopback address = startup WARN.
spring.actuator.token=
spring.actuator.username=
spring.actuator.password=

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics served by actuator only
```

**conf/govern.yaml** (not actuator-specific, just part of the full picture):

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
  └─ gs.Provide(&Server{})  [only if spring.actuator.addr is set]
        │   exports gs.Server under its own name, so it coexists with the app's
        │   main HTTP server (which also exports gs.Server)
gs.Run()
  ├─ config bind: addr / endpoints.include / token / username / password
  ├─ bean wiring: []health.Indicator, []endpoint.Endpoint — all optional
  ├─ Server.Run(): listen, register routes, wrap with the auth guard,
  │               WARN if unauthenticated && non-loopback — SERVING IMMEDIATELY,
  │               before the app is ready
  ├─ readiness: sig.TriggerAndWait() → once ALL servers (this one included)
  │     report ready, s.ready flips to true
  ├─ on SIGTERM: PreStop sets draining=true
  │     → /readyz answers 503 OUT_OF_SERVICE but the server KEEPS SERVING,
  │       so K8s pulls the pod from Service endpoints before Stop()
  └─ Stop: http.Server.Shutdown rides the shutdown context
```

Why serving comes before readiness: a readiness probe must catch the
not-ready → ready flip, and a liveness probe must answer all through a slow boot —
otherwise the pod gets restarted for no reason.

### 2.2 Endpoint registration order

```
probes:          /healthz /readyz /startupz (+ /health /readiness /startup aliases)
                 — always registered, no filter can touch them (that would break
                 the Kubernetes contract)
introspection:   /info — passes the include filter
contributed:     every []endpoint.Endpoint bean, filter key = the pattern's path
                 (method prefix dropped, e.g. "/metrics"); registered after the
                 built-ins, so a contributor can never shadow /health — and a
                 duplicate path panics ServeMux at startup, so the mistake is loud
```

### 2.3 One readiness request, end to end

`GET /readyz` after startup, both indicators registered, mysql up, redis down:

1. Not ready yet, or draining? → immediate `503 {"status":"OUT_OF_SERVICE"}`.
2. Otherwise sweep every indicator in the readiness group. Which group an indicator
   belongs to: explicit `HealthGroups()`, else the default **readiness + startup,
   never liveness** — a dependency check must never be able to restart the pod.
3. The sweep has a **3s total timeout**: one slow dependency cannot drag the probe
   past a typical kubelet timeout; an indicator that comes in late sees an expired
   context and reports DOWN with the deadline error.
4. Per indicator: no error → `components[name] = {status: UP}`; error →
   `{status: DOWN, error: err}`.
5. Verdict: all UP → `UP`; only non-critical DOWN → `DEGRADED`; any critical DOWN →
   `DOWN`.
6. Only `DOWN` maps to HTTP 503. `DEGRADED` stays 200 — the app still serves
   traffic, one tolerable dependency is failing and you can see which.

```json
{
  "status": "DEGRADED",
  "components": {
    "mysql:orders": {"status": "UP"},
    "redis:cache":  {"status": "DOWN", "error": "context deadline exceeded"}
  }
}
```

`/healthz` sweeps only the **liveness** group — normally nothing declares liveness,
so it is a plain `200 {"status":"UP"}` while the process serves. `/startupz` also
gates on `s.ready` and ignores draining (once startup succeeds, the kubelet stops
polling it).

### 2.4 Endpoint shapes

| Endpoint | Response | Source |
|----------|----------|--------|
| `GET /info` | `{"go","module":{"path","version"},"build":{"revision","time","modified"}}` from `debug.ReadBuildInfo` | actuator.go |
| contributed (e.g. `/metrics`) | entirely the contributor's business (Prometheus text for starter-otel) | cloud/actuator/endpoint |

---

## 3. Drills

### 3.1 Baseline

```bash
curl -s :9370/readyz | jq .status          # "UP" (or "DEGRADED" if redis is down)
curl -s :9370/healthz                      # {"status": "UP"}
curl -s :9370/startupz | jq .status        # "UP" after startup
```

Early in startup you can still catch `curl -i :9370/readyz` → `503 OUT_OF_SERVICE`
before the readiness barrier crosses — the example's self-test asserts exactly this.

### 3.2 Critical indicator DOWN → readyz 503, healthz stays 200

```bash
curl -i -X POST :8002/admin/dep/down       # flip the critical mysql:orders indicator
curl -i :9370/readyz                       # 503 DOWN, components name the culprit
curl -i :9370/healthz                      # 200 — a dead dependency must not
                                           # trigger a liveness restart
curl -i -X POST :8002/admin/dep/up         # recover → readyz 200 UP again
```

### 3.3 Non-critical indicator DOWN → DEGRADED, still 200

```bash
curl -i -X POST :8002/admin/cache/down     # redis:cache is NonCritical
curl -s :9370/readyz | jq '.status, .components."redis:cache".status'
# "DEGRADED"        <- HTTP 200: still serving
# "DOWN"
curl -i -X POST :8002/admin/cache/up
```

### 3.4 Drain flip (graceful shutdown)

```bash
kill -TERM <pid>                           # or Ctrl+C
# immediately, repeatedly:
curl -s -o /dev/null -w '%{http_code}\n' :9370/readyz   # 503 OUT_OF_SERVICE
curl -s -o /dev/null -w '%{http_code}\n' :9370/healthz  # 200 — probes stay answerable
                                                          # through the drain window
```

`PreStop` sets `draining` **before** the server stops, so Kubernetes pulls the pod
from Service endpoints while in-flight requests finish. The example's `runTest`
automates this whole sequence (SIGTERM + poll loop). `/startupz` ignores draining by
design.

### 3.5 Endpoint filtering

```properties
spring.actuator.endpoints.include=/info,/metrics
```

```bash
curl -i :9370/info        # 200 (default-on, also in include)
curl -i :9370/metrics     # 200 (contributed endpoint in whitelist)
curl -i :9370/readyz      # 200 — probes exempt from the filter
```

Want a contributed endpoint like `metrics` gone? Turn it off in its own starter —
there is no exclude list. No auth + non-loopback addr also logs:
`WARN actuator listening on ":9370" without authentication; ...`.

### 3.6 Auth

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

### 3.7 Slow dependency bound

The whole readiness sweep shares one 3s budget. Wire an indicator whose `CheckHealth`
sleeps 5s → `readyz` answers within ~3s with that component `DOWN "context deadline
exceeded"`, instead of a probe timeout.

---

## 4. Design

The actuator is a Server-archetype starter: a management HTTP server on its own port
— the ops counterpart of your business server.

**What it owns**

- `/healthz`, `/readyz`, `/startupz` (K8s probes; the non-z names are aliases) and
  `/info`.
- Collecting every `health.Indicator` bean into `/readyz` — it knows no concrete
  backend (redis, gorm, ...); the seam is the stdlib interface.
- Collecting every `endpoint.Endpoint` bean onto the same port (otel's `/metrics`
  today, future contributors tomorrow) — no cross-starter imports.
- Coexisting with the app's main HTTP server (distinct bean name) and with
  `starter-pprof` / `starter-admin-ui` — one port each.

**Decisions worth knowing**

- **Serves during startup, not after readiness.** Bind and answer immediately;
  `sig.TriggerAndWait` only watches the aggregate. The probe contract demands it.
- **`health.Indicator` lives in `cloud/actuator/health`, not in the starter.** Every
  contributor (redis, gorm, ...) must reach it without importing this starter.
- **`PreStop` flips readiness.** `draining=true` → `/readyz` 503 while in-flight
  requests finish; the endpoint controller pulls the pod, then servers stop.
- **Endpoint contribution.** Built-ins register first, contributors after; a
  duplicate pattern panics ServeMux at boot — misconfiguration fails loudly.
  Sensitivity is the contributor's call (`Endpoint.Sensitive`), not the actuator's.
- **One 3s budget per readiness sweep.** One slow indicator cannot stall the probe.

**Rejected alternatives**

- **Reuse the app's main HTTP mux.** Probes must answer during startup and drain —
  the listener's lifecycle has to be decoupled from the app's readiness gate.
- **Push indicators from backend starters.** Push forces every backend to import the
  actuator; pull via interface export composes with any subset of backends.
- **Open: the 3s budget is shared.** Many indicators thin it out and one slow one
  starves the rest. Candidate: a per-indicator bound.

---

## 5. Troubleshooting

| Symptom | Why | Fix |
|---------|-----|-----|
| No actuator endpoints at all | `spring.actuator.addr` missing | Set it — the key is the switch. |
| Boot fails: `actuator: failed to listen on :9370` | port taken (another actuator, pprof overlap) | Change `addr` or free the port. |
| `/readyz` stuck at 503 OUT_OF_SERVICE after startup | some other `gs.Server` never reported ready — the barrier never crossed | Find which server blocks readiness; `s.ready` flips only when ALL servers are ready. |
| `/readyz` 503 DOWN, `/healthz` 200 | a critical readiness indicator is failing — working as intended | Read `components` for the failing dependency and its error. |
| `/metrics` 404 | starter-otel not imported, or `metrics` missing from a non-empty `endpoints.include` | Import starter-otel; remember contributed endpoints when whitelisting. |
| Sensitive contributed endpoint 404, nothing in logs | default-off; filtering logs at Debug only | Add its path to `endpoints.include` (case-insensitive exact match). |
| Boot panic: `http: multiple registrations for /...` | contributed pattern collides with a built-in or another contributor | Change the contributor's pattern — the panic is the intended fail-fast. |
| Every endpoint answers 401 | token (or Basic pair) configured — the guard covers probes too | Give probes and scrapers the header, or drop the keys and stay loopback. |
| Probe intermittently 503 DOWN with `context deadline exceeded` | one indicator is eating the shared 3s budget | Make `CheckHealth` honor ctx / tighten the check. |
