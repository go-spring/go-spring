# starter-session-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`, `store.go`), the shared abstraction
[cloud/experimental/session](../../../cloud/experimental/session), and the self-asserting
[example/](example) (`example/check.sh`). Session semantics (cookie handling, idle timeout,
`RenewID`) live in the session package; Redis semantics are [Redis docs](https://redis.io/docs/latest/commands/set/) —
everything below is go-spring's increment.

**Activation**: any `spring.session.redis.<name>.*` property registers one `session.SessionStore`
instance per `<name>`; each reuses the `*redis.Client` bean named by its `client` field (provided
by starter-go-redis under `spring.go-redis.<client>`). The starter holds no connection of its own.

---

## 1. Complete worked project

Two HTTP "replicas" sharing one session store through Redis — login rotates the session id and
both replicas read the same session. File tree:

```
demo/
├── go.mod
├── main.go
├── web.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/redis/go-redis/v9     latest
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-go-redis   latest
    go-spring.org/starter-session-redis latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-session-redis"
)

func main() { gs.Run() }
```

**web.go** — the application's entire session surface:

```go
package main

import (
    "net/http"
    "time"

    "go-spring.org/cloud/experimental/session"
    "go-spring.org/spring/gs"
)

func init() {
    // The store bean is injected by name; Manager construction (cookies,
    // idle timeout) is application policy, not starter config.
    gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
        opt := session.Options{
            CookieName:  "SESSION",        // default
            IdleTimeout: 30 * time.Minute, // default; expiry is sliding
        }
        mgrA := session.NewManager(store, opt) // "replica A"
        mgrB := session.NewManager(store, opt) // "replica B"

        mux := http.NewServeMux()
        mux.Handle("/a/set", mgrA.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            s, _ := session.FromContext(r.Context())
            s.Set("user", r.URL.Query().Get("user"))
        })))
        mux.Handle("/a/login", mgrA.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            s, _ := session.FromContext(r.Context())
            s.RenewID() // session-fixation defense: new id, data kept
        })))
        mux.Handle("/b/get", mgrB.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            s, _ := session.FromContext(r.Context())
            if v, ok := s.Get("user"); ok {
                _, _ = w.Write([]byte(v.(string)))
            }
        })))
        return &gs.HttpServeMux{Handler: mux}
    }, gs.TagArg("web"))
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- redis client (owned by starter-go-redis; the store reuses it by name) --
spring.go-redis.cache.addr=127.0.0.1:6379

# --- session store -------------------------------------------------------------
spring.session.redis.web.client=cache                 # required; fail-fast when empty
spring.session.redis.web.key-prefix=starter-session-redis:example:
```

**Verify** (with a local Redis, e.g. `example/docker-compose.yml`):

```bash
go run . &
curl -si 'localhost:9090/a/set?user=alice' | grep -i set-cookie   # SESSION=...
curl -s --cookie 'SESSION=<id>' localhost:9090/b/get              # alice — cross-replica
```

The runnable [example/](example) asserts cross-replica read, id rotation on login (old id dead),
and idle-timeout expiry; `example/check.sh` wraps it in docker compose.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-go-redis + starter-session-redis
  └─ gs.Module(gs.OnProperty("spring.session.redis"))
        └─ conf.BindEach("${spring.session.redis}") per entry <name>:
             ├─ fail fast when client == ""   (boot error naming the instance)
             └─ Provide newStore → bean "<name>"   (Export session.SessionStore;
                  gs.ValueArg(c), gs.TagArg(c.Client)   NO destroy hook)

gs.Run()
  ├─ config bind: ${spring.session.redis.<name>} → Config (value tags)
  ├─ newStore: no dialing — the *redis.Client bean is injected ready-made.
  │    Store embeds session.FromByteStore(&redisByteStore{client, prefix}):
  │    all session (de)serialization stays in the stdlib abstraction; the
  │    concrete type is only a named, exportable wrapper (gs beans cannot
  │    return unexported interface impls).
  ├─ bean wiring: consumers' autowire:"<name>" resolved
  └─ on SIGTERM: nothing to release — no destroy hook; sessions persist in
                 Redis and expire via key TTL. The redis client's Close belongs
                 to starter-go-redis.
```

### 2.2 One request, layer by layer (sliding expiry)

`GET /b/get` with cookie `SESSION=<id>`:

1. `Manager.Middleware` reads the cookie (name from `session.Options`, default `SESSION`).
2. The store's `Get(ctx, id)` maps to `GET <key-prefix><id>` — the value is the opaque JSON
   blob produced by `session.FromByteStore`; `redis.Nil` (miss) maps to `found=false`, which the
   Manager turns into a fresh, lazily-allocated session.
3. On response, `sessionWriter.commit` writes through to `SET <key-prefix><id> <json> EX
   <IdleTimeout>` — **the Redis key TTL is the idle timeout**, so expiry and sliding renewal are
   enforced by Redis itself: every request through the middleware with an existing session
   (modified or merely read) re-saves and re-arms the full idle window server-side. A brand-new,
   untouched session writes nothing — anonymous traffic allocates no session. A non-positive idle
   timeout maps to Redis expiration 0 = no expiry.
4. `RenewID()` (login) generates a new id, saves the data under it and deletes the old key —
   the old session id is dead immediately (asserted by the example).
5. Cookie handling: `Set-Cookie` with `Max-Age = IdleTimeout` is rewritten once, before the first
   byte reaches the client (header ordering guaranteed by the Manager's responseWriter wrapper).

HTTP concerns (cookie name/path/Secure/SameSite, idle timeout) live on `session.Options` at
Manager construction — deliberately not starter config; one store can back many Managers.

---

## 3. Per-key behavior reference

All keys live under `spring.session.redis.<name>` (exact-match, no relaxed forms).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `client` | string | — | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.<client>`; checked before bean registration. `TagArg(c.Client)` is the seam tying store to redis instance. | Empty → boot fails naming the instance; typo → bean-wiring failure at boot. |
| `key-prefix` | string | `session:` | Prepended to every session id before it hits Redis; keeps key spaces of apps sharing one Redis disjoint. | Shared prefix across apps → sessions leak across applications (same cookie value resolves). |

⚠ That is the entire surface: two keys. Everything else (cookies, expiry, serialization) is either
`session.Options` or the shared abstraction.

---

## 4. Verification & fault drills

### 4.1 Cross-replica session sharing

```bash
curl -si 'localhost:9090/a/set?user=alice' | awk -F': ' '/[Ss]et-[Cc]ookie/{print $2}'
curl -s --cookie 'SESSION=<id>' localhost:9090/b/get     # "alice" — replica B reads replica A's write
```

Under the hood:

```bash
redis-cli --scan --pattern 'starter-session-redis:example:*'
redis-cli ttl 'starter-session-redis:example:<id>'       # = IdleTimeout
```

### 4.2 Sliding window drill

With `IdleTimeout: 2s` (as in the example):

```bash
id=<cookie>
redis-cli ttl "starter-session-redis:example:$id"   # ~2
sleep 1; curl -s --cookie "SESSION=$id" localhost:9090/b/get >/dev/null
redis-cli ttl "starter-session-redis:example:$id"   # back to ~2 — re-armed by the request
sleep 3
curl -s --cookie "SESSION=$id" localhost:9090/b/get # empty — idle expiry by Redis
```

### 4.3 Session-id rotation drill (fixation defense)

```bash
old=$(curl -si 'localhost:9090/a/set?user=alice' | awk ... )     # first id
new=$(curl -si --cookie "SESSION=$old" localhost:9090/a/login | awk ...)  # rotated id
curl -s --cookie "SESSION=$new" localhost:9090/b/get   # alice — data survived
curl -s --cookie "SESSION=$old" localhost:9090/b/get   # empty — old id destroyed
```

### 4.4 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up redis, run self-asserting example
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `session-redis: instance "<n>" missing required property ...client` | instance without `client` | Set it to an existing `spring.go-redis.<name>`. |
| Boot fails wiring `*redis.Client` | `client` typo | Fix the name to match a `spring.go-redis.<client>` entry. |
| Sessions shared across unrelated apps | same `key-prefix` (or default `session:`) on one Redis | Distinct `key-prefix` per app. |
| Sessions never expire | `IdleTimeout` 0/negative on `Options` maps to Redis "no expiry" | Set a positive `IdleTimeout`. |
| Sessions expire mid-activity despite requests | traffic bypasses the Manager's Middleware, so nothing slides the TTL | Every request through `mgr.Middleware` with an existing session re-saves and re-arms the window; make sure the middleware actually wraps the route. |
| Old session still valid after login | handler didn't call `RenewID` | Call `s.RenewID()` on privilege change. |
| Redis restart logs everyone out | sessions are in-memory only | Expected; pair with Redis persistence or accept re-login. |
| Cookie not observed in `curl` | `Set-Cookie` is written once before the first body byte; later writes are ignored by the wrapper | Don't inspect after streaming has begun; use `-i` on the raw response. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 2 |
| Required | 1 (`client`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 2 |

Design suspects (for the audit ledger):

- None structural. The two-key surface is minimal and the HTTP/session split is clean; the only
  friction is the cross-module `client` name reference shared by every redis-consuming starter
  (documented above).
- No observability at all on this starter (no observer block, unlike the lock family) — session
  store latency/errors are invisible; a future observe-session bridge would mirror observe-lock.
