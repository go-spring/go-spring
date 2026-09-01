# starter-oauth2-resource-server

Turns a Go-Spring application into an OAuth2 resource server: it verifies
incoming JWT bearer tokens against a trusted issuer and exposes the result
behind the framework-neutral `security.TokenValidator` seam from
`go-spring.org/cloud/security`.

One `Validator` bean is registered per entry under
`spring.security.oauth2.resource.jwt.<name>` — configuration is the enable
switch, so importing the starter without configuration is inert.

## Features

- **JWT signature verification** — RS256/RS384/RS512, ES256/ES384/ES512,
  PS256/PS384/PS512 and HS256/HS384/HS512, using `golang-jwt/jwt/v5`.
- **Standard claim validation** — `exp`/`nbf` (with clock-skew leeway), and
  `iss`/`aud` when configured.
- **Three key sources** (exactly one per instance, fail-fast otherwise):
  - `issuer-uri` — resolved through OIDC discovery
    (`{issuer-uri}/.well-known/openid-configuration`) to a JWKS endpoint; keys
    are cached and refreshed, and an unknown `kid` triggers an immediate
    reload to absorb key rotation.
  - `public-key` / `public-key-file` — a static RSA/ECDSA PEM.
  - `secret` — a shared HMAC secret.
- **Algorithm-confusion hardening** — HMAC is never accepted for an asymmetric
  key source; `algorithm` optionally pins one algorithm.
- **Authority mapping** — `scope` and `roles` claims (string or array) are
  flattened into `Authentication.Authorities` for `security.Authorize` /
  `security.Require`.

## Quick Start

```properties
spring.security.oauth2.resource.jwt.api.issuer-uri=https://auth.example.com
spring.security.oauth2.resource.jwt.api.audiences=orders-api
```

```go
import (
    "go-spring.org/cloud/security"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-oauth2-resource-server"
)

func init() {
    gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            fmt.Fprintf(w, "hello %s", a.Principal.Subject)
        })
        return &gs.HttpServeMux{
            Handler: security.Chain(security.Authenticate(v, true))(mux),
        }
    }, gs.TagArg("api"))
}
```

## Configuration

Key: `spring.security.oauth2.resource.jwt.<name>.*`

| Property | Default | Description |
| --- | --- | --- |
| `issuer-uri` | — | Issuer identifier; discovered to a JWKS endpoint, also the expected `iss` |
| `issuer` | — | Overrides the expected `iss` claim |
| `public-key` / `public-key-file` | — | Static RSA/ECDSA public key (PEM) |
| `secret` | — | Shared HMAC secret |
| `audiences` | — | Acceptable `aud` values (any-of) |
| `algorithm` | any compatible | Pin one signing algorithm |
| `jwks-refresh` | `15m` | JWKS cache refresh interval |
| `jwks-timeout` | `10s` | Discovery/JWKS fetch timeout |
| `scope-claim` / `roles-claim` | `scope` / `roles` | Claims mapped into authorities |
| `leeway` | `0` | Clock-skew tolerance for `exp`/`nbf`/`iat` |

## Usage

Run the example:

```bash
cd example
./check.sh
```

See [DESIGN.md](DESIGN.md) for the design rationale.

## License

Apache License 2.0
