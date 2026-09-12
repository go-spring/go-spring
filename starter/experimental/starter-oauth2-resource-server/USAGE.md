# starter-oauth2-resource-server Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`config.go`, `jwks.go`, `starter.go`, `validator.go`,
`validator_test.go`) and the runnable [example/](example/) (smoke: `example/check.sh`).
**JWT claim semantics are [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519) and
[jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)'s own; discovery is
[OIDC Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html)** — everything
below is go-spring's increment: binding, beans, the fail-fast assembly, key-source
discipline.

**Activation**: one `*Validator` bean per entry under `spring.security.oauth2.resource.jwt.instances.<name>`;
an empty (or absent) map registers nothing — configuration is the enable switch
(starter.go:37-49). **Exactly one key source per entry** (`issuer-uri` / `public-key(-file)` /
`secret`); zero or more than one fails the boot (config.go:94-113).

---

## 1. Complete worked project

An orders API that accepts bearer tokens minted by an authorization server. The example
below uses the HMAC secret mode (as example/ does) so it runs with zero external systems;
the issuer-uri deployment is one key swap (§3). File tree:

```
demo/
├── go.mod
├── main.go
├── router.go
├── token.go          # dev-only local token minter (stand-in for the auth server)
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/golang-jwt/jwt/v5 v5.3.1      // only if you mint dev tokens yourself
    go-spring.org/spring                     v1.3.x
    go-spring.org/cloud                      latest   // security seam lives here
    go-spring.org/starter-oauth2-resource-server latest
    go-spring.org/starter-actuator           latest   // optional: probes
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "demo/router"
)

func main() { gs.Run() }
```

**router.go** — the whole HTTP surface. The starter exports its validator as the
framework-neutral `security.TokenValidator` seam, so the app composes
`security.Authenticate` / `security.Authorize` without importing the starter's types:

```go
package router

import (
    "net/http"

    "go-spring.org/cloud/security"
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-oauth2-resource-server" // registers validators from config
)

func init() {
    // gs.TagArg("api") selects the validator configured under
    // spring.security.oauth2.resource.jwt.instances.api.*.
    gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
        mux := http.NewServeMux()

        // Identity echo: reaching this handler already means the bearer token verified.
        mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            _, _ = w.Write([]byte("hello " + a.Principal.Subject))
        })

        // Route-level authority gate: needs the orders:read scope/authority.
        mux.Handle("/orders", security.Authorize("orders:read")(
            http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
                _, _ = w.Write([]byte("orders ok"))
            })))

        // Identify first, then gate — the security kit's chain order.
        handler := security.Chain(
            security.Authenticate(v, true), // required=true: no token -> 401
            security.Authorize(),           // authenticated-caller fallback
        )(mux)
        return &gs.HttpServeMux{Handler: handler}
    }, gs.TagArg("api"))
}
```

