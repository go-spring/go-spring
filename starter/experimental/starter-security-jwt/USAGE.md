# starter-security-jwt Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`config.go`, `jwks.go`, `jwt.go`, `starter.go`, `jwt_test.go`)
and the runnable [example/](example/) (smoke: `example/check.sh`). **JWT claim semantics
are [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519) and
[jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)'s own** — everything below is
go-spring's increment: binding, beans, the `Wrap(http.Handler)` seam and the fail-fast
key-source discipline.

**Activation**: one `*Authenticator` bean per entry under `spring.security.jwt.<name>`;
an empty (or absent) map registers nothing — configuration is the enable switch
(starter.go:35). **Exactly one verification key source per entry** (`secret` /
`public-key(-file)` / `jwks-url`); zero or more than one fails the boot
(config.go:97-116). This starter **verifies** tokens; it does not mint them — pair it
with your issuer (or, for demos, mint with jwt/v5 in a dev binary as example/ does; see
also sibling [starter-oauth2-resource-server](../starter-oauth2-resource-server/) for the
discovery-based variant).

---

## 1. Complete worked project

A small API where `/me` echoes the verified identity and `/admin` requires an `admin`
authority. HMAC mode keeps it dependency-free; production swaps in `jwks-url` (§3).
File tree:

```
demo/
├── go.mod
├── main.go
├── router.go
├── token.go          # dev-only token minter (stand-in for the identity provider)
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/golang-jwt/jwt/v5 v5.3.1      // only if you mint dev tokens yourself
    go-spring.org/spring                     v1.3.x
    go-spring.org/cloud                      latest   // security seam lives here
    go-spring.org/starter-security-jwt       latest
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

**router.go** — the whole HTTP surface. `Wrap` is the seam: it decorates a plain
`http.Handler`, so any framework engine (gin/echo/net-http) can sit behind it:

```go
package router

import (
    "net/http"

    "go-spring.org/cloud/experimental/security"
    "go-spring.org/spring/gs"
    StarterSecurityJWT "go-spring.org/starter-security-jwt"
)

func init() {
    // gs.TagArg("api") selects the authenticator configured under
    // spring.security.jwt.api.*. gs registers its default HttpServeMux only when
    // none is provided, so this custom one wins.
    gs.Provide(func(auth *StarterSecurityJWT.Authenticator) *gs.HttpServeMux {
        mux := http.NewServeMux()

        // Reaching this handler means the bearer token already verified in Wrap.
        mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            _, _ = w.Write([]byte("hello " + a.Principal.Subject))
        })

        // Method-level authority check on top of authentication.
        mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            if !a.HasAuthority("admin") {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            _, _ = w.Write([]byte("admin ok"))
        })

        return &gs.HttpServeMux{Handler: auth.Wrap(mux)}
    }, gs.TagArg("api"))
}
```

**token.go** — dev-only minter (production tokens come from your identity provider;
see [RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519#section-3)):

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

const secret = "example-shared-secret" // must match spring.security.jwt.api.secret

func mint(subject string, roles ...string) string {
    claims := jwt.MapClaims{
        "sub":   subject,
        "exp":   time.Now().Add(time.Hour).Unix(),
        "roles": roles, // matches roles-claim; array form, space-delimited also works
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
# --- security-jwt (this starter) ---------------------------------------------
# One entry per authenticator. Exactly ONE verification key source per entry:
#   secret                (shared HMAC; dev/demo)
#   public-key / public-key-file  (static RSA/ECDSA PEM)
#   jwks-url              (remote JWKS; keys fetched at startup, rotated on kid)
spring.security.jwt.api.secret=example-shared-secret

# Claim carrying roles; flattened into Authentication.Authorities.
spring.security.jwt.api.roles-claim=roles

# true (default): missing token -> 401. false: pass through with no identity,
# letting method-level guards decide.
spring.security.jwt.api.required=true

# For a remote issuer, replace `secret` with e.g.:
#   spring.security.jwt.api.jwks-url=https://auth.example.com/.well-known/jwks.json
#   spring.security.jwt.api.issuer=https://auth.example.com
#   spring.security.jwt.api.audience=demo-api

# --- http server (gs core) ---------------------------------------------------
# The authenticator owns no port; gs's built-in server serves the provided mux on :9090.
# spring.http.server.addr=:9090    # explicit is better (project convention)

# --- actuator (optional) -----------------------------------------------------
spring.actuator.addr=:9370
```

