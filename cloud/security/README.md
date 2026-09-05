# security
[English](README.md) | [中文](README_CN.md)

`security` is a framework-agnostic, zero-dependency authentication and
authorization abstraction — the Spring Security equivalent expressed in Go
idioms rather than a port of its filter-chain machinery. It answers "who is
the caller?" (`Authentication` on the request context) and "may this caller
do this?" (`HasAnyAuthority`, `Require`).

## Features

- Zero third-party dependencies.
- Neutral identity model: `Principal{Subject, Claims}`,
  `Authentication{Principal, Token, Authenticated, Authorities}` with
  nil-safe `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities`.
- Pluggable `TokenValidator` seam with a driver registry
  (`RegisterValidator` / `GetValidator` / `MustGetValidator`) mirroring
  `discovery.Register` / `resilience.RegisterDriver`.
- Method-level guard: `Require(authorities...)` returns a plain decorator —
  the `@PreAuthorize` equivalent, composed with anything else by ordinary
  function nesting.
- `WithAuthentication` / `FromContext` for context propagation.
- Shared pure helpers so per-family middleware cannot drift:
  `ParseBearerToken`, `NewCSRFToken` / `MatchCSRFToken` (constant-time),
  `DefaultCSRFCookieName` / `DefaultCSRFHeaderName`.

## HTTP middleware

This package deliberately ships no HTTP middleware: each server family
installs its own, in its own idiom, on top of the shared identity model —
stdlib `http.Handler` decorators in `starter-http-server`, `gin.HandlerFunc`
in `starter-gin`, `echo.MiddlewareFunc` in `starter-echo`. CORS is likewise
each family's own concern (`starter-http-server` ships one; gin uses
`gin-contrib/cors`, echo its built-in).

## Quick Start

Import path: `go-spring.org/cloud/security`.

A validator produces the identity; a server-family middleware attaches it:

```go
package main

import (
    "context"
    "net/http"

    "go-spring.org/cloud/security"
    httpsvr "go-spring.org/starter-http-server"
)

type myValidator struct{ /* ... */ }

func (v *myValidator) Validate(ctx context.Context, token string) (*security.Authentication, error) {
    // verify the bearer token here and return an Authentication
    return &security.Authentication{
        Principal:     security.Principal{Subject: "u-1"},
        Token:         token,
        Authenticated: true,
        Authorities:   []string{"orders:read"},
    }, nil
}

func main() {
    v := &myValidator{}
    mux := http.NewServeMux()
    mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
        // auth is on r.Context() already
        _, _ = security.FromContext(r.Context())
        _, _ = w.Write([]byte("ok"))
    })

    chain := httpsvr.Chain(
        httpsvr.Authenticate(v, true),
        httpsvr.Authorize("orders:read"),
    )
    _ = http.ListenAndServe(":8080", chain(mux))
}
```

For method-level checks inside a service, wrap the call with the decorator
chain and `security.Require`:

```go
err := security.Require("orders:write")(ctx, svc.placeOrder)
```

A JWT resource-server starter (`starter-security-jwt`) contributes a
concrete `TokenValidator`; an authorization server starter
(`starter-oauth2-server`) issues the tokens the middleware verifies.
