# starter-oauth2-resource-server Design

## Position

This starter is the assembly layer between the `security` abstraction in
`cloud/security` and the `golang-jwt/jwt/v5` library. The
abstraction owns the seams (`TokenValidator`, `Authenticate`/`Authorize`
middleware, context propagation); this starter owns exactly one thing: turning
configuration into a working JWT `TokenValidator` bean. It exports no port and
holds no server — the application composes `security.Authenticate(v, required)`
onto its own HTTP mux.

## Why a separate starter from starter-security-jwt

`starter-security-jwt` already verifies JWTs under the
`spring.security.jwt` key. This starter exists for the OAuth2 resource-server
workflow specifically:

- **OIDC discovery** — `issuer-uri` alone is enough: the starter fetches
  `.well-known/openid-configuration` and follows its `jwks_uri`, mirroring
  Spring Boot's `spring.security.oauth2.resourceserver.jwt.issuer-uri`
  semantics. No manual JWKS URL.
- **OAuth2 claim semantics** — `audiences` (any-of) and the issuer identifier
  doubling as the expected `iss`, per the OIDC spec.
- **Its own configuration key** per the client-starter config principles:
  same capability, different workflow — the key the user types declares the
  technology choice.

The verification core (parser options, JWKS cache, algorithm-confusion
hardening) follows the proven pattern of starter-security-jwt rather than
sharing code across starter modules; starters are independently versioned and
must not depend on each other.

## Choices

- **JWT library**: `golang-jwt/jwt/v5` — the de-facto maintained Go JWT
  implementation. Signature primitives stay in crypto/stdlib; only parsing,
  claim validation and PEM/JWK decoding are delegated.
- **Key source is exclusive** — exactly one of `issuer-uri`,
  `public-key`/`public-key-file`, `secret`. A resource server that cannot
  decide how to verify a token is misconfigured; failing fast at startup beats
  a runtime 401 investigation.
- **Fail-fast discovery** — the discovery document and the initial JWKS are
  fetched during bean construction, so a wrong `issuer-uri` fails the boot.
- **JWKS caching** — keys are cached for `jwks-refresh`; an unknown `kid`
  forces one immediate reload (key rotation); a failed refresh keeps serving
  the cached keys.
- **Bean shape** — one named bean per config entry, exported as the
  `security.TokenValidator` interface (`gs.As[security.TokenValidator]()`),
  registered via `gs.Module` + `conf.BindEach` so the enable condition is the
  config key itself. The package-global `security.RegisterValidator` registry
  is deliberately not used: registering a live, config-derived validator into
  a process-global map is wrong across tests and restarts (the same reasoning
  documented in `cloud/experimental/session`'s registry).
- **No middleware of its own** — transport composition (401 vs pass-through,
  authority checks) already lives in `security.Authenticate`/`Authorize`;
  duplicating it here would fork the policy.

## Testing

Unit tests cover HS256 happy-path and wrong-secret, RS256/ES256 static PEM,
expired / not-yet-valid / issuer-mismatch / audience-mismatch claims, leeway,
algorithm-confusion rejection, and full issuer-uri discovery against an
`httptest` fake IdP (discovery + JWKS + unknown-kid), plus fail-fast cases
(broken discovery endpoint, zero/multiple key sources). The example's
`check.sh` smoke-tests the assembled application end-to-end with a shared
secret.
