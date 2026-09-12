# session
[English](README.md) | [中文](README_CN.md)

`session` is a framework-agnostic, zero-dependency abstraction for
server-side HTTP sessions. A stateful web / SSO deployment can have replica A
write the session and replica B read it, with no change to business handlers.
The bundled `Memory` store serves single-node and tests; a distributed
backend comes from `starter-session-redis` behind the same interface.

## Features

- Zero third-party dependencies; works with any `net/http`-compatible router.
- Typed attribute access: `session.Set[T]` / `session.Get[T]` with automatic
  JSON re-encoding for byte-backend round trips.
- Lazy id assignment: an untouched visit creates no store entry and no cookie.
- Sliding renewal: every request that carries a session refreshes the TTL
  and the cookie `Max-Age`.
- `Session.RenewID` rotates the id on privilege change (login) to defeat
  session fixation.
- `Session.Invalidate` destroys server state and expires the cookie (logout).

## Quick Start

Import path: `go-spring.org/cloud/experimental/session`.

```go
package main

import (
    "fmt"
    "net/http"
    "time"

    "go-spring.org/cloud/experimental/session"
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
        s.RenewID()                     // defeat session fixation
        session.Set(s, "user", "u-1")   // typed attribute access
        _, _ = w.Write([]byte("ok"))
    })
    mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
        s, _ := session.FromContext(r.Context())
        if v, ok, _ := session.Get[string](s, "user"); ok {
            fmt.Fprintf(w, "user=%v", v)
            return
        }
        http.Error(w, "unauthenticated", http.StatusUnauthorized)
    })

    _ = http.ListenAndServe(":8080", mgr.Middleware(mux))
}
```

A runnable, self-asserting version of this flow (lazy allocation, login
rotation, typed attributes, logout, idle expiry) lives in
[example/](example/), as a Go-Spring application that takes the store as a
bean.

## Usage

### Mount the middleware

Construct a `Manager` over a store and wrap the handlers that use the
session — this is the only place session transport lives. Multiple Managers
(even across replicas) may share one store; that is exactly how session
state is shared:

```go
mgr := session.NewManager(store, session.Options{IdleTimeout: 30 * time.Minute})
mux.Handle("/profile", mgr.Middleware(profileHandler))
```

Inside a wrapped handler, obtain the session from the request context:

```go
s, _ := session.FromContext(r.Context())
```

### Read and write attributes

Go methods cannot carry type parameters, so typed access is package-level:

```go
session.Set(s, "cart", []string{"sku-1", "sku-2"})
items, ok, err := session.Get[[]string](s, "cart")
```

With the in-process `Memory` store the original type is returned as-is. A
session loaded from a byte-oriented backend comes back as
`map[string]any` / `float64`; `Get` re-encodes through JSON so the intended
type is still yielded — which also means remotely stored attributes should
stay JSON-friendly. A value that cannot decode into `T` is an error, not a
silent zero. `s.Delete(key)` removes one attribute; `s.Keys()` lists them.

### Log in, log out

```go
// login: rotate the id the client presented pre-authentication, then record
// the authenticated user. The old id becomes invalid on write-back.
s.RenewID()
session.Set(s, "user", user)

// logout: delete the store entry and expire the client cookie.
s.Invalidate()
```

Both take effect at write-back, before the first response byte reaches the
client.

### Configure the cookie and idle timeout

`Options` covers `CookieName` (default `"SESSION"`), `Path`, `Domain`,
`Secure`, `SameSite` (default `Lax`), and `IdleTimeout` (default 30m).
`IdleTimeout` is how long a session may sit idle: every request carrying it
slides the deadline forward. Non-positive means the session never expires
server-side and the cookie is a session cookie. The cookie is always
`HttpOnly` and not configurable; the id is 32 bytes of `crypto/rand`,
base64url-encoded.

### Contribute a distributed backend

Remote stores implement the narrow `ByteStore` (`Get` / `Set` / `Delete` of
`[]byte`) over their client and lift it to a full `SessionStore` with
`FromByteStore`, which owns the JSON serialization for every backend:

```go
store := session.FromByteStore(myRedisByteStore)
mgr := session.NewManager(store, opt)
```

A store reaches the `Manager` by constructor injection (`NewManager`) — there
is no package-level store registry, so no store state is shared across tests
or restarts by accident. The bundled `Memory` is constructed explicitly
(`session.NewMemory()`); a backend that needs a live client (Redis, ...) is
contributed as a bean. `starter-session-redis` does exactly this; the
`Manager` API is unchanged when you switch to it.

### Use it in a Go-Spring application

A starter contributes the store as a bean; the application builds the
`Manager` at the seam where it already assembles its handlers:

```go
gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
    mgr := session.NewManager(store, session.Options{IdleTimeout: 30 * time.Minute})
    return &gs.HttpServeMux{Handler: mgr.Middleware(mux)}
})
```

`starter-session-redis` contributes that `session.SessionStore` per
`spring.session.redis.instances.<name>` entry. An in-process store is
contributed the same way: [example/](example/) defines a local starter in
[starter.go](example/starter.go) that does it under
`spring.session.memory.instances.<name>`, so the two backends differ by a
blank import and a config key, not by handler code.

### Reclaim expired sessions in a long-running process

`Memory` treats an expired session as absent the next time it is read, and
drops it then. A session that expires and is never read again would therefore
stay in the map for the life of the process — the case a distributed backend
covers with its key TTL. `Memory.StartCleanup(interval)` adds a background
sweep for that, with a matching `Stop`. The example's local starter exposes
the interval as `clean-interval` and stops the sweeper from the bean's
destroy hook; a non-positive interval (the default) leaves only the lazy path.

## Behavioral contract

- **Mutate early.** `Set-Cookie` must precede the response body, so the
  session is committed before the first `WriteHeader` / `Write` (and once
  more at middleware exit for handlers that never wrote). Attribute changes
  after the first write are silently not persisted — the same constraint any
  header carries.
- **Anonymous traffic allocates nothing.** A request that never touches the
  session gets no id, no store entry, and no cookie.
- **Every carried session is re-saved**, even without changes, to refresh
  the store TTL and cookie `Max-Age` (sliding renewal).
- **If the store fails mid-response** for a brand-new session, no cookie is
  issued — the client never receives an id that has no store entry; the
  response itself completes normally.

## Design notes

- `SessionStore` is the persistence seam; the same middleware serves an
  in-process `Memory` or a shared Redis backend without change.
- `ByteStore` serialization is JSON, not gob: cross-language readable and no
  version surprises.
- The package is not an identity provider — session attributes are an
  arbitrary bag; "who the caller is" belongs to `cloud/security`.
