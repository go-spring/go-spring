# starter-websocket-coder Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`) and the self-asserting [example/](example)
(`example/check.sh`). coder/websocket semantics (`websocket.Accept`, subprotocols,
`OriginPatterns`, compression modes) are [coder/websocket docs](https://github.com/coder/websocket) —
everything below is go-spring's increment.

**Activation**: unconditional on import — the blank import registers the provider; the bean is
created from `${spring.websocket}` even with zero keys (all defaults). **No-port design**: unlike
gorilla, coder/websocket has no Upgrader *object* — the server upgrade is the free function
`websocket.Accept(w, r, *AcceptOptions)`, so this starter contributes the injectable
`*websocket.AcceptOptions` itself. Mount routes on whatever HTTP server the app already runs.
It is the coder/websocket sibling of [starter-websocket](../starter-websocket) (gorilla) — both
import fine together, but they share the `spring.websocket` prefix.

---

## 1. Complete worked project

An echo service with subprotocol negotiation, context-takeover compression, JSON echo via
`wsjson`, and an auth gate in front of the upgrade. File tree:

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
    github.com/coder/websocket      latest
    github.com/coder/websocket/wsjson latest
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-websocket-coder latest
    go-spring.org/starter-actuator  latest   // optional: probes
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-websocket-coder"
)

func main() { gs.Run() }
```

**router.go** — the application's entire WebSocket surface:

```go
package router

import (
    "context"
    "net/http"

    "github.com/coder/websocket"
    "github.com/coder/websocket/wsjson"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Controller struct{}

func (c *Controller) Echo(ctx context.Context, conn *websocket.Conn) {
    defer conn.CloseNow()
    for {
        mt, msg, err := conn.Read(ctx)
        if err != nil {
            return
        }
        if err = conn.Write(ctx, mt, msg); err != nil {
            return
        }
    }
}

func init() {
    gs.Provide(&Controller{})

    // The starter injects the configured *websocket.AcceptOptions; the mux
    // bean mounts routes on the gs built-in HTTP server.
    gs.Provide(func(c *Controller, opts *websocket.AcceptOptions) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.Handle("/echo", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            conn, err := websocket.Accept(w, r, opts)
            if err != nil {
                log.Errorf(r.Context(), log.TagAppDef, "accept /echo failed: %v", err)
                return
            }
            c.Echo(r.Context(), conn)
        })))
        mux.Handle("/json", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            conn, err := websocket.Accept(w, r, opts)
            if err != nil {
                return
            }
            defer conn.CloseNow()
            var req struct{ Name string `json:"name"` }
            for {
                if err := wsjson.Read(r.Context(), conn, &req); err != nil {
                    return
                }
                _ = wsjson.Write(r.Context(), conn, map[string]string{"message": "Hi, " + req.Name})
            }
        })))
        return &gs.HttpServeMux{Handler: mux}
    })
}

