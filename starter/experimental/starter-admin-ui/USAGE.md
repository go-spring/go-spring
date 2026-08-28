# starter-admin-ui Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go` — the whole server lives in this one file,
`config.go`, plus the embedded HTML template string) and the runnable, self-asserting
[example/](example/) (`example/check.sh`, zero external dependencies — it stands up two fake
actuator instances itself). Actuator endpoint semantics belong to starter-actuator — only the
dashboard server wiring is covered here.

**Activation**: blank import; the bean condition is
`gs.OnProperty("spring.admin-ui.addr")` — the starter ships dark and is activated **only by
configuring a listen address**, matching the starter-actuator contract
(`spring.actuator.addr`). There is no `spring.admin-ui.enabled` key and no default port:
not configuring `addr` means no admin-ui server bean at all (decision
`starter-server-port-must-be-configured`).

> Migration (from the previous default-on behavior): releases before this change enabled
> the dashboard by default on `:9280`. If you relied on that, add
> `spring.admin-ui.addr=:9280` to your configuration — `spring.admin-ui.enabled` is no
> longer read.

**Auth**: the dashboard can be protected with `spring.admin-ui.token` (Bearer) or
`spring.admin-ui.username` + `spring.admin-ui.password` (HTTP Basic), via the shared
`stdlib/httpauth` guard. When no credentials are configured and `addr` binds a
non-loopback interface, startup logs a WARN (same pattern as starter-pprof).

---

## 1. Complete worked project

A process that serves the aggregated dashboard for two actuator instances. File tree
(isomorphic to `example/`):

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-admin-ui   latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-admin-ui"
)

func main() { gs.Run() }
```

That is the whole application: the starter's `init()` registers the `Server` bean named
`adminUIServer` (starter.go:62-65), exported as `gs.Server`; the container populates its
`Config` field from the `spring.admin-ui` tree.

**conf/app.properties** — the complete, commented surface (copied from the example):

```properties
# Listen address — REQUIRED; setting it is what activates the starter.
# Distinct from the main HTTP server (:9090), the actuator (:9370), and
# pprof (:9981), so all four can coexist in one process.
spring.admin-ui.addr=:9280

# Optional dashboard auth (stdlib/httpauth guard): a bearer token, or HTTP
# Basic (spring.admin-ui.username + spring.admin-ui.password). Without either,
# on a non-loopback addr, startup logs a warning. The example uses a token.
spring.admin-ui.token=example-token

# Actuator base URLs the UI polls (indexed form; comma-separated also works).
# The UI appends /health, /readiness, /startup, /info itself.
spring.admin-ui.instances[0]=http://127.0.0.1:19371
spring.admin-ui.instances[1]=http://127.0.0.1:19372

# Poll cadence and per-request timeout. The page also auto-refreshes at this
# cadence via a <meta http-equiv="refresh"> tag.
spring.admin-ui.interval=1s
spring.admin-ui.timeout=500ms

# Page title — label the dashboard per environment.
spring.admin-ui.title=Go-Spring Admin

# This example only needs the Admin UI server, so the default main HTTP
# server is disabled to keep the ports clean.
spring.http.server.enabled=false
```

**Verify** (isomorphic to `example/example.go runTest`):

```bash
go run . -manual &
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/ | grep -E '19371|19372|pill up'   # rows render, UP pills
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/api/status | python3 -m json.tool  # machine-readable snapshot
```

Kill one fake actuator (`lsof -ti:19371 | xargs kill`) and watch its row flip to a red
DOWN pill with the error message within one `interval`.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-admin-ui
  └─ init(): gs.Provide(&Server{}).Name("adminUIServer")
              .Condition(OnProperty("spring.admin-ui.addr"))
              .Export(gs.As[gs.Server]())                       [starter.go:62-66]

gs.Run()
  ├─ config bind: ${spring.admin-ui} → Server.Config (field-level value tags, config.go)
  ├─ Server.Run (starter.go:100-156), order is deliberate:
  │    1. net.Listen(addr)        — port conflict fails FAST, before the readiness barrier
  │    2. template.Parse          — embedded constant; error would be a starter bug
  │    3. http.Client build       — MaxIdleConns sized to len(Instances)*4 for reuse
  │    4. s.refresh(ctx)          — SEED poll synchronously: first page load shows real data
  │    5. newHandler(): mux GET / and GET /api/status, wrapped by the httpauth
  │       guard (token / basic); WARN when unguarded on non-loopback addr
  │    6. sig.TriggerAndWait()    — triggers readiness but does NOT block: the dashboard
  │                                  stays reachable during startup "just like the actuator"
  │    7. go pollLoop(); Serve
  ├─ steady state: one poller goroutine; handlers read the last snapshot under RWMutex
  └─ SIGTERM: StopContext → close(stop) → wait <-done → http.Server.Shutdown(ctx)
```