**Verify** (isomorphic to example/check.sh, which runs the same five assertions
in-process; here with `-manual` and curl):

```bash
go run . -manual &                                # server stays up
TOKEN=$(go run ./cmd/mint alice user)             # tiny main wrapping token.go's mint
ADMIN=$(go run ./cmd/mint root admin)

curl -i :9090/me                                     # 401 missing bearer token
curl -i -H "Authorization: Bearer $TOKEN" :9090/me   # 200 "hello alice"
curl -i -H "Authorization: Bearer $TOKEN" :9090/admin # 403 forbidden
curl -i -H "Authorization: Bearer $ADMIN" :9090/admin # 200 "admin ok"
curl -i :9370/healthz                                # actuator liveness
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-security-jwt
  └─ gs.Group("${spring.security.jwt}", newAuthenticator, nil)      [starter.go:35]
gs.Run()
  ├─ bind: each sub-key -> Config (value tags)
  ├─ newAuthenticator (jwt.go:57-103), per source — all fail fast:
  │     source(): exactly one of secret / PEM / jwks-url, else boot error  [config.go:97-116]
  │     HMAC : keyfunc returns the secret                          — no I/O
  │     PEM  : parsePEMPublicKey (RSA then ECDSA; file beats inline)  [jwt.go:130-146]
  │     JWKS : newJWKSCache -> initial fetch at startup            — unreachable
  │             endpoint or empty key set aborts the boot           [jwks.go:52-62]
  ├─ parser built: jwt.WithValidMethods(...), WithLeeway, WithIssuer when non-empty
  ├─ app wiring: *Authenticator injected by name (gs.TagArg) into your mux provider;
  │     Wrap decorates the business mux -> *gs.HttpServeMux replaces gs's default
  ├─ serve; each request: Wrap -> (optional) JWKS key lookup -> Validate -> handler
  └─ SIGTERM: nothing to close — the JWKS cache refreshes on demand with no
     background goroutine, so the group's destroy hook is nil   [starter.go:33-34]
```

### 2.2 One request, layer by layer — `GET /me` with a bearer token

1. **Wrap** (jwt.go:182-200): `bearerToken(r)` extracts `Authorization: Bearer <token>`
   (case-insensitive prefix, jwt.go:204-211).
2. No token: `Required` (default true) → 401 `missing bearer token` with
   `WWW-Authenticate: Bearer error="invalid_token"`; false → pass through with **no**
   Authentication attached, deferring to method-level guards.
3. Token present → `Validate` (jwt.go:150-172): `parser.ParseWithClaims`:
   - **algorithm screening first**: `WithValidMethods(validMethods)` — for an
     asymmetric source the allowed set is RS256/384/512, ES256/384/512, PS256/384/512
     only, **never HS*** (jwt.go:110-126). This structurally blocks the classic
     algorithm-confusion attack ("sign with the public key as an HMAC secret") even
     when `algorithm` is left unpinned;
   - **key resolution**: HMAC → the configured secret; PEM → the parsed key; JWKS →
     `kid` header selects a cached key; unknown `kid` or a cache older than
     `jwks-refresh` triggers one synchronous reload (rotation absorbed without waiting
     for the interval); a failed reload still serves the cached key when one exists
     (jwks.go:66-88);
   - **claims**: signature, `exp`/`nbf` (+`leeway`), `iss` when configured
     (jwt.go:98-100), then `aud` any-of against `audience` (jwt.go:159-161). Any
     failure → error, never an unauthenticated success.
