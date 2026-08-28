# starter-pprof Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `pprof.go`) and the runnable, self-asserting
[example/](example/) (`example/check.sh`, zero external dependencies). **Profiling semantics
(profile types, `go tool pprof` workflows) are the [Go standard docs](https://pkg.go.dev/net/http/pprof)** —
everything below is go-spring's increment: server wiring, enablement, and auth.

**Activation**: blank import. **Enabled by default** — `gs.OnProperty("spring.pprof.enabled").
HavingValue("true").MatchIfMissing()` (starter.go:28-29). This is the repo's one deliberate
port exception (decision `starter-server-port-must-be-configured`): the pprof starter defaults
to `:9981` on all interfaces and starts with zero configuration, explicitly to make runtime
diagnostics available out of the box. Opt out with `spring.pprof.enabled=false`.

---

## 1. Complete worked project

A service exposing protected pprof endpoints on loopback. File tree (isomorphic to `example/`):

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
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-pprof   latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-pprof"
)

func main() { gs.Run() }
```

That is the entire application: the blank import registers `NewSimplePProfServer` as a bean
(starter.go:34-37), exported as `gs.Server`, so the framework starts and drains it alongside
any other servers.

**conf/app.properties** — the complete, commented surface (copied from the example):

```properties
# Dedicated pprof HTTP server address. Starter default is ":9981" (ALL
# interfaces); the example pins loopback so the self-test hits it deterministically.
spring.pprof.addr=127.0.0.1:9981

# Token protection. When set, every request must present the token as an
# "Authorization: Bearer <token>" header. Basic auth is also supported via
# spring.pprof.username / spring.pprof.password.
spring.pprof.token=s3cr3t

# Opt-out switch (uncomment to disable the server entirely):
# spring.pprof.enabled=false
```

**Verify** (isomorphic to `example/example.go runTest`):

```bash
go run . -manual &
curl -i http://127.0.0.1:9981/debug/pprof/                          # 401 unauthorized
curl -i -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/   # 200
curl -i -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/heap  # 200
go tool pprof http://127.0.0.1:9981/debug/pprof/profile?seconds=5 -H '' # see §4.3
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-pprof
  └─ init(): gs.Provide(NewSimplePProfServer,
            gs.IndexArg(1, gs.TagArg("${spring.pprof}")))
              .Condition(OnProperty("spring.pprof.enabled").MatchIfMissing())
              .Export(gs.As[gs.Server]())                      [starter.go:28-37]

gs.Run()
  ├─ config bind: ${spring.pprof} → Config (value tags on pprof.go:35-48)
  ├─ bean construction: NewSimplePProfServer
  │    ├─ mux: registers /debug/pprof/{,cmdline,profile,symbol,trace} (pprof.go:68-72)
  │    ├─ warn check: !guard.Enabled() && !httpauth.IsLoopback(addr) →
  │    │   log.Warnf("pprof server listening on %q without authentication; ...")  [pprof.go:74-78]
  │    └─ guard: httpauth.Guard token/basic-auth wrapper around the mux (stdlib/httpauth)
  ├─ Run: the gs.Server collection starts it with the main HTTP server
  └─ SIGTERM: rides the framework's graceful server drain
```

Note the ctor shape: two args (`ctx *gs.ContextProvider`, `c Config`) bound via
`gs.IndexArg(1, gs.TagArg(...))` — a zero-config blank import still gets defaults bound.

### 2.2 One request, step by step

`GET /debug/pprof/heap` with `token=s3cr3t` configured:

1. `http.ServeMux` routes the `GET /debug/pprof/` pattern to `pprof.Index` (heap and the
   other profile handlers hang off the index — Go stdlib behavior).
2. `guard` (`httpauth.Guard.Wrap`, stdlib/httpauth): token is configured, so the
   `Authorization: Bearer` header is compared in constant time
   (`subtle.ConstantTimeCompare`) so timing does not leak the secret. A `?token=`
   query parameter is NOT accepted — header only.
3. Match → stdlib pprof handler renders; mismatch → `401 unauthorized`.
4. With no scheme configured, `guard` returns the handler unchanged — but the constructor
   has already logged the no-auth warning if the bind is non-loopback (pprof.go:74-78).

**gs.Server, not Runner**: the bean is exported as `gs.Server` (starter.go:37) and collected
into the `[]Server` slice by type — the framework runs it like any server. This matters:
a server that blocks would deadlock registration, but `SimpleHttpServer.Serve` returns after
`TriggerAndWait`, matching the repo's "Runner side-effects must not block readiness" rule.

---

## 3. Per-key behavior reference

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.pprof.enabled` | bool | true | Activation condition with `MatchIfMissing` (starter.go:28). Absent = on. | Forgetting to disable → endpoints on `:9981` in every environment. |
| `spring.pprof.addr` | string | `:9981` | Listen address. `:9981` binds **all interfaces**; `127.0.0.1:9981` loopback (`httpauth.IsLoopback`, stdlib/httpauth, treats ""/`0.0.0.0` host as non-loopback). ⚠ Deliberate exception to the no-default-port rule — every off-host deployment must set this or auth. | Default + no auth → runtime internals exposed off-host (only a startup warning). Port clash with another process → bind error at startup. |
| `spring.pprof.token` | string | `""` | When set, requires an `Authorization: Bearer <token>` header on every request; **takes precedence over** username/password (`httpauth.Guard`, stdlib/httpauth). | Setting only username/password + token="" works; setting token makes basic auth unreachable. |
| `spring.pprof.username` | string | `""` | HTTP Basic user — used only when **both** username and password are set (`httpauth.Guard.Enabled`, stdlib/httpauth). | Only one of the pair set → silently no auth (with the no-auth warning if non-loopback). |
| `spring.pprof.password` | string | `""` | HTTP Basic password, constant-time compared (`httpauth`, stdlib/httpauth). | Same coupling as username. |

