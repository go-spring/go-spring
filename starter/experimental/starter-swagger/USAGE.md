# starter-swagger Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim is verified against
the starter source (`starter.go`, `config.go`, `ui.go`) and the runnable self-asserting
[example/](example/) (`example/check.sh`, zero external services). Swagger UI itself is
[swagger-ui's own](https://github.com/swagger-api/swagger-ui) — this document covers the binding,
the endpoint mount, and the CDN dependency.

**Activation**: unlike the addr-gated server starters, this one follows the **enabled-switch
pattern with default on** — the bean registers when `spring.swagger.enabled` is `true` *or
missing* (`MatchIfMissing`, starter.go:37). Blank-importing is enough to get the UI; disable in
production with `spring.swagger.enabled=false` without removing the import.

---

## 1. Complete worked project

A service exposing a greeter API whose docs are browsable at `/swagger/`. Two variants: with the
actuator (zero wiring) and without (own HTTP server). The example/ runs variant B. File tree:

```
demo/
├── go.mod
├── main.go
├── openapi.json            # produced by `gs-http-gen --openapi`
└── conf/
    └── app.properties
```

**go.mod**:

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-swagger    latest
    go-spring.org/starter-actuator   latest   // variant A: management port + endpoint mount
)
```

**Variant A — actuator mount (zero wiring)**. main.go is just `gs.Run()` plus blank imports:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-swagger"
)

func main() { gs.Run() }
```

**conf/app.properties** (variant A):

```properties
# --- actuator management port --------------------------------------------------
spring.actuator.addr=:9370

# --- swagger (all defaults shown; only specFile usually changes) ---------------
spring.swagger.enabled=true
spring.swagger.basePath=/swagger
spring.swagger.specFile=openapi.json
spring.swagger.title=Greeter API Docs
# CDN serving swagger-ui.css / swagger-ui-bundle.js; pin a major version.
spring.swagger.assetBaseURL=https://unpkg.com/swagger-ui-dist@5
```

**Variant B — no actuator** (this is `example/example.go:43-47`): the bean is also a plain
`*UI` (`http.Handler`), so mount it yourself:

```go
gs.Provide(func(ui *StarterSwagger.UI) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle(ui.Path(), ui)          // ui.Path() = "<basePath>/"
    return &gs.HttpServeMux{Handler: mux}
})
```

with `spring.http.server.enabled=true` + `spring.http.server.addr=:9696`.

