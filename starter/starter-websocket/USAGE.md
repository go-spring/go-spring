# starter-websocket Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`) and the self-asserting [example/](example)
(`example/check.sh`). gorilla/websocket semantics (frames, subprotocol negotiation, `CheckOrigin`)
are [gorilla/websocket docs](https://github.com/gorilla/websocket) — everything below is
go-spring's increment.

**Activation**: unconditional on import — the blank import registers the provider; the bean is
created from `${spring.websocket}` even with zero keys (all defaults). **No-port design**: the
starter contributes a configured `*websocket.Upgrader` only; WebSocket routes are mounted on
whatever HTTP server the app already runs (gs's built-in server, gin, echo, ...), which owns the
listening address and timeouts.

---

## 1. Complete worked project

An echo service with subprotocol negotiation, origin allowlisting, compression, and an
auth gate in front of the upgrade. File tree:

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
    github.com/gorilla/websocket  latest
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-websocket latest
    go-spring.org/starter-actuator latest   // optional: probes
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-websocket"
)

func main() { gs.Run() }
```

**router.go** — the application's entire WebSocket surface:

```go
package router

import (
    "net/http"

    "github.com/gorilla/websocket"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

type Controller struct{}

func (c *Controller) Echo(conn *websocket.Conn) {
    defer conn.Close()
    for {
        mt, msg, err := conn.ReadMessage()
        if err != nil {
            return
        }
        if err = conn.WriteMessage(mt, msg); err != nil {
            return
        }
    }
}

func init() {
    gs.Provide(&Controller{})

    // The starter injects the configured *websocket.Upgrader; the mux bean
    // mounts routes on the gs built-in HTTP server.
    gs.Provide(func(c *Controller, upgrader *websocket.Upgrader) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.Handle("/echo", requireApp(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            conn, err := upgrader.Upgrade(w, r, nil)
            if err != nil {
                log.Errorf(r.Context(), log.TagAppDef, "upgrade /echo failed: %v", err)
                return
            }
            c.Echo(conn)
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
# --- the HTTP server owns the port (starter-websocket owns none) -------------
spring.http.server.addr=:9696
# Long-lived connections: zero the server timeouts or they cut sockets mid-stream.
spring.http.server.readTimeout=0
spring.http.server.writeTimeout=0
spring.http.server.idleTimeout=0

# --- upgrader tuning (all optional; keys are exact-match camelCase) -----------
spring.websocket.handshakeTimeout=10s
spring.websocket.readBufferSize=1024
spring.websocket.writeBufferSize=1024
spring.websocket.enableCompression=true
spring.websocket.allowedOrigins=http://127.0.0.1:9696
spring.websocket.subprotocols=echo.v1
```

**Verify**:

```bash
go run . &
curl -si -H 'X-App: go-spring' 'localhost:9696/echo'    # 400/426 without upgrade headers
# with a ws client requesting echo.v1, the negotiated subprotocol is echoed back:
#  conn.Subprotocol() == "echo.v1"   (asserted by example/check.sh)
```

The runnable [example/](example) dials text/JSON echo, asserts subprotocol negotiation and the
403 from the auth gate; `example/check.sh` runs it self-asserting.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-websocket
  └─ gs.Provide(NewUpgrader, gs.TagArg("${spring.websocket}"))
        └─ Condition: gs.OnMissingBean[*websocket.Upgrader]()

gs.Run()
  ├─ config bind: ${spring.websocket} → Config (value tags; all optional)
  ├─ bean wiring: your *gs.HttpServeMux provider gets the *websocket.Upgrader
  │    injected; an application-provided Upgrader bean takes precedence over
  │    the starter's (OnMissingBean)
  └─ run: nothing else — the starter has no server, no port, no lifecycle,
     no shutdown hooks. Connection lifecycle is gorilla/websocket's.
```

Order note: the upgrader bean exists before route providers run, so handler wiring never races
the config — there is no timing surface beyond standard bean wiring.

### 2.2 One connection, layer by layer

`GET /echo` with `Sec-WebSocket-Protocol: echo.v1`, header `X-App`, Origin
`http://127.0.0.1:9696`:

1. The gs HTTP server (timeouts zeroed for streaming) dispatches to your mux.
2. Your guard middleware runs *before* the upgrade — a 403 here never reaches gorilla (asserted
   by the example's bad-dial case).
3. `upgrader.Upgrade(w, r, nil)`:
   - `CheckOrigin` — empty `allowedOrigins` keeps gorilla's default same-origin policy; a
     non-empty allowlist replaces it (exact match on the Origin header; a single `*` entry
     accepts any origin).
   - subprotocol negotiation: the first entry of your `subprotocols` that the client also
     requests wins; no match ⇒ no subprotocol selected (connection proceeds).
   - `handshakeTimeout` bounds the server-side handshake read.
4. Application read/write loop (gorilla semantics — ping/pong, message types, close handshake
   are yours to handle). Buffer sizes apply to the connection's I/O buffers.

---

## 3. Per-key behavior reference

All keys live under `spring.websocket` — **exact-match camelCase, matching the value tags
literally** (no relaxed binding: `handshaketimeout`/`handshake-timeout` do not resolve).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `handshakeTimeout` | duration | `10s` | Bounds the server-side handshake read (gorilla `Upgrader.HandshakeTimeout`). | Too low kills slow/mobile clients mid-handshake. |
| `readBufferSize` | int | `1024` | I/O buffer size in bytes for the connection's reader. | Too small ⇒ fragmented reads, more syscalls; too big ⇒ memory per connection. |
| `writeBufferSize` | int | `1024` | I/O buffer for the writer. | Same tradeoff on the write path. |
| `enableCompression` | bool | `false` | permessage-deflate (RFC 7692). ⚠ Clients must negotiate it too. | Enabled but client doesn't support ⇒ plain frames (harmless); expecting compression savings without client support ⇒ none. |
| `subprotocols` | []string | — | Advertised in preference order; first client match wins during handshake. | Listing none leaves negotiation to your code; mismatch ⇒ no subprotocol on the conn (silent). |
| `allowedOrigins` | []string | — | Empty = gorilla default same-origin `CheckOrigin`; non-empty replaces it with an exact-match allowlist; `*` accepts any origin. ⚠ There is **no key for an arbitrary CheckOrigin function** — provide your own `*websocket.Upgrader` bean for that. | Browser clients from unlisted origins get 403 at upgrade; `*` disables CSRF-relevant origin checking. |

⚠ Sibling starter [starter-websocket-coder](../starter-websocket-coder) shares this exact
`spring.websocket` prefix — see §6.

---

## 4. Verification & fault drills

### 4.1 Subprotocol negotiation

```bash
# any ws client requesting echo.v1:
#   websocket.DefaultDialer.Dial(url, header{"Sec-WebSocket-Protocol": "echo.v1"})
# then conn.Subprotocol() == "echo.v1" — asserted by example/check.sh
```

Request a protocol not in `subprotocols` ⇒ `conn.Subprotocol()` is empty and the connection
still works — a silent drift if the client assumes a protocol.

### 4.2 Origin gate drill

```bash
curl -si -o/dev/null -H 'Origin: http://evil.example' -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: x' \
  localhost:9696/echo        # 403 from CheckOrigin
curl -si ... -H 'Origin: http://127.0.0.1:9696' ...   # passes the gate
```

### 4.3 Compression drill

With `enableCompression=true`, send a large (>buffer) text frame from a compressing client and
compare frame sizes on the wire (devtools/wireshark); with a non-compressing client the
connection proceeds uncompressed.

### 4.4 Auth-gate-before-upgrade drill

The example asserts a dial without the `X-App` header yields HTTP 403 — proving guards run
before `Upgrade`, so upgrade errors and guard errors never mix.

### 4.5 Smoke test

```bash
cd example && ./check.sh    # self-asserting: echo, JSON echo, subprotocol, 403 gate
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|-----|--------------|-----|
| Connections drop after ~5s/60s | built-in HTTP server timeouts cut streams | `spring.http.server.readTimeout/writeTimeout/idleTimeout=0` (as in the example). |
| Browser gets 403 at connect | Origin not in `allowedOrigins` (or default same-origin policy) | Add the exact Origin; `*` only for public endpoints. |
| `conn.Subprotocol()` empty | client's requested protocols don't intersect `subprotocols` | Align the lists; check client spelling. |
| Key seems ignored (e.g. `handshake-timeout`) | camelCase exact-match keys — kebab/case variants don't bind | Use the literal tag spelling: `handshakeTimeout`. |
| Need custom CheckOrigin logic | no config key for a function | Provide your own `*websocket.Upgrader` bean — `OnMissingBean` yields to it. |
| 400/426 on plain curl | curl doesn't send upgrade headers | Expected; test with a real ws client. |
| Big messages stall | `readBufferSize`/`writeBufferSize` too small for the frame cadence | Raise the buffers; they are per-connection. |
| Both websocket starters configured from one key block | shared `spring.websocket` prefix with the coder sibling | Import one per protocol family, or scope keys to fields that exist in both. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 6 |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 2 (server timeouts; CheckOrigin ceiling) |

Design suspects (for the audit ledger):

1. camelCase keys (`handshakeTimeout`) deviate from the repo's kebab-case convention — a trap
   given exact-match binding (no relaxed forms).
2. Sibling starters `starter-websocket` / `starter-websocket-coder` share the same
   `spring.websocket` prefix — importing both configures both from one key block, and their value
   keys collide conceptually (`subprotocols` means the same thing in both).
3. No observability whatsoever (no connection counter, no upgrade-failure metric) — for a
   long-lived-connection starter, upgrade success rate is the health signal operators miss most.
