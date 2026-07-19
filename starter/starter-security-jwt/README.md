# starter-security-jwt

[English](README.md) | [中文](README_CN.md)

`starter-security-jwt` turns a Go-Spring application into an OAuth2/OIDC
**resource server**: it verifies JWT bearer tokens on incoming requests and, on
success, attaches the authenticated identity to the request context. It sits at
the `net/http` layer as a middleware, so it stays agnostic to whichever web
framework (gin/echo/hertz/net-http) serves the routes behind it, and it also
implements `security.TokenValidator` so tokens can be verified programmatically
on non-HTTP transports.

## Installation

```bash
go get go-spring.org/starter-security-jwt
```

## Quick Start

### 1. Import the `starter-security-jwt` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-security-jwt"
```

### 2. Configure an Authenticator

Add configuration in your project's [configuration file](example/conf/app.properties).
Each entry under `spring.security.jwt.*` yields one named `*Authenticator`.
Exactly one verification key source must be set — an HMAC secret, a PEM public
key, or a remote JWKS endpoint:

```properties
# HMAC (symmetric)
spring.security.jwt.api.secret=example-shared-secret

# or an asymmetric PEM public key
# spring.security.jwt.api.public-key-file=./conf/public.pem

# or a remote JWKS endpoint (keys fetched and refreshed automatically)
# spring.security.jwt.api.jwks-url=https://issuer.example.com/.well-known/jwks.json
```

### 3. Wire the Authenticator into the HTTP Server

The authenticator is injected by its config sub-key (`api`) and wraps your
business handler. Handing the wrapped handler to a `*gs.HttpServeMux` places
authentication in front of the server. Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(auth *StarterSecurityJWT.Authenticator) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
        a, _ := security.FromContext(r.Context())
        _, _ = fmt.Fprintf(w, "hello %s", a.Principal.Subject)
    })
    return &gs.HttpServeMux{Handler: auth.Wrap(mux)}
}, gs.TagArg("api"))
```

### 4. Read the Identity in Handlers

Inside a handler, `security.FromContext` returns the verified
`*security.Authentication`. Its `HasAuthority`/`HasAnyAuthority` helpers gate
access on scopes and roles:

```go
a, _ := security.FromContext(r.Context())
if !a.HasAuthority("admin") {
    http.Error(w, "forbidden", http.StatusForbidden)
    return
}
```

## Configuration

All keys live under `spring.security.jwt.<name>`:

| Key | Default | Description |
| --- | --- | --- |
| `issuer` | — | expected `iss` claim; empty disables the check |
| `audience` | — | accepted `aud` values (list); empty disables the check |
| `algorithm` | — | pin a single signing alg (e.g. `RS256`); empty accepts any alg compatible with the key source |
| `secret` | — | shared HMAC secret (HS256/384/512) |
| `public-key` | — | inline PEM RSA/ECDSA public key |
| `public-key-file` | — | path to a PEM public key file |
| `jwks-url` | — | remote JWKS endpoint |
| `jwks-refresh` | `15m` | how long a fetched JWKS is cached before refresh |
| `jwks-timeout` | `10s` | per-fetch HTTP timeout for JWKS |
| `scope-claim` | `scope` | claim carrying granted scopes (space-delimited string or array) |
| `roles-claim` | `roles` | claim carrying granted roles (string or array) |
| `leeway` | `0` | clock-skew tolerance for exp/nbf/iat |
| `required` | `true` | `true` rejects a missing token with 401; `false` passes it through with no identity |

## Core Features

The [example.go](example/example.go) program demonstrates and asserts:

* **Reject missing token** — with `required=true` (default) a request without a
  bearer token gets `401`.
* **Authenticate** — a valid token verifies, and the subject is available via
  `security.FromContext`.
* **Method-level authority** — a handler enforces the `admin` authority and
  returns `403` when it is absent.
* **Reject invalid token** — a garbage or expired token gets `401`.

## Advanced Features

* **Three key sources**: HMAC secret, asymmetric PEM public key (RSA or ECDSA),
  or a remote JWKS endpoint whose keys are fetched at startup, cached, and
  refreshed on interval or on an unknown `kid` (absorbing key rotation).
* **Algorithm-confusion protection**: HMAC algorithms are never accepted for an
  asymmetric key source, blocking the classic "sign the token with the public
  key as an HMAC secret" attack. Pin `algorithm` to lock down further.
* **Framework-agnostic**: because it wraps a plain `http.Handler`, the same
  authenticator works whether gin, echo, hertz, or net/http serves the routes.
* **Optional authentication**: set `required=false` to let unauthenticated
  requests through with no identity attached, deferring the decision to a
  method-level guard.
* **Programmatic validation**: the `*Authenticator` implements
  `security.TokenValidator`, so it can verify a raw token string outside the
  HTTP path (e.g. on gRPC/WebSocket transports).
* **Multiple authenticators**: define several entries under
  `spring.security.jwt.*` and select each by name with `gs.TagArg("...")`.