**openapi.json** — minimal spec the starter accepts (it serves any valid JSON document verbatim;
structure is [the OpenAPI spec's](https://spec.openapis.org/oas/v3.1.0), typically generated):

```json
{
  "openapi": "3.0.3",
  "info": { "title": "Greeter API", "version": "1.0.0" },
  "paths": {
    "/greeter/{name}": {
      "get": {
        "summary": "Greet by name",
        "parameters": [{ "name": "name", "in": "path", "required": true,
                         "schema": { "type": "string" } }],
        "responses": { "200": { "description": "ok" } }
      }
    }
  }
}
```

In a real project this file is produced by the generator (`gs-http-gen --openapi` over your
handlers, config.go:30-34) and committed or built alongside the binary — the starter only reads it.

**Verify** (variant A; adjust host for B):

```bash
curl -s :9370/swagger/            | grep -c swagger-ui    # 1 — the UI shell
curl -s -o/dev/null -w '%{http_code}\n' :9370/swagger/index.html    # 200, same shell
curl -s :9370/swagger/openapi.json | jq -r .info.title    # Greeter API
```

External services: none required by the starter; the **browser** needs to reach `assetBaseURL`
(CDN) unless you self-host a mirror — see §4.4.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-swagger (+ starter-actuator)
  └─ gs.Provide(NewUI, TagArg("${spring.swagger}"))
        .Export(endpoint.Endpoint)
        .Condition(OnProperty("spring.swagger.enabled")="true" MatchIfMissing)  starter.go:35-37

gs.Run()
  ├─ config bind: ${spring.swagger} → Config (basePath / specFile / title / assetBaseURL)
  ├─ NewUI:
  │    ├─ normalize basePath to "/<trimmed>"                       ui.go:72
  │    ├─ os.ReadFile(specFile) — missing/unreadable spec FAILS STARTUP
  │    │   (read once, so a broken spec never 404s after go-live)  ui.go:74-77
  │    ├─ render the HTML shell once via pageTemplate               ui.go:80-88
  │    └─ log "swagger ui configured basePath=… specFile=…"         ui.go:89
  ├─ actuator (if present) autowires every endpoint.Endpoint bean and mounts it
  │   at Path() = "<basePath>/" on the management port — the trailing slash claims
  │   the whole subtree                                              ui.go:98-100
  └─ runtime: ServeHTTP is a pure switch on r.URL.Path — spec bytes and page bytes
      are pre-rendered; no per-request file I/O                      ui.go:102-113
```

### 2.2 One request, layer by layer

`GET /swagger/openapi.json` on the actuator port (variant A):

1. The actuator mux routes by prefix: everything under `/swagger/` delegates to the `*UI` handler
   (its `Path()` registered at mount time).
2. `ServeHTTP` matches `r.URL.Path == specURL` → `Content-Type: application/json`, the spec bytes
   captured at startup are written verbatim (ui.go:104-106).
3. `GET /swagger/` or `/swagger` or `/swagger/index.html` → the pre-rendered HTML shell
   (ui.go:107-109). Anything else under the subtree → 404 (ui.go:110-111).
4. In the **browser**, the shell then pulls `swagger-ui.css` / `swagger-ui-bundle.js` from
   `assetBaseURL` (CDN — server-side proxying does not happen), and `SwaggerUIBundle` fetches
   the spec from the same-origin `specURL` baked into the page.

### 2.3 Why the two-mount design

The starter deliberately owns no listener (starter.go:25-31): mounting via
`endpoint.Endpoint` means the actuator's management port — already the place ops traffic
(probes, metrics) lives — also carries the docs, with zero wiring in the app. The plain
`http.Handler` escape hatch exists for apps without the actuator: inject `*UI`, mount `Path()`,
done (ui.go:59-61). Both mounts serve byte-identical content because the page and the spec are
rendered/read exactly once in `NewUI`.

Timing rationale (source comments): the spec is read exactly once *so that* a missing file fails
fast at boot rather than surfacing as a 404 after the app is live (config.go:30-34, ui.go:69-70);
the heavy assets live on a CDN *so that* the starter ships no vendored megabytes and serves only a
tiny shell (config.go:22-23).

Shutdown: nothing to drain — no listener, no goroutines; the bean is pure precomputed bytes.

---

## 3. Per-key behavior reference

All keys under `spring.swagger.*` (5 including the switch; 0 of the Config fields are required).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `enabled` | bool | true (MatchIfMissing) | The on/off switch — **diverges from the addr-activation convention** of server starters (starter.go:37). ⚠ absent means ON. | Expecting off-by-default → docs silently mounted in prod; set `false` explicitly. |
| `basePath` | string | `/swagger` | Subtree the UI claims; trimmed/normalized in NewUI (ui.go:72). `Path()` returns `basePath + "/"`, so the whole subtree (index, spec) is served by this one handler. ⚠ collisions with other actuator endpoints fail at mount. | Overlapping path with another endpoint → mount clash at startup. |
| `specFile` | string | `openapi.json` | Path to the OpenAPI document (typically from `gs-http-gen --openapi`); read once at startup (ui.go:74). ⚠ cwd-relative. ⚠ served **verbatim, never re-read** — regenerating the file needs a restart. | Missing/unreadable → startup fails (`swagger: reading spec file …`). |
| `title` | string | `API Documentation` | Browser tab title of the docs page (HTML shell only). | Cosmetic only. |
| `assetBaseURL` | string | `https://unpkg.com/swagger-ui-dist@5` | CDN base for `swagger-ui.css` / `swagger-ui-bundle.js`; trailing slash trimmed (ui.go:83). ⚠ pin a major version; point at a self-hosted mirror in air-gapped clusters (config.go:39-43). | Unpinned URL → a breaking UI release can silently change the page; unreachable CDN → blank page, see §5. |

No dead keys: all five are read (four via the Config value tags, `enabled` via the bean condition).

---

## 4. Verification & fault drills

### 4.1 Endpoint content checks

```bash
curl -s :9370/swagger/            | grep -o 'swagger-ui-bundle.js'      # shell references CDN bundle
curl -s :9370/swagger/            | grep -o '/swagger/openapi.json'     # spec URL is same-origin
curl -s -o/dev/null -w '%{ct}\n'  :9370/swagger/openapi.json            # application/json
curl -s -o/dev/null -w '%{http_code}\n' :9370/swagger/nope              # 404 inside the subtree
```

### 4.2 Disable drill (enabled switch)

Add `spring.swagger.enabled=false` and restart: `/swagger/` returns the actuator's 404 for an
unmounted path; the rest of the actuator is unaffected. Remove the key → UI is back (default on).

### 4.3 Fail-fast drill

Point `specFile` at a non-existent file and start: the process refuses to boot with
`swagger: reading spec file "…": open …: no such file or directory` — the guarantee that a broken
spec never reaches production (ui.go:74-77).

### 4.4 Air-gap drill (CDN dependency)

Block egress to `unpkg.com` and open `/swagger/` in a browser: the shell loads (200) but the page
is blank — the assets fail client-side. Fix by mirroring the assets and setting
`spring.swagger.assetBaseURL=https://assets.internal/swagger-ui-dist@5`; the shell then loads them
from your mirror. Server-side curls in §4.1 are unaffected — the CDN is a browser-side dependency
only.

### 4.5 Spec regeneration drill

Regenerate `openapi.json` (route changed) while the app runs: `/swagger/openapi.json` still serves
the old bytes (read once at startup) — restart to pick up the new spec.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails `swagger: reading spec file …` | specFile missing/unreadable, or wrong cwd | Fix path; it is working-directory relative (chdir or use an absolute path). |
| UI served in production unexpectedly | `enabled` defaults ON (MatchIfMissing) | Set `spring.swagger.enabled=false` in the prod profile. |
| Page loads but is blank | browser cannot reach `assetBaseURL` (CDN egress blocked) | Self-host the assets and point `assetBaseURL` at the mirror (§4.4). |
| `/swagger/openapi.json` stale after route changes | spec read once at startup, never re-read | Restart the process (regeneration alone is invisible). |
| 404 for everything under the subtree in variant B | mux mounted without the trailing-slash pattern | Mount exactly `mux.Handle(ui.Path(), ui)` — `Path()` supplies the `/`-suffixed pattern (ui.go:98-100). |
| Mount clash with another actuator endpoint | two endpoints claim overlapping subtrees | Change `basePath` for one of them. |
| UI renders but "Failed to load API definition" | spec URL not reachable from the browser (different host/port than the page) | Serve UI and spec from the same origin (they are, by construction — check reverse-proxy rewrites of `/swagger/`). |
| Actuator absent, nothing served | the starter owns no listener | Use variant B: inject `*UI` and mount it on your own server (§1). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 (incl. `enabled`) |
| Required | 0 (all defaulted; a missing spec file is what actually fails) |
| Quickstart external deps | 0 server-side (CDN reachable from the browser) |
| "Watch out" entries | 4 |

Design suspects (first two carried over from the previous audit):

1. `enabled` switch pattern diverges from the addr-activation convention used by server starters —
   default-on means an unconfigured import still mounts docs.
2. `assetBaseURL` CDN dependency — offline clusters need a mirror; the failure mode (blank page)
   is client-side and invisible to server-side monitoring.
3. Spec is read once and served forever: no `gs.Dync`-style refresh even though the config system
   supports hot reload — every spec update costs a restart.
4. `title` renders into an HTML template unescaped-by-convention (`html/template` auto-escapes, so
   it is safe, but it is the only purely cosmetic key — candidate for deletion per the template's
   single-key rule if no one configures it).
