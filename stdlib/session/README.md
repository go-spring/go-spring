# session
[English](README.md) | [中文](README_CN.md)

`session` is a framework-agnostic, zero-dependency abstraction for
server-side HTTP sessions — the Spring Session equivalent expressed in Go
idioms rather than a port of its `@EnableRedisHttpSession` machinery. A
stateful web / SSO deployment can have replica A write the session and
replica B read it, with no change to business handlers.

## Features

- Zero third-party dependencies.
- Three-piece split (like `cache` / `lock` / `security`):
  - `Session` — id + attribute bag + createdAt; obtained from context via
    `FromContext`, never constructed by business code.
  - `SessionStore` — `Load` / `Save(ttl)` / `Delete`. Remote backends
    implement the narrower `ByteStore` and are lifted with `FromByteStore`
    (JSON encoding). Registered stores via `Register` / `Get` / `MustGet`.
    Bundled `Memory` store is registered as `"memory"`.
  - `Manager` — the single HTTP seam. `Manager.Middleware` loads by cookie,
    attaches to ctx, and writes back before the first response byte.
- Lazy id assignment: an untouched visit creates no store entry and no
  cookie.
- Sliding renewal: every request that carries a session refreshes the TTL
  and cookie `Max-Age`.
- `Session.RenewID` rotates the id on privilege change (login) to defeat
  session fixation.
- `Session.Invalidate` destroys server state and expires the cookie (logout).

## Quick Start

Import path: `go-spring.org/stdlib/session`.

```go
package main

import (
    "fmt"
    "net/http"
    "time"

    "go-spring.org/stdlib/session"
)

func main() {
    store := session.NewMemory() // or a distributed backend from a starter
    mgr := session.NewManager(store, session.Options{
        CookieName:  "SESSION",
        IdleTimeout: 30 * time.Minute,
    })

    mux := http.NewServeMux()
    mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        s.RenewID()             // defeat session fixation
        s.Set("user", "u-1")
        _, _ = w.Write([]byte("ok"))
    })
    mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        if v, ok := s.Get("user"); ok {
            fmt.Fprintf(w, "user=%v", v)
            return
        }
        http.Error(w, "unauthenticated", http.StatusUnauthorized)
    })

    _ = http.ListenAndServe(":8080", mgr.Middleware(mux))
}
```

For cross-replica sharing use `starter-session-redis`; it contributes a
`session.SessionStore` bean built on `FromByteStore` over the Redis client.
The `Manager` API is unchanged.