4. Success: `Authentication{Subject, Claims, Authorities}` attached to the request
   context via `security.WithAuthentication` (jwt.go:198); authorities =
   `scope-claim` + `roles-claim` flattened (space-delimited string or JSON array,
   jwt.go:236-253).
5. Handler reads it with `security.FromContext` and gates with
   `HasAuthority`/`HasAnyAuthority` (security.go:81-107). Invalid tokens always yield
   401 `invalid token` — the parser's specific reason is not leaked to the client.

---

## 3. Per-key behavior reference

Keys under `spring.security.jwt.<name>.*` — 13 keys (matches
`grep -rhoE 'value:"[^"]+"' | sort -u`).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `secret` | string | — | Key source ①: HMAC secret for HS256/384/512. | Combined with another source → boot fails (multiple sources). |
| `public-key` | string | — | Key source ②a: inline RSA/ECDSA PEM. | Non-PEM → boot fails ("neither a valid RSA nor ECDSA PEM"). |
| `public-key-file` | string | — | Key source ②b: PEM file path; **beats `public-key`** when both set (still one source, jwt.go:131-138). | Unreadable path → boot fails. |
| `jwks-url` | string | — | Key source ③: remote JWKS endpoint; fetched eagerly at startup, `kid` selects the key. | Unreachable / empty key set → boot fails. |
| `jwks-refresh` | duration | 15m | JWKS cache TTL; unknown `kid` forces an immediate reload too. ⚠ dead for HMAC/PEM sources. | Too long delays rotation pickup to the kid-trigger path only. |
| `jwks-timeout` | duration | 10s | Per-fetch JWKS HTTP timeout. ⚠ dead for HMAC/PEM sources. | Too low → refresh failures (stale keys served) / boot flakiness. |
| `issuer` | string | — | Expected `iss`; empty **disables** issuer checking. | Wrong value → every token 401. |
| `audience` | []string | — | Accepted `aud` values, **any-of**; empty disables the check. | Token with different/missing `aud` → 401 (`token audience not accepted`). |
| `algorithm` | string | — | Pins one alg (case-insensitive); must be compatible with the source. Empty = full compatible set. HMAC never valid for asymmetric sources. | Incompatible pin (e.g. `HS256` with PEM) → **boot fails** (jwt.go:125). |
| `scope-claim` | string | `scope` | Claim flattened into Authorities (space-delimited or array). | Wrong name → scope-based guards start returning 403. |
| `roles-claim` | string | `roles` | Same, appended after scopes. | Same. |
| `leeway` | duration | 0 | Clock-skew tolerance for exp/nbf/iat. | Too large accepts expired tokens; 0 rejects borderline clocks. |
| `required` | bool | true | false = missing token passes through with no identity (Wrap, jwt.go:185-191). An **invalid** token still 401s regardless. | false + no downstream guard → endpoints silently anonymous. |

⚠ Coupling: `{secret}` XOR `{public-key, public-key-file}` XOR `{jwks-url}` — exactly
one group; `public-key` + `public-key-file` together count as ONE source (file wins).
⚠ `algorithm` must belong to the chosen source's family.

---

## 4. Verification & fault drills

All drills run against the §1 project (`go run . -manual`); mint via token.go.

1. **Round trip**: user token → `200 hello alice`; admin token → `200 admin ok`;
   user token on `/admin` → `403` (example/check.sh asserts all three).
2. **Missing token**: `curl -s -o/dev/null -w '%{http_code}\n' :9090/me` → `401`,
   body `missing bearer token`, header `WWW-Authenticate: Bearer error="invalid_token"`.
3. **Garbage/invalid token**: `curl -s -H "Authorization: Bearer not-a-real-token" :9090/me`
   → `401 invalid token` (check.sh Feature 5). Also mint with a wrong secret
   (`[]byte("wrong")`) → same 401.
