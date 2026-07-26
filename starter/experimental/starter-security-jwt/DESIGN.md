# starter-security-jwt Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-security-jwt` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that implements the JWT
resource-server side of Spring Security's authentication story on top of
the zero-dep `spring/security` abstraction. It opens no port; it mounts as
middleware onto an existing `*gs.HttpServeMux`.

## 1. Responsibilities & Boundaries

- **In scope:** decode and verify bearer JWTs, three key sources
  (HMAC secret, PEM public key, JWKS URL), map claims to
  `security.Authentication`, expose a `Wrap(next http.Handler)` seam and
  a matching `security.TokenValidator` for non-HTTP transports.
- **Out of scope:** issuing tokens (that is `starter-oauth2-server`);
  authorization *policy* (that is aspect / middleware sugar in
  `spring/security`); user store / login UI.

## 2. Key Abstractions & Seams

- **`spring/security` — zero-dependency abstraction.** The starter is one
  concrete implementation of that shape. `TokenValidator` is the driver
  seam (registry-style — the same pattern `spring/discovery` and
  `spring/resilience` use). `Principal` + `Authentication` are neutral
  types with `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities`
  helpers, all nil-safe and short-circuit `false` when not authenticated.
- **Two mount points from one bean.**
  - `Wrap(next http.Handler) http.Handler` — HTTP middleware seam, mirrors
    `starter-lua-filter` so no framework coupling.
  - `security.TokenValidator` — the same `*Authenticator` also satisfies
    the transport-neutral validator, reusable from gRPC metadata,
    WebSocket handshake, etc.
- **Multi-instance via `gs.Group("${spring.security.jwt}", ...)`.** An
  application can validate tokens from more than one issuer by adding
  entries to the map. Destroy is `nil` (no goroutine, JWKS cache refreshes
  on demand).

## 3. Constraints

- **Exactly one key source per instance — fail-fast.** HMAC `secret`, PEM
  `public-key` / `public-key-file`, or JWKS `jwks-url`. Two configured =
  boot error; zero configured = boot error.
- **Algorithm-confusion protection.** An asymmetric source never accepts
  HMAC algorithms — this blocks the classic "sign with the public key as
  an HMAC secret" attack. `validMethods` is enforced against
  `golang-jwt/jwt/v5`'s parser.
- **JWKS parsed in-tree**, not through an external `keyfunc` wrapper. RSA
  `n`/`e` and EC `crv`/`x`/`y` are decoded from base64url directly, so the
  dependency graph stays limited to `golang-jwt/jwt/v5`. The cache
  refreshes on its configured interval and on an unknown-`kid` miss.
- **No name at construction time.** `gs.Group`'s constructor does not
  receive the bean name (a limitation of `gs.go:488`), so a per-instance
  `RegisterValidator(name)` would need a runtime hack. Instead, callers
  wire the `*Authenticator` directly via DI when they need to name it.
- Internal deps resolve through `go.work`; do not run `go mod tidy`.
  External dep is `github.com/golang-jwt/jwt/v5` only.

## 4. Trade-offs / Alternatives Rejected

- **`MicahParks/keyfunc` for JWKS — rejected.** A ~few hundred lines of
  in-tree parsing keeps the transitive dependency graph small.
- **Server-archetype (own port) — rejected.** Authentication has no
  intrinsic port; running a second listener would just multiplex the
  application's own routes.
- **Force-tie to a specific web framework — rejected.** `Wrap` and
  `TokenValidator` are the only two seams, so any framework that ends in
  `http.Handler` (all Go web frameworks do) works unchanged.
