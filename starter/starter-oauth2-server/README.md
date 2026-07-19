# starter-oauth2-server

[English](README.md) | [中文](README_CN.md)

`starter-oauth2-server` turns a Go-Spring application into an OAuth2/OIDC
**authorization server**: it issues tokens through the standard `/authorize`,
`/token` and `/jwks` endpoints and supports the `authorization_code` (with
**PKCE**), `client_credentials` and `refresh_token` grants. It is the
server-side counterpart to `starter-oauth2-client` (which already covers the
client side) and pairs with `starter-security-jwt` on the resource-server side:
tokens this server signs are verified there using shared key material.

It is a **Contributor**-form starter — it opens no listener of its own. The
application injects the `*AuthServer` bean and mounts its `Handler()` onto the
HTTP server it already runs.

## Scope

This starter implements the OAuth2/OIDC **protocol endpoints**, not a full
identity provider. It ships no user store, no MFA, and no social-login
aggregation; the resource-owner login is a seam (`UserAuthFunc`) the
application plugs its own session/login into. It is not a port of Spring
Security's `SecurityFilterChain` DSL — the equivalent Web-security filter chain
is assembled from ordinary `net/http` middleware in `stdlib/security`
(`Chain` / `CORS` / `CSRF` / `Authenticate` / `Authorize`).

## Installation

```bash
go get go-spring.org/starter-oauth2-server
```

## Quick Start

### 1. Import the Package

```go
import _ "go-spring.org/starter-oauth2-server"
```

### 2. Configure the Server and Its Clients

Add configuration in your project's [configuration file](example/conf/app.properties).
Exactly one signing key must be set — a shared HMAC `secret` (verified by the
resource server with the same secret) or a PEM `private-key` / `private-key-file`
(whose public half is published at `/jwks`):

```properties
spring.oauth2.server.enabled=true
spring.oauth2.server.issuer=https://issuer.example.com
spring.oauth2.server.secret=example-shared-secret

# A public client (SPA / native app): no secret, PKCE mandatory.
spring.oauth2.server.clients.spa.public=true
spring.oauth2.server.clients.spa.redirect-uris=http://127.0.0.1:9090/callback
spring.oauth2.server.clients.spa.scopes=read,write

# A confidential client restricted to the client_credentials grant.
spring.oauth2.server.clients.svc.secret=svc-secret
spring.oauth2.server.clients.svc.scopes=read
spring.oauth2.server.clients.svc.grant-types=client_credentials
```

### 3. Mount the Endpoints and Wire the Login Seam

Inject the `*AuthServer`, set `UserAuthFunc` (the resource-owner login), and
mount `Handler()` onto your HTTP server. Refer to the [example.go](example/example.go) file.

```go
gs.Provide(func(as *StarterOAuth2Server.AuthServer) *gs.HttpServeMux {
    // Plug in your own login: return the subject and authorities to grant.
    as.UserAuthFunc = func(r *http.Request) (string, []string, bool) {
        return "alice", []string{"admin"}, true
    }
    mux := http.NewServeMux()
    mux.Handle("/oauth2/", http.StripPrefix("/oauth2", as.Handler()))
    return &gs.HttpServeMux{Handler: mux}
})
```

### 4. Protect Resources with the Filter Chain

The unified Web-security filter chain lives in `stdlib/security`. Compose the
concerns in order — CORS, then authentication, then authorization — with
`security.Chain`:

```go
validator := /* a security.TokenValidator, e.g. a starter-security-jwt Authenticator */
mux.Handle("/api/admin", security.Chain(
    security.CORS(security.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}),
    security.Authenticate(validator, true), // authenticate the bearer token
    security.Authorize("admin"),            // require the "admin" authority
)(businessHandler))
```

## Endpoints

| Endpoint         | Method   | Purpose                                                        |
|------------------|----------|----------------------------------------------------------------|
| `/authorize`     | GET      | Authorization endpoint of the `authorization_code` grant.      |
| `/token`         | POST     | Token endpoint for all three grants.                           |
| `/jwks`          | GET      | Publishes verification keys (empty for an HMAC signing key).   |

## Grants

- **`authorization_code` (+ PKCE)** — the user is authenticated via
  `UserAuthFunc`, a single-use code is redirected back to the client, and the
  client redeems it at `/token`. **PKCE is mandatory for public clients**
  (`public: true`); the `code_verifier` presented at `/token` is checked against
  the `code_challenge` captured at `/authorize`.
- **`client_credentials`** — a confidential client authenticates with its
  secret and receives an access token acting on its own behalf (no refresh
  token).
- **`refresh_token`** — a refresh token is rotated (single use) for a fresh
  access token; the request may narrow, but not widen, the granted scopes.

## Security Notes

- The signing key is shared with the resource server by configuration; the two
  never need to talk directly for HMAC, and for asymmetric keys the resource
  server fetches `/jwks`.
- `redirect_uri` is validated against the per-client allow-list before it is
  ever used as a redirect target, closing the open-redirect vector.
- Client secrets and PKCE verifiers are compared in constant time.
- The authorization codes and refresh tokens are held in process memory
  (single-node); a multi-node deployment should front this with a shared store.

## Design

For the design constraints every official starter follows, see
[DESIGN.md](../DESIGN.md).