4. **Expired token** — mint with `"exp": time.Now().Add(-time.Minute).Unix()`:
   → `401`. Re-mint with `+30s` expiry and set `spring.security.jwt.api.leeway=1m`:
   now accepted — leeway in action.
5. **Alg-confusion attempt rejected** — configure a `public-key` source, then mint an
   HS256 token signed with the **public PEM itself as the HMAC key**:
   `curl -s -H "Authorization: Bearer $CONFUSED" :9090/me` → `401` — `validMethods`
   never admits HS* for an asymmetric source (jwt.go:113-119), pin or no pin.
   Configuring `algorithm=HS256` next to a PEM source fails at **boot** instead
   (`algorithm "HS256" is not compatible ...`).
6. **Missing issuer/audience** — set `issuer=https://auth.example.com` and mint a
   token with a different `iss` → `401`; set `audience=demo-api` and mint without an
   `aud` claim → `401` (no aud satisfies any-of).
7. **Wrong key source / JWKS drill** — swap `secret` for
   `jwks-url=http://127.0.0.1:8080/jwks` with nothing listening, restart: boot
   aborts (`fetch JWKS ...`). Then serve a JWKS with kid `k1`, verify a `k1` token;
   publish `k2` only and mint a `k2` token — the first request triggers an on-demand
   reload (jwks.go:66-88) and succeeds without restart.
8. **required=false posture** — set `spring.security.jwt.api.required=false`, restart:
   `curl -s :9090/me` no longer 401s (handler sees no identity — guard it, e.g. with
   `a, ok := security.FromContext(...); !ok` → 401). A garbage token still 401s.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot: `no verification key source configured` | no key entry under the instance | set `secret`, `public-key(-file)` or `jwks-url` |
| Boot: `multiple verification key sources configured` | e.g. `secret` + `jwks-url` both set | keep exactly one group |
| Boot: `fetch JWKS ...: status ...` / `contains no usable key` | endpoint down, wrong URL, or only okp/ed25519 keys | fix `jwks-url`; only RSA + EC P-256/384/521 JWKs are parsed (jwks.go:147-156) |
| Boot: `public key is neither a valid RSA nor ECDSA PEM` | inline/file PEM invalid | check the PEM block format |
| Every request 401 `invalid token` | issuer/audience/algorithm mismatch or wrong key | verify `iss` vs `issuer`, `aud` vs `audience`, minter key |
| 403 everywhere after a token format change | authority claim renamed or claim-name mismatch | check token claim names vs `scope-claim`/`roles-claim` |
| Anonymous access where 401 expected | `required=false` without a downstream guard | re-enable `required` or guard with `FromContext` |
| Freshly rotated keys 401 briefly | JWKS cache not yet stale | unknown `kid` already forces reload; confirm the new key is published |
| Boot: `algorithm ... not compatible` | pin contradicts key source family | align `algorithm` with the source (RS/ES/PS for PEM/JWKS, HS for secret) |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 13 |
| Required | 1 key source per instance |
| Quickstart external deps | 0 (example mints its own token) |
| "Watch out" entries | 5 |

Design suspects (kept + new, for the audit ledger):

- **Kept**: near-total overlap with `starter-oauth2-resource-server` — same parser,
  JWKS cache and validation core; that one adds `issuer-uri` discovery under a
  different prefix; a shared-validator-module candidate.
- New: `jwks.go` duplicates the resource-server starter's file byte-for-byte (module
  prefix excepted) — extract a common internal package.
- New: a failed JWKS reload serves stale cached keys with **no staleness bound**
  (jwks.go:75-79) — a long outage silently keeps old keys authoritative.
- New: no auth-failure observability (no counter/log on 401 paths).
- New: `WWW-Authenticate` always says `error="invalid_token"` even for *missing*
  tokens (jwt.go:214-217) — technically the missing-token case should carry no error
  param per RFC 6750; cosmetic but spec-visible.
- New: `required` exists here as a config key but the sibling starter pushes the same
  knob app-side (`security.Authenticate(v, required)`) — inconsistent posture knobs.