Rationale cited from source comments:

- **Bind before readiness** (starter.go:95-98): a port conflict fails fast during startup,
  before the readiness barrier.
- **Seed poll synchronous** (starter.go:130-133): bounded by the poll timeout, so it cannot
  delay startup; the first page load "shows real data rather than an empty table".
- **Trigger-but-not-block** (starter.go:144-147): the dashboard must stay reachable during
  startup so operators can watch the transition.
- **Poller cadence capping** (pollLoop, starter.go:204-209): each sweep gets a context
  bounded by one interval "so a slow set of instances never lets consecutive sweeps overlap".
- **Snapshot over live reads** (starter.go:86-88): "a stale snapshot beats a slow page load"
  — handlers never block on a poll.

### 2.2 One poll cycle, step by step (refresh → pollOne)

1. Ticker fires every `interval` (≤0 falls back to 10s at three separate sites — pollLoop,
   handleDashboard, config default).
2. `refresh` polls **all instances in parallel** (goroutine per instance, `sync.WaitGroup`).
3. Per instance, `pollOne` (starter.go:275-358) issues **four independent GETs**:
   `/health` (failure ⇒ `Health=DOWN` + `Error` — the operator's "unreachable" cue),
   `/readiness` (also extracts the `components` map into sorted per-indicator rows),
   `/startup`, `/info` (build metadata; errors silently ignored — "nice-to-have").
   Each call: per-request context timeout of `timeout` (≤0 falls back to 3s);
   non-2xx bodies are still decoded (a 503 readiness body carries a valid status).
4. Results are sorted by base URL (stable table order), stored as one atomic snapshot
   under the mutex with `polledAt`.
5. The next `GET /` or `GET /api/status` copies the snapshot out under RLock and renders.

**How it discovers instances**: it does not — `instances` is static config read once at
bind time; there is no registry/discovery integration (suspect #2 in §6).

### 2.3 Shutdown walk

`StopContext` (starter.go:166-183): idempotent `close(stop)` unblocks the poller's select;
`<-done` waits for poller exit; then `http.Server.Shutdown(ctx)` drains in-flight page
loads riding the framework's shutdown context.

---

## 3. Per-key behavior reference

Bound from the `spring.admin-ui` tree (prefix set by the `${spring.admin-ui}` tag on
`Server.Config`, starter.go:75 — a top-level absolute reference, not instance-prefixed).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | none (required) | **Activation key**: setting it assembles the server bean (starter.go:62-66); deliberately distinct from main (:9090), actuator (:9370), pprof (:9981) so four servers coexist. No `enabled` key; migration from default-on releases: add `addr`. | Unset → no server at all (default-off). Port clash fails fast at listen; all-interfaces bind without credentials logs a WARN. |
| `token` | string | `""` | When set, every request needs `Authorization: Bearer <token>`; takes precedence over username/password (httpauth guard in `newHandler`). | Unset on non-loopback addr → WARN logged; dashboard readable unauthenticated. |
| `username` / `password` | string | `""` | When both set, HTTP Basic auth (`WWW-Authenticate` challenge on failure). | Only one of the two set → guard stays disabled (WARN as above). |
| `instances` | []string | `""` (empty) | Actuator base URLs, indexed (`instances[0]=`) or comma-separated. Empty is deliberately allowed — UI degrades to "No instances configured" (config.go:34-38; template empty state). ⚠ Static: no discovery, no hot reload of the list. | Empty string entry → row shows `empty instance URL` / UNKNOWN (guard in pollOne, starter.go:281-285). Wrong port → DOWN rows with error text. |
| `interval` | duration | 10s | Poll cadence; also the page `<meta refresh>` period (min 1s in the template data). ≤0 falls back to 10s (pollLoop starter.go:193-196 and handleDashboard). | Very low + many instances ⇒ 4N/interval req/s against the actuators. |
| `timeout` | duration | 3s | Per-endpoint HTTP timeout during a poll; ≤0 falls back to 3s (refresh, starter.go:220-223). Bounded by the sweep context = one interval. | Too low ⇒ flapping UNKNOWN on slow actuators; too high lets a wedged instance eat into the sweep budget. |
| `title` | string | `Go-Spring Admin` | Rendered in `<title>` and header — per-environment labelling (config.go:50-53). | Cosmetic only. |

⚠ The poller speaks plain HTTP with **no auth and no TLS** toward the instances — flat,
trusted networks only (suspects #2/#3 in §6). The dashboard itself is protectable via
`token` / `username`+`password` (see table above).

---

## 4. Verification & fault drills

### 4.1 Dashboard renders (matches example `runTest`)

```bash
body=$(curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/)
echo "$body" | grep -c 'pill up'          # >= 2 (one per healthy instance)
echo "$body" | grep 'redis:alpha'         # per-component readiness rows
echo "$body" | grep 'last polled:'        # snapshot timestamp + refresh cadence
```

### 4.2 JSON polling endpoint

```bash
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/api/status
# {"polled_at": "2026-...T..Z", "instances": [
#   {"base":"http://127.0.0.1:19371","health":"UP","readiness":"UP",
#    "components":[{"name":"redis:alpha","status":"UP"}],
#    "module":"example.com/alpha","version":"v0.0.1",
#    "revision":"deadbeef","build_time":"...","go":"go1.26"} ...]}
```

### 4.3 Instance-down drill

```bash
lsof -ti:19371 | xargs kill               # "crash" one instance
sleep 2                                    # > one poll interval
curl -s -H "Authorization: Bearer example-token" http://127.0.0.1:9280/api/status | grep 19371
#   "health":"DOWN","readiness":"DOWN","startup":"DOWN","error":"...connection refused"
```

The dashboard row flips to red DOWN pills with the error inline; the server itself keeps
serving — a broken target never wedges the poller (per-request timeouts + sweep cap).

### 4.4 Partial-failure drill

Serve a 503 with body `{"status":"OUT_OF_SERVICE"}` from one endpoint (readiness of a
starting app does exactly this): `getJSON` accepts non-2xx (starter.go:377-379), so the row
shows the orange OUT_OF_SERVICE pill — the body's status, not a transport DOWN.

### 4.5 Empty-instances degradation

Remove the `instances` keys (or set none): the page serves with
"No instances configured. Set spring.admin-ui.instances ..." — a deliberate degradation,
not an error (template empty state, starter.go:552-554).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Rows all DOWN with `connection refused` | Wrong port / actuator not on the base URL; actuator is on its own `:9370`, not the app port | Point `instances` at the actuator management port. |
| Row shows `empty instance URL` / UNKNOWN | Blank entry in the instances list (pollOne guard, starter.go:281-285) | Remove the empty entry. |
| Startup fails: `admin-ui: failed to listen on :9280` | Port conflict | Change `addr`; unsetting it disables the starter entirely. |
| Dashboard shows "No instances configured" | `instances` unset/typo'd (key is exact-match, no relaxed binding) | Check the key spelling and indexed form. |
| Status stuck / page never updates | `interval` misconfigured (≤0 falls back to 10s silently); or the sweep is timing out | Check `interval`; raise `timeout` if actuators are slow. |
| UNKNOWN statuses on healthy instances | Non-JSON response from a non-actuator target — `statusOf` returns UNKNOWN when `status` is missing (starter.go:391-399) | Target must speak the actuator JSON shape. |
| Components column shows "—" | Target's `/readiness` has no `components` map | Expected for third-party endpoints; only /info build data may still show. |
| HTTPS/auth-protected actuators unreachable | Poller is plain HTTP, no credentials, no TLS (suspect #3) | Not supported; expose plain actuator ports on a trusted network. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 7 |
| Required | 1 (`addr` — it is the activation key) |
| Quickstart external deps | 0 (targets need actuator) |
| "Watch out" entries | 2 |

Design suspects (kept from the previous audit, re-verified):

1. Resolved (was: default-enabled + defaulted port with no auth/warning). Activation is now
   `spring.admin-ui.addr` (default-off), and the dashboard supports token/Basic auth via
   the shared stdlib/httpauth guard, with a pprof-style WARN when unguarded on a
   non-loopback bind.
2. `instances` is static config; adding an instance requires a redeploy or config refresh —
   no discovery integration.
3. No credentials/TLS support for polled instances limits it to flat, trusted networks.