**token.go** — dev-only minter (in production the authorization server issues tokens;
see [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519#section-3)):

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

const secret = "example-shared-secret" // must match ...jwt.api.secret

func mint(subject string, scopes ...string) string {
    claims := jwt.MapClaims{
        "sub":   subject,
        "aud":   "example-api", // must match ...jwt.api.audiences
        "exp":   time.Now().Add(time.Hour).Unix(),
        "scope": scopes, // array form; space-delimited string also accepted
    }
    s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    return s
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- resource server (this starter) ------------------------------------------
# One entry per validator. Exactly ONE key source per entry:
#   issuer-uri  (recommended in production: OIDC discovery -> JWKS, auto-rotation)
#   public-key / public-key-file  (static RSA/ECDSA PEM)
#   secret  (shared HMAC; dev/demo only)
spring.security.oauth2.resource.jwt.instances.api.secret=example-shared-secret

# Accepted "aud" values (any-of). Empty disables audience checking.
spring.security.oauth2.resource.jwt.instances.api.audiences=example-api

# For production with a real issuer, replace the two lines above with:
#   spring.security.oauth2.resource.jwt.instances.api.issuer-uri=https://auth.example.com
# (expected "iss" then defaults to the issuer URI; JWKS auto-discovered)

# --- http server (gs core) ---------------------------------------------------
# The validator owns no port; gs's built-in server serves the provided mux on :9090.
# spring.http.server.addr=:9090    # explicit is better (project convention)

# --- actuator (optional) -----------------------------------------------------
spring.actuator.addr=:9370
```

**Verify** (isomorphic to example/check.sh, which runs the same five assertions
in-process; here with `-manual` and curl):

```bash
go run . -manual &                     # server stays up
TOKEN=$(go run ./cmd/mint alice orders:read)  # tiny main wrapping token.go's mint

curl -i :9090/me                                   # 401 missing bearer token
curl -i -H "Authorization: Bearer $TOKEN" :9090/me # 200 "hello alice"
curl -i :9370/healthz                              # actuator liveness
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-oauth2-resource-server
  └─ gs.Module(gs.OnProperty("spring.security.oauth2.resource.jwt"))   [starter.go:37]
        │   (prefix check: any spring.security.oauth2.resource.jwt.instances.* entry fires it)
gs.Run()
  ├─ bind: each sub-key -> Config (value tags), then Config.source() validated INLINE
  │        (starter.go:39-41) — "no key source"/"multiple key sources" fail before
  │        any bean exists, with the instance name in the error
  ├─ r.Provide(newValidator, gs.ValueArg(c)).Name(name)
  │        .Export(gs.As[security.TokenValidator]())                    [starter.go:43-46]
  ├─ newValidator (validator.go:59-106), per source:
  │     HMAC : keyfunc returns the secret                          — no I/O
  │     PEM  : parsePEMPublicKey (RSA then ECDSA; file beats inline)  — fails fast
  │     Issuer: discoverJWKSURI  GET {issuer-uri}/.well-known/openid-configuration
  │             then newJWKSCache -> initial JWKS fetch              — fails fast:
  │             unreachable issuer, bad PEM or empty key set aborts the boot
  ├─ parser built: jwt.WithValidMethods(...), WithLeeway, WithIssuer when non-empty
  ├─ app wiring: security.TokenValidator injected by name into your mux provider
  ├─ serve; requests flow through Authenticate -> Authorize -> handler
  └─ SIGTERM: nothing to close — the JWKS cache refreshes on demand,
     no background goroutine (jwks.go:35-39, starter comment)
```

### 2.2 One request, layer by layer — `GET /orders` with a bearer token

1. **Authenticate** (security/middleware.go:72): `BearerToken(r)` extracts the
   `Authorization: Bearer <token>` header; missing + `required=true` → 401
   `WWW-Authenticate: Bearer` (writeUnauthorized).
2. **Validator.Validate** (validator.go:189-211): `parser.ParseWithClaims` with the
   source-specific keyfunc:
   - **signature**: algorithm first screened by `WithValidMethods(validMethods)` —
     for an asymmetric source the allowed set is RS/ES/PS only, **never HS***
     (validator.go:110-129), which structurally kills the "sign with the public key
     as an HMAC secret" confusion attack even when `algorithm` is unpinned;
   - **issuer-uri source**: `kid` from the token header selects a cached JWKS key;
     an unknown `kid` or a cache older than `jwks-refresh` triggers one synchronous
     reload (rotation absorbed without waiting for the interval); a failed reload
     still serves a cached key when one exists (jwks.go:66-88);
   - **claims**: `exp`/`nbf` (+`leeway`), `iss` when derived non-empty
     (validator.go:101-103), then `aud` any-of against `audiences`
     (validator.go:198-200) — each failure is an error, never an unauthenticated success.
3. On success an `Authentication{Subject, Claims, Authorities}` is attached to the
   request context; authorities = `scope-claim` + `roles-claim` flattened
   (string-space-delimited or JSON array, validator.go:230-247).
4. **Authorize("orders:read")** (security/middleware.go:102): anonymous → 401;
   authenticated but lacking the authority → 403; else the handler runs.
5. All failure paths return 401 with reason text (`missing bearer token`,
   `invalid token`) — the parser's specific error (expired, wrong issuer, ...) is
   returned by `Validate` but rendered generically at the HTTP edge to avoid
   leaking validation detail.

---

## 3. Per-key behavior reference

Keys under `spring.security.oauth2.resource.jwt.instances.<name>.*` — 12 keys (matches
`grep -rhoE 'value:"[^"]+"' | sort -u`).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `issuer-uri` | string | — | Key source ①. OIDC discovery at `{issuer-uri}/.well-known/openid-configuration` → `jwks_uri` (trailing slash tolerated, validator.go:134-136). Also the expected `iss` unless `issuer` overrides. | Boot fails if discovery/JWKS unreachable at startup. |
| `public-key` | string | — | Key source ②a: inline RSA/ECDSA PEM. | Non-PEM value → boot fails ("neither a valid RSA nor ECDSA PEM"). |
| `public-key-file` | string | — | Key source ②b: PEM file path; **beats `public-key`** when both set (still one source, validator.go:169-175). | Unreadable path → boot fails. |
| `secret` | string | — | Key source ③: HMAC secret for HS256/384/512. | Combined with either source above → boot fails (multiple sources). |
| `issuer` | string | — | Overrides the expected `iss` (e.g. internal issuer differs from public URL). | Wrong value → every token 401 ("invalid token"). |
| `audiences` | []string | — | Accepted `aud` values, **any-of**; empty disables the check. | Token with different `aud` → 401 (`token audience not accepted`). |
| `algorithm` | string | — | Pins one alg (case-insensitive); must be compatible with the source. Empty = full compatible set. HMAC never valid for asymmetric sources. | Incompatible pin (e.g. `HS256` with PEM) → **boot fails** (validator.go:128). |
| `jwks-refresh` | duration | 15m | JWKS cache TTL; unknown `kid` forces an immediate reload too. ⚠ dead for HMAC/PEM sources. | Too long delays rotation pickup to the kid-trigger path only. |
| `jwks-timeout` | duration | 10s | Per-fetch HTTP timeout for discovery **and** JWKS. ⚠ dead for HMAC/PEM sources. | Too low → refresh failures (stale keys served) / boot flakiness. |
| `scope-claim` | string | `scope` | Claim flattened into Authorities (space-delimited or array). | Wrong name → scope-based Authorize gates start returning 403. |
| `roles-claim` | string | `roles` | Same, appended after scopes. | Same. |
| `leeway` | duration | 0 | Clock-skew tolerance for exp/nbf/iat. | Too large accepts expired tokens; 0 rejects borderline clocks. |

⚠ Coupling: `{issuer-uri}` XOR `{public-key, public-key-file}` XOR `{secret}` — exactly
one group; `public-key` + `public-key-file` together count as ONE source (file wins).
⚠ `algorithm` must belong to the chosen source's family.

---

## 4. Verification & fault drills

All drills run against the §1 project (`go run . -manual`). Mint helper as in token.go.

1. **Round trip**: valid token → `200 hello alice`; scoped token → `200 orders ok`.
2. **Missing token**:
   `curl -s -o/dev/null -w '%{http_code}\n' :9090/me` → `401`, body `missing bearer token`,
   header `WWW-Authenticate: Bearer`.
3. **Wrong key source (bad secret)** — mint with `[]byte("wrong")`:
   `curl -s -H "Authorization: Bearer $FORGED" :9090/me` → `401 invalid token`
   (example/check.sh Feature 5 asserts exactly this).
4. **Expired token** — mint with `"exp": time.Now().Add(-time.Minute).Unix()`:
   → `401`. Re-mint with `+30s` expiry and `spring.security...api.leeway=1m` set:
   now accepted — leeway in action.
5. **Alg-confusion attempt** — with a `public-key` source configured, mint an HS256
   token signed with the *public* PEM as the HMAC secret:
   still `401` — `validMethods` never admits HS* for an asymmetric source
   (validator.go:113-119), and pinning isn't required for this protection.
   Configuring `algorithm=HS256` alongside a PEM source fails at **boot** instead.
6. **Missing issuer/audience** — with `issuer-uri=https://auth.example.com`, mint a
   token whose `iss` is anything else → `401`; drop the `aud` claim with
   `audiences=example-api` set → `401` (no `aud` satisfies any-of).
7. **Key-source misconfig** — set both `secret` and `issuer-uri` (or neither), restart:
   boot aborts with `multiple/no verification key source configured ... instance "api"`.
8. **JWKS rotation** (issuer-uri deployments): serve a JWKS with kid `k1`, verify a
   `k1` token; publish `k2` only, mint a `k2` token — first request triggers an
   on-demand reload (jwks.go:66-88) and succeeds without restart.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot: `no verification key source configured` | no key entry under the instance | set `issuer-uri`, `public-key(-file)` or `secret` |
| Boot: `multiple verification key sources configured` | e.g. `secret` + `issuer-uri` both set | keep exactly one group |
| Boot: `fetch discovery document ... status ...` | issuer down / wrong URL / TLS | fix `issuer-uri`; check `jwks-timeout` |
| Boot: `JWKS ... contains no usable key` | endpoint serves only okp/ed25519 keys or garbage | only RSA + EC P-256/384/521 JWKs are parsed (jwks.go:147-156) |
| Every request 401 `invalid token` | issuer/audience/algorithm mismatch or wrong key | verify `iss` vs `issuer-uri`, `aud` vs `audiences`, minter key |
| 403 on scope-gated routes after token change | authority claim renamed or scope-claim mismatch | check token's claim names vs `scope-claim`/`roles-claim` |
| Tokens from a freshly rotated key 401 briefly | cache not yet stale | unknown `kid` already forces reload; verify the new key is actually published |
| `algorithm ... not compatible` at boot | pin contradicts key source family | align `algorithm` with the source (RS/ES/PS for PEM/issuer, HS for secret) |
| Nothing registers at all (no validator bean) | prefix typo / empty map | keys must sit under `spring.security.oauth2.resource.jwt.instances.<name>.*` |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 12 |
| Required | 1 key source per instance |
| Quickstart external deps | 0 (HMAC mode); 1 OIDC issuer for production mode |
| "Watch out" entries | 5 |

Design suspects (kept + new, for the audit ledger):

- **Kept**: heavy overlap with `starter-security-jwt` — same parser/JWKS/validation
  core, differing mainly in prefix + `issuer-uri` discovery; shared-validator-module
  candidate.
- New: `jwks.go` is a byte-for-byte duplicate (module prefix excepted) of the jwt
  starter's — merge into a common internal package.
- New: a failed JWKS reload serves stale cached keys with **no bound on staleness**
  (jwks.go:75-79) — a long outage silently keeps old keys authoritative.
- New: no auth-failure observability (no counter/log on 401 paths) — security posture
  is invisible in metrics.
- New: `algorithm` case-insensitive but `audiences` any-of and leeway default 0 are
  silent foot-guns for multi-clock deployments; nothing surfaces "why 401" beyond the
  generic body text.
- New: `Authenticate`'s `required` lives app-side here, while the jwt starter has a
  config `required` key — inconsistent posture knobs between sibling starters.
