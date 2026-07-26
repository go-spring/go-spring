# starter-oauth2-server Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-oauth2-server` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that implements an in-process
OAuth2 / OIDC authorization server. It opens no port; it exposes a
`Handler()` that the application mounts on its existing HTTP server.

## 1. Responsibilities & Boundaries

- **Endpoints:** `/authorize`, `/token`, `/jwks`.
- **Grants:** `authorization_code` (+ PKCE), `client_credentials`,
  `refresh_token`.
- **Out of scope:** user identity / login UI (that is the application's
  `UserAuthFunc`), token *validation* (that is
  `starter-security-jwt`), distributed / clustered session and code
  storage (this starter is single-process).

## 2. Key Decisions

- **Single bean, not `gs.Group`.** An application has one authorization
  server; `clients` is configuration data
  (`clients.<id>.public / secret / redirect-uris / scopes / grant-types`),
  not a bean map. Registration is
  `gs.Provide(newAuthServer, gs.TagArg("${spring.oauth2.server}")) +
  OnProperty(...enabled).HavingValue("true")`.
- **Sign key: exactly one — fail-fast.** HMAC `secret` **or** PEM
  `private-key` / `private-key-file`, never both, never neither
  (`errNoSigningKey` / `errBothSigning`). With HMAC, `/jwks` returns an
  empty set; with asymmetric keys, the public key is published there.
- **Login seam = `UserAuthFunc`.** `func(r) (subject, authorities, ok)`
  is a struct field on the server, not a bean injection. The application
  builds the bean, sets `UserAuthFunc`, then mounts `Handler()` — the
  same "inject and build mux" pattern the jwt example uses. No optional
  bean, no reflective wiring.
- **PKCE forced for public clients.** `public:true` clients must send
  `code_challenge`; verifier / client-secret comparison is constant-time.
  `redirect_uri` is checked against each client's exact whitelist to
  block open redirects.
- **In-memory code and refresh store, single-node.** Lazy expiry plus
  write-time sweep; no background goroutine, so no destroy hook. Refresh
  tokens rotate once per use, and refresh scope can only narrow the
  original grant.

## 3. Utilities Exposed

`GenerateVerifier()` and `Challenge(verifier, method)` are exported so
client SDKs and tests can build PKCE pairs without hand-rolling the
algorithm.

## 4. Constraints

- **Async JWKS bootstrap is a footgun.** The example uses HMAC on purpose
  — with RSA + a JWKS URL back to the same process, a jwt authenticator
  that eagerly pulls JWKS deadlocks against the authorization server
  which has not started serving yet. Ship HMAC for demos; asymmetric is
  for real deployments where the JWKS URL is another service.
- **`errutil.Explain` on config errors.** Signing-key mismatch,
  unknown grant types, missing redirects — all raise clear boot errors
  rather than half-working defaults.
- Internal deps resolve through `go.work`. Third-party dep is
  `golang-jwt/jwt/v5` only.

## 5. Trade-offs / Alternatives Rejected

- **`gs.Group` multiple authorization servers — rejected.** An
  application runs one AS; multi-tenancy is a `clients` map, not a bean
  group.
- **Distributed store (Redis, database) — rejected in v1.** Single-node
  is enough for the "small-app-that-issues-its-own-tokens" case; a
  Contributor pattern lets a durable store starter contribute a
  `CodeStore` bean later without touching this module.
- **Cross-import `starter-security-jwt` in the example — rejected.**
  Would require an internal `require` on another workspace module (404s
  through the proxy). The example inlines a tiny `hmacValidator` that
  satisfies `security.TokenValidator` instead.