---

## 4. Verification & fault drills

### 4.1 Token auth, header only (matches example `runTest`)

```bash
curl -i http://127.0.0.1:9981/debug/pprof/cmdline                # 401 unauthorized
curl -i -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/cmdline   # 200
curl -i 'http://127.0.0.1:9981/debug/pprof/cmdline?token=s3cr3t'  # 401 (query form removed)
curl -i -H 'Authorization: Bearer wrong' http://127.0.0.1:9981/debug/pprof/cmdline   # 401
```

### 4.2 Basic-auth variant

```properties
spring.pprof.username=admin
spring.pprof.password=secret
```

```bash
curl -i http://127.0.0.1:9981/debug/pprof/                 # 401 + WWW-Authenticate: Basic realm="restricted"
curl -i -u admin:secret http://127.0.0.1:9981/debug/pprof/ # 200
```

(Note `token` set at the same time would make basic auth unreachable — precedence rule, §3.)

### 4.3 Profile capture

```bash
curl -s -H 'Authorization: Bearer s3cr3t' 'http://127.0.0.1:9981/debug/pprof/profile?seconds=5' -o cpu.pb
go tool pprof -top demo-binary cpu.pb        # CPU profile interpretation: Go stdlib docs
curl -s -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/heap -o heap.pb
go tool pprof -sample_index=alloc_space -top demo-binary heap.pb
```

### 4.4 No-auth exposure drill

Remove `token` from the config and set `spring.pprof.addr=:9981`; at startup the log shows
(pprof.go:75-77):

```
WARN ... pprof server listening on ":9981" without authentication; set ${spring.pprof.token} or ${spring.pprof.username}/${spring.pprof.password}
```

Any host on the network can now read goroutine stacks and heap data — that warning is the
only signal. Restore loopback or auth to extinguish.

### 4.5 Opt-out drill

`spring.pprof.enabled=false` → the condition fails, no bean, no port, no warning.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| 401 on every request | token/basic misconfigured or wrong credential | Check which scheme is active; remember token takes precedence over basic. |
| Basic auth always 401 | `token` is also set — basic path unreachable | Clear `token`, set both `username` and `password`. |
| Startup warning "without authentication" | `:9981` bind + no auth | Set `spring.pprof.addr=127.0.0.1:9981` or configure auth (pprof.go:74-78). |
| Port :9981 already in use | Another process or a second gs app on the host | Change `addr` or disable via `spring.pprof.enabled=false`. |
| Server not running at all | `spring.pprof.enabled=false` somewhere in the config chain | Remove/flip the key; absent means enabled. |
| `go tool pprof` can't fetch profile behind auth | It does not send credentials from URL in some setups | Download with curl (§4.3) and pass the file to `go tool pprof`. |
| Wanted it off in prod, on in dev | Single shared config | Per-environment config: `spring.pprof.enabled=false` in the prod profile. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 2 |

Design suspects (kept from the previous audit, re-verified):

1. This starter violates the repo's "server ports are never defaulted" rule — `:9981`
   all-interfaces + enabled-by-default is an accepted, deliberate exception
   (starter.go:24-29; decision `starter-server-port-must-be-configured`), but every doc
   must restate the exposure risk.
2. RESOLVED: the `?token=` query fallback was removed (header-only now) because query
   strings leak tokens into access logs, browser history and shell history. The guard
   itself now lives in the shared `stdlib/httpauth` package (bearer + Basic, constant-time
   compares, `IsLoopback`), reusable by actuator and admin UI starters.