func requireApp(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-App") != "go-spring" {
            http.Error(w, "forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- the HTTP server owns the port (the starter owns none) -------------------
spring.http.server.addr=:9797
# Long-lived connections: zero the server timeouts or they cut sockets mid-stream.
spring.http.server.readTimeout=0
spring.http.server.writeTimeout=0
spring.http.server.idleTimeout=0

# --- AcceptOptions tuning (all optional; exact-match camelCase keys) ----------
spring.websocket.subprotocols=echo.v1
spring.websocket.compressionMode=1          # 0 disabled, 1 enabled (context takeover),
                                            # 2 no context takeover — raw int, see §3
# spring.websocket.compressionThreshold=0
# spring.websocket.insecureSkipVerify=false
# spring.websocket.originPatterns=
```

**Verify**:

```bash
go run . &
curl -si -H 'X-App: go-spring' localhost:9797/echo     # 400/426 without upgrade headers
# with a ws client requesting echo.v1: conn.Subprotocol() == "echo.v1"
# (asserted by example/check.sh)
```

The runnable [example/](example) dials text/JSON echo, asserts subprotocol negotiation and the
403 from the auth gate; `example/check.sh` runs it self-asserting.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-websocket-coder
  └─ gs.Provide(NewAcceptOptions, gs.TagArg("${spring.websocket}"))
        └─ Condition: gs.OnMissingBean[*websocket.AcceptOptions]()

gs.Run()
  ├─ config bind: ${spring.websocket} → Config (value tags; all optional)
  ├─ bean wiring: your *gs.HttpServeMux provider gets the *websocket.AcceptOptions
  │    injected; an application-provided AcceptOptions bean takes precedence
  │    (OnMissingBean)
  └─ run: nothing else — no server, no port, no lifecycle, no shutdown hooks.
     Connection lifecycle (ctx-scoped Read/Write, CloseStatus handshake) is
     coder/websocket's.
```

### 2.2 One connection, layer by layer

`GET /echo` with `Sec-WebSocket-Protocol: echo.v1`, header `X-App`:

1. The gs HTTP server (timeouts zeroed for streaming) dispatches to your mux.
2. Your guard middleware runs *before* the accept — a 403 here never reaches coder/websocket
   (asserted by the example's bad-dial case).
3. `websocket.Accept(w, r, opts)`:
   - origin verification: coder/websocket verifies the Origin by default; `originPatterns`
     (glob match) is the `CheckOrigin` equivalent; `insecureSkipVerify` disables verification
     (browser clients only — non-browser clients have no Origin and are never verified).
   - subprotocol negotiation: your `subprotocols` × the client's request — first server-side
     match wins; none ⇒ no subprotocol on the connection.
   - `compressionMode` selects permessage-deflate behavior (see §3 for the raw-int trap);
     `compressionThreshold` gates compression by message size (0 = coder default behavior).
4. Application loop: reads/writes are context-scoped (`conn.Read(ctx)`/`conn.Write(ctx, ...)`);
   close with a status (`conn.Close(code, reason)`) or hard (`CloseNow`).

---

## 3. Per-key behavior reference

All keys live under `spring.websocket` — **exact-match camelCase, matching the value tags
literally** (no relaxed binding), and **shared with the gorilla sibling** (§6).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `subprotocols` | []string | — | Advertised in order; negotiated against the client's list. | Mismatch ⇒ no subprotocol on the conn (silent drift). |
| `insecureSkipVerify` | bool | `false` | Skips origin verification — browser clients only; non-browser clients are never verified anyway. | `true` disables the CSRF-relevant origin check for browser clients. |
| `originPatterns` | []string | — | Glob patterns for the Origin header (coder's `CheckOrigin` equivalent), e.g. `*.example.com`. | Wrong glob shape ⇒ browser clients rejected at accept. |
| `compressionMode` | int | `0` | Cast straight to `websocket.CompressionMode`: `0` disabled, `1` enabled with context takeover, `2` enabled without context takeover. ⚠ **Raw int** — a typo (e.g. `3`) silently becomes a different/invalid mode rather than an error. | Out-of-range values yield undefined compression behavior without any boot-time signal. |
| `compressionThreshold` | int | `0` | Min message size (bytes) before compressing; 0 keeps coder/websocket's default gating. | Too low ⇒ CPU burn on small frames; too high ⇒ no compression in practice. |

⚠ There is no key beyond these fields — provide your own `*websocket.AcceptOptions` bean for
anything else (`OnMissingBean` yields to it).

---

## 4. Verification & fault drills

### 4.1 Subprotocol negotiation

```go
conn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:9797/echo", &websocket.DialOptions{
    Subprotocols: []string{"echo.v1"},
})
// conn.Subprotocol() == "echo.v1" — asserted by example/check.sh
```

Request a protocol not listed ⇒ `Subprotocol()` empty, connection still works — silent drift.

### 4.2 Origin gate drill

```bash
curl -si -o/dev/null -H 'Origin: http://evil.example' -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: x' \
  localhost:9797/echo       # rejected by origin verification
# with spring.websocket.originPatterns=*.example.com it passes — verify both ways
```

### 4.3 Compression-mode drill

With `compressionMode=1`, send repeated similar payloads from a compressing client — context
takeover makes the second message much smaller on the wire (devtools/wireshark). With mode `0`
no compression is negotiated regardless of client support.

### 4.4 Auth-gate-before-accept drill

The example asserts a dial without the `X-App` header yields HTTP 403 — guards run before
`websocket.Accept`.

### 4.5 Smoke test

```bash
cd example && ./check.sh    # self-asserting: echo, JSON echo, subprotocol, 403 gate
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|-----|--------------|-----|
| Connections drop after ~5s/60s | built-in HTTP server timeouts cut streams | Zero `spring.http.server.*Timeout`/`idleTimeout` (as in the example). |
| Browser clients rejected at accept | default origin verification; Origin not covered | Add `originPatterns` globs (or `insecureSkipVerify=true` for public endpoints only). |
| `conn.Subprotocol()` empty | client's protocols don't intersect `subprotocols` | Align the lists. |
| Key seems ignored (e.g. `compression-mode`) | camelCase exact-match keys — kebab/case variants don't bind | Use the literal tag spelling: `compressionMode`. |
| Compression behaves oddly | raw-int `compressionMode` — a typo gives a silently different mode | Stick to 0/1/2; verify with §4.3. |
| Need custom accept behavior | no config key for it | Provide your own `*websocket.AcceptOptions` bean — `OnMissingBean` yields. |
| 400/426 on plain curl | curl doesn't send upgrade headers | Expected; test with a real ws client. |
| Configuring both websocket starters unexpectedly | shared `spring.websocket` prefix with the gorilla sibling | Import one per protocol family, or scope keys to shared fields (`subprotocols`). |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 3 (server timeouts; shared prefix; raw-int compressionMode) |

Design suspects (for the audit ledger):

1. Shares the `spring.websocket` prefix with starter-websocket (gorilla) — importing both
   configures both from one key block; key names differ but `subprotocols` is common.
2. camelCase keys (`insecureSkipVerify`) deviate from the repo's kebab-case convention — a trap
   given exact-match binding (no relaxed forms).
3. `compressionMode` as a raw int pushes coder/websocket's enum into config — a typo gives a
   silently different mode rather than an error.
4. No observability whatsoever (no connection counter, no accept-failure metric) — upgrade
   success rate is the health signal operators miss most.
