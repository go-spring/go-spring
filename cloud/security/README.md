# security
[English](README.md) | [中文](README_CN.md)

`security` is a framework-agnostic, zero-dependency authentication and
authorization abstraction — the Spring Security equivalent expressed in Go
idioms rather than a port of its filter-chain machinery. It answers two
questions for a resource server: **who is the caller?** (`Authentication` on
the request context) and **may this caller do this?** (`HasAnyAuthority`,
`Require`).

Token verification itself is pluggable: a starter implements the single
`TokenValidator` interface and contributes it as a container bean; a
server-family middleware takes that validator, validates the incoming
credential, and attaches the resulting identity to the context for downstream
guards.

## Quick Start

Import path: `go-spring.org/cloud/security`.

A complete, runnable version of this walkthrough lives in
[`example/`](example/) (`go run ./example`, or `./example/check.sh`).

### 1. Verifying a credential: the `TokenValidator` seam

Implement the one method: given a raw token, return the identity it represents,
or a non-nil error for any credential you cannot vouch for.

```go
type jwtValidator struct{ /* keys, issuer, ... */ }

func (v *jwtValidator) Validate(ctx context.Context, token string) (*security.Authentication, error) {
    claims, err := v.verify(ctx, token) // your verification + claims type
    if err != nil {
        return nil, err
    }
    return &security.Authentication{
        Principal:     security.Principal{Subject: claims.Subject, Claims: claims.Raw},
        Token:         token,
        Authenticated: true,
        Authorities:   claims.Roles, // flattened scopes/roles, your naming convention
    }, nil
}
```

A starter contributes a concrete validator as a container bean (for example
`starter-security-jwt`); which validator a resource server uses is a wiring
decision, not a name-keyed lookup here.

### 2. Attaching the identity to a request

Each server family ships its own middleware, in its own idiom, on top of this
identity model — nothing to install from this package. The shell validates
through the `TokenValidator` it was wired with and puts the result on the
request context:

```go
import httpsvr "go-spring.org/starter-http-server"

chain := httpsvr.Chain(
    httpsvr.Authenticate(validator, true), // required=true: no token -> 401
    httpsvr.Authorize("orders:read"),      // route-level gate
)
http.ListenAndServe(":8080", chain(mux))
```

Downstream code reads the caller back with `FromContext`:

```go
auth, _ := security.FromContext(r.Context()) // auth may be nil / unauthenticated
if auth.HasAnyAuthority("orders:read") {     // predicates are nil-safe
    _ = auth.Principal.Subject               // fields are not: guard first
}
```

### 3. Gating a method

`Require` is a plain decorator — the `@PreAuthorize` equivalent. It reads the
identity from the context and returns a sentinel the caller maps to a status:

```go
err := security.Require("orders:write")(ctx, svc.placeOrder)
switch {
case errors.Is(err, security.ErrUnauthenticated): // 401: no verified identity
case errors.Is(err, security.ErrForbidden):       // 403: authenticated, lacked the authority
}
```

With no authorities, `Require()` degrades to "an authenticated caller is
required". There is deliberately no shared interceptor-chain protocol: the
decorator is an ordinary function, so combining cross-cutting concerns is
ordinary nesting —

```go
err := security.Require("orders:write")(ctx, func(ctx context.Context) error {
    return transaction.GlobalTransactional(coord, reg)(ctx, "OrderService.Place", place)
})
```

## What's in the package

- **Identity model.** `Principal{Subject, Claims}` and
  `Authentication{Principal, Token, Authenticated, Authorities}`, with
  `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities`.
- **`TokenValidator`** — the single seam a starter or app implements to plug in
  JWT verification, opaque-token introspection, etc.
- **`Require(authorities...)`** — the method-level guard decorator.
- **`WithAuthentication` / `FromContext`** — context propagation, keyed by an
  unexported type so nothing else collides.
- **Shared pure helpers** so per-family middleware cannot drift:
  `ParseBearerToken`, `NewCSRFToken` / `MatchCSRFToken` (constant-time
  compare), `DefaultCSRFCookieName` / `DefaultCSRFHeaderName`.
- **Error sentinels** `ErrUnauthenticated` (→ 401) and `ErrForbidden` (→ 403).

## Where the HTTP middleware lives

This package deliberately ships no HTTP middleware: each server family installs
its own, in its own idiom, on top of the shared identity model — stdlib
`http.Handler` decorators in `starter-http-server`, `gin.HandlerFunc` in
`starter-gin`, `echo.MiddlewareFunc` in `starter-echo`. CORS is likewise each
family's own concern (`starter-http-server` ships one; gin uses
`gin-contrib/cors`, echo its built-in).

## Design

### Responsibilities & boundaries

- Two questions only: **who is the caller**, and **may they do this**.
- Not the crypto library. `TokenValidator` is the seam; JWT / opaque-token /
  session-cookie implementations live in starters or the calling app.
- Not a session library (see `cloud/experimental/session`), not an OAuth2 authorization
  server (see `starter-oauth2-server`).
- HTTP middleware, route-level `Authorize`, and CORS stay with each server
  family; only the security-sensitive logic those shells share is exposed here
  as pure helpers, so behavior cannot drift between families. The method-level
  `Require` lives here — the same authority set, two gates: a route gate in the
  server's idiom, a decorator gate in this package.

### Constraints (do not break)

- **`Authentication` methods are nil-safe** and `!Authenticated` always returns
  false. Downstream code may call `HasAnyAuthority` on the value fetched from
  `FromContext` without a nil guard; do not add fields that break this.
- **`Authenticate(v, required=false)`** (in every family shell) must let the
  request through with no `Authentication` attached when the token is absent —
  the "authority decision deferred to a later filter" case. An **invalid**
  token always yields 401; a missing token only 401s when `required=true`.
- **CORS wildcard vs credentials**: with `AllowCredentials=true`, do not emit
  `Access-Control-Allow-Origin: *` — the spec forbids it. Echo the concrete
  origin instead, and add `Vary: Origin`.
- **CSRF is double-submit-cookie**: server-side state-free. Safe methods seed
  the cookie; unsafe methods must echo the cookie in the header via
  `MatchCSRFToken` (constant-time). It is orthogonal to bearer-token APIs,
  which are not CSRF-prone; do not force it on APIs.
- **Never accept HMAC on an asymmetric-key configuration** in `TokenValidator`
  implementations (algorithm-confusion defence). This invariant lives in the
  starter, not here, but is called out because building a validator without it
  is a well-known footgun.

### Trade-offs / alternatives rejected

- **No shared HTTP middleware protocol.** A shared
  `func(http.Handler) http.Handler` layer fit only net/http and forced
  gin/echo through adapters (hertz not at all); it was removed in favor of
  per-family shells. The cost of duplicated shells is bounded by sharing the
  pure helpers and the identity model; the win is that each family reads
  natively.
- **No Spring Security filter registry.** Ordering is the shell's chain order;
  reasoning is explicit and there is no invisible priority system.
- **No driver registry for validators.** Validators are contributed as
  container beans by starters; a middleware takes the `TokenValidator` value it
  was wired with and validates each request through it — no name-keyed global
  lookup at request or wiring time.
- **No annotation scanning.** `@PreAuthorize` is replaced by an explicit
  `security.Require(...)` wrapping the call.

A JWT resource-server starter (`starter-security-jwt`) contributes a concrete
`TokenValidator`; an authorization server starter (`starter-oauth2-server`)
issues the tokens the middleware verifies.
