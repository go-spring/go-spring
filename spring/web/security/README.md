# security
[English](README.md) | [中文](README_CN.md)

`security` is a framework-agnostic, zero-dependency authentication and
authorization abstraction — the Spring Security equivalent expressed in Go
idioms rather than a port of its filter-chain machinery. It answers "who is
the caller?" (`Authentication` on the request context) and "may this caller
do this?" (`HasAnyAuthority`, `Require`, `Authorize`).

## Features

- Zero third-party dependencies.
- Neutral identity model: `Principal{Subject, Claims}`,
  `Authentication{Principal, Token, Authenticated, Authorities}` with
  nil-safe `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities`.
- Pluggable `TokenValidator` seam with a driver registry
  (`RegisterValidator` / `GetValidator` / `MustGetValidator`) mirroring
  `discovery.Register` / `resilience.RegisterDriver`.
- Method-level guard: `Require(authorities...)` is an `aspect.Interceptor`
  that plugs into the aspect chain — the `@PreAuthorize` equivalent.
- HTTP middleware chain: `Chain`, `Authenticate`, `Authorize`, `CORS`,
  `CSRF` (double-submit-cookie) — plain `func(http.Handler) http.Handler`
  decorators, not a bespoke filter registry.
- `WithAuthentication` / `FromContext` for context propagation.

## Quick Start

Import path: `go-spring.org/spring/security`.

A resource server wires the security filter chain in front of business
handlers:

```go
package main

import (
    "context"
    "net/http"

    "go-spring.org/spring/web/security"
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

    chain := security.Chain(
        security.CORS(security.CORSConfig{AllowedOrigins: []string{"*"}}),
        security.Authenticate(v, true),
        security.Authorize("orders:read"),
    )
    _ = http.ListenAndServe(":8080", chain(mux))
}
```

For method-level checks inside a service, wrap the call with the aspect
chain and `security.Require`:

```go
import "go-spring.org/spring/aspect"

chain := aspect.NewChain(security.Require("orders:write"))
_, err := aspect.Around(chain, ctx, "PlaceOrder", svc.placeOrder)
```

A JWT resource-server starter (`starter-security-jwt`) contributes a
concrete `TokenValidator` and `Wrap`s the server mux; an authorization
server starter (`starter-oauth2-server`) issues the tokens `Authenticate`
verifies.
