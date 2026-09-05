# security Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`security` is the zero-dependency abstraction for authentication and
authorization. `starter-security-jwt` contributes a JWT `TokenValidator`
without owning a port; `starter-oauth2-server` issues the tokens; business
code sees only `security.*`.

## 1. Responsibilities & Boundaries

- Two questions only: **who is the caller** (`Authentication` on ctx) and
  **may they do this** (`HasAnyAuthority` / `Require`).
- Not the crypto library. `TokenValidator` is the seam; JWT / opaque-token /
  session-cookie implementations live in starters or the calling app.
- No HTTP middleware lives here. Each server family installs its own shell in
  its own idiom — stdlib decorators in `starter-http-server`,
  `gin.HandlerFunc` in `starter-gin`, `echo.MiddlewareFunc` in
  `starter-echo` — all on top of this identity model. The security-sensitive
  logic those shells share is exposed as pure helpers (`ParseBearerToken`,
  `NewCSRFToken` / `MatchCSRFToken` with constant-time compare) so behavior
  cannot drift between families.
- Not a session library (see `cloud/session`), not an OAuth2 authorization
  server (see `starter-oauth2-server`).

## 2. Key Abstractions & Seams

- `TokenValidator` — single-method interface driving both the family
  middlewares and the driver registry. Implementations must be
  concurrent-safe and must return **a non-nil error** for any credential they
  cannot vouch for, rather than an `Authentication` with `Authenticated=false`.
- `RegisterValidator` / `GetValidator` / `MustGetValidator` — driver-registry
  idiom (panic on empty/nil/duplicate), same as `discovery.Register` /
  `resilience.RegisterDriver`.
- `WithAuthentication` / `FromContext` — ctx propagation with an unexported
  key type so nothing else collides.
- `Require(authorities...)` — a plain decorator; reads
  `FromContext(jp.Context)`, returns `ErrUnauthenticated` when missing and
  `ErrForbidden` when authenticated but lacking any authority; otherwise
  `Proceed`. This is the **AOP-equivalent** method guard using an ordinary
  chain rather than a bytecode/annotation port.

## 3. Constraints (do not break)

- **`Authentication` methods are nil-safe** and `!Authenticated` always
  returns false. Downstream code may check `HasAnyAuthority` on the value
  fetched from `FromContext` without a nil guard; do not add fields that
  break this.
- **`Authenticate(v, required=false)`** (in every family shell) must let the
  request through with no `Authentication` attached when the token is absent
  — the "authority decision deferred to a later filter" case. An **invalid**
  token always yields 401; a missing token only 401s when `required=true`.
- **CORS wildcard vs credentials**: with `AllowCredentials=true`, do not
  emit `Access-Control-Allow-Origin: *` — the spec forbids it. Echo the
  concrete origin instead, and add `Vary: Origin`.
- **CSRF is double-submit-cookie**: server-side state-free. Safe methods
  seed the cookie; unsafe methods must echo the cookie in the header via
  `MatchCSRFToken` (constant-time). It is orthogonal to bearer-token APIs,
  which are not CSRF-prone; do not force it on APIs.
- **Never accept HMAC on an asymmetric-key configuration** in
  `TokenValidator` implementations (algorithm-confusion defence). This
  invariant lives in the starter, not here, but is called out because
  building a validator here without it is a well-known footgun.

## 4. Trade-offs / Alternatives Rejected

- **No shared HTTP middleware protocol**. A shared
  `func(http.Handler) http.Handler` layer fit only net/http and forced
  gin/echo through adapters (hertz not at all); it was removed in favor of
  per-family shells. The cost of duplicated shells is bounded by sharing the
  pure helpers and the identity model; the win is that each family reads
  natively.
- **No Spring Security filter registry**. Ordering is the shell's chain
  order; reasoning is explicit and there is no invisible priority system.
- **HTTP-layer `Authorize` / route gate lives with each family**, the
  method-layer `Require` lives here — the same authority set, two gates: a
  route gate in the server's idiom, a decorator gate in this package.
- **Registry does not resolve validators at request time**. The middlewares
  take a `TokenValidator` value; use the registry to _look up_ a validator
  at wiring time, not on every request.
- **No annotation scanning**. `@PreAuthorize` is replaced by an explicit
  `security.Require(...)` wrapping the call — the AOP-equivalent decorator.
