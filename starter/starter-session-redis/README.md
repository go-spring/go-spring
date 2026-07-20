# starter-session-redis

[English](README.md) | [中文](README_CN.md)

`starter-session-redis` contributes a Redis-backed
[`session.SessionStore`](../../spring/session) bean to a Go-Spring application,
so HTTP sessions are shared across replicas: replica A writes, replica B reads —
the Spring Session equivalent, expressed with a middleware plus config instead of
`@EnableRedisHttpSession`.

It follows the *Contributor* archetype (see
[starter/DESIGN.md](../DESIGN.md)): the starter exports no port and holds no
client of its own. It reuses the `*redis.Client` bean registered by
`starter-go-redis` and contributes a bean behind the framework-neutral
`session.SessionStore` seam. Switching the session backend to any other
distributed store is therefore a blank-import swap — no business code changes.

## Installation

```bash
go get go-spring.org/starter-session-redis
```

## Quick Start

### 1. Import both starters

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-session-redis"
)
```

### 2. Configure a Redis client, then a session store that references it

```properties
# A Redis client managed by starter-go-redis.
spring.go-redis.cache.addr=127.0.0.1:6379

# A session store bound to that client. `client` is the redis instance name.
spring.session.redis.web.client=cache
spring.session.redis.web.key-prefix=myapp:session:
```

The `client` property is **required**. Booting without it fails fast — the
starter refuses to silently default to some arbitrary Redis instance.

### 3. Inject the store and mount the session middleware

The store is exported behind the `session.SessionStore` interface. Hand it to a
`session.Manager` and wrap your handler; the middleware reads the session id from
the request cookie, loads the session into the context, and writes it back before
the response headers are sent.

```go
import (
    "net/http"

    "go-spring.org/spring/gs"
    "go-spring.org/spring/session"
)

gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
    mgr := session.NewManager(store, session.Options{
        CookieName:  "SESSION",
        IdleTimeout: 30 * time.Minute, // sliding: refreshed on every request
        Secure:      true,             // send only over HTTPS in production
    })

    mux := http.NewServeMux()
    mux.HandleFunc("/cart/add", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        s.Set("item", r.URL.Query().Get("item"))
    })
    mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        s.RenewID()               // rotate the id after auth — defeats fixation
        s.Set("user", "alice")
    })

    return &gs.HttpServeMux{Handler: mgr.Middleware(mux)}
}, gs.TagArg("web"))
```

Because the middleware talks only to `session.SessionStore`, several `Manager`s
across several replicas backed by the same Redis share session state
transparently.

## Configuration

All keys sit under `spring.session.redis.<name>`:

| Key          | Default     | Description                                                                        |
|--------------|-------------|------------------------------------------------------------------------------------|
| `client`     | —           | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.<client>`.   |
| `key-prefix` | `session:`  | Prepended to every session id so multiple apps can share a Redis instance safely.  |

Cookie name, path, `Secure`, `SameSite`, and the idle timeout are set on
`session.Options` when you build the `Manager` — they are HTTP concerns, not
storage concerns, so they live with the middleware rather than in the store
config.

## Behavior

* **Cross-replica sharing** — the session lives in Redis under its id; any
  replica that receives the cookie loads the same session.
* **Sliding renewal** — every request that carries a session refreshes the Redis
  key TTL (and the cookie `Max-Age`) to the idle timeout, so an active session
  stays alive; an idle one expires exactly `IdleTimeout` after the last request.
* **Session-fixation defense** — `Session.RenewID()` rotates the id (deleting the
  old Redis entry) while preserving attributes; call it right after login.
* **Lazy allocation** — a visitor who never writes to the session gets no cookie
  and creates no Redis entry; the id is issued on the first `Set`.
* **Secure ids** — 256 bits of `crypto/rand`, so ids are unguessable.
* **Fail-fast configuration** — a missing `client` refuses to boot instead of
  surfacing on the first session read.
