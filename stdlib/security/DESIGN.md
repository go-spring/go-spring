# security Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`security` is the zero-dependency stdlib abstraction for authentication and
authorization. `starter-security-jwt` contributes a JWT `TokenValidator`
without owning a port; `starter-oauth2-server` issues the tokens; business
code sees only `security.*`.

## 1. Responsibilities & Boundaries

- Two questions only: **who is the caller** (`Authentication` on ctx) and
  **may they do this** (`HasAnyAuthority` / `Require` / `Authorize`).
- Not the crypto library. `TokenValidator` is the seam; JWT / opaque-token /
  session-cookie implementations live in starters or the calling app.
- Web filter chain is bundled here because it is plain net/http glue with no
  external dependency, and it is the transport-side counterpart of the
  aspect-side `Require`.
- Not a session library (see `stdlib/session`), not an OAuth2 authorization
  server (see `starter-oauth2-server`).

## 2. Key Abstractions & Seams

- `TokenValidator` — single-method interface driving both `Authenticate`
  middleware and the driver registry. Implementations must be
  concurrent-safe and must return **a non-nil error** for any credential they
  cannot vouch for, rather than an `Authentication` with `Authenticated=false`.
- `RegisterValidator` / `GetValidator` / `MustGetValidator` — driver-registry
  idiom (panic on empty/nil/duplicate), same as `discovery.Register` /
  `resilience.RegisterDriver`.
- `WithAuthentication` / `FromContext` — ctx propagation with an unexported
  key type so nothing else collides.
- `Require(authorities...)` — an `aspect.Interceptor`; reads
  `FromContext(jp.Context)`, returns `ErrUnauthenticated` when missing and
  `ErrForbidden` when authenticated but lacking any authority; otherwise
  `Proceed`. This is the **AOP-equivalent** method guard using the aspect
  chain rather than a bytecode/annotation port.
- `Middleware = func(http.Handler) http.Handler`, `Chain(a,b,c)(h) ==
  a(b(c(h)))` — outermost first. Canonical order for the resource server:
  `Chain(CORS, CSRF, Authenticate, Authorize)`.

## 3. Constraints (do not break)

- **`Authentication` methods are nil-safe** and `!Authenticated` always
  returns false. Downstream code may check `HasAnyAuthority` on the value
  fetched from `FromContext` without a nil guard; do not add fields that
  break this.
- **`Authenticate(v, required=false)`** must let the request through with no
  `Authentication` attached when the token is absent — the "authority
  decision deferred to a later filter" case. An **invalid** token always
  yields 401; a missing token only 401s when `required=true`.
- **CORS wildcard vs credentials**: with `AllowCredentials=true`, do not
  emit `Access-Control-Allow-Origin: *` — the spec forbids it. Echo the
  concrete origin instead, and add `Vary: Origin`.
- **CSRF is double-submit-cookie**: server-side state-free. Safe methods
  seed the cookie; unsafe methods must echo the cookie in the header
  (constant-time compare). It is orthogonal to bearer-token APIs, which are
  not CSRF-prone; do not force it on APIs.
- **Never accept HMAC on an asymmetric-key configuration** in
  `TokenValidator` implementations (algorithm-confusion defence). This
  invariant lives in the starter, not here, but is called out because
  building a validator here without it is a well-known footgun.

## 4. Trade-offs / Alternatives Rejected

- **No Spring Security filter registry**. Ordering is `Chain(...)` order;
  reasoning is explicit and there is no invisible priority system.
- **`Authorize` at the HTTP layer, `Require` at the method layer** — the
  same authority set, two gates. HTTP gates a route, aspect gates a service
  method. Both live in this package so they are consistent.
- **Registry does not resolve validators at request time**. `Authenticate`
  takes a `TokenValidator` value; use the registry to _look up_ a validator
  at wiring time, not on every request.
- **No annotation scanning**. `@PreAuthorize` is replaced by an explicit
  `aspect.NewChain(security.Require(...))` — the AOP-equivalent chain.
