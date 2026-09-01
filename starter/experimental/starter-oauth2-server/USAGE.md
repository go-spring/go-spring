# starter-oauth2-server Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `server.go`, `token.go`, `store.go`,
`pkce.go`) and the runnable [example/](example/). **OAuth2/OIDC protocol semantics are the
RFCs' own — [RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749) (framework, token
endpoint error codes) and [RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636) (PKCE);
JWT/JWKS formats are [RFC 7515/7517](https://datatracker.ietf.org/doc/html/rfc7517)** —
everything below is go-spring's increment: activation, binding, the mount, the seams.

**Activation**: exactly one switch — `spring.oauth2.server.enabled=true`. Importing the
starter without it is inert. Single-bean model: one authorization server per application;
its clients are config data, not beans.

---

## 1. Complete worked project

A single service that is both the authorization server (under `/oauth2`) and the resource
server (business API under `/api`), issuing and then accepting its own HS256 tokens. This
is exactly what the example runs — no external identity provider.

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/golang-jwt/jwt/v5     v5.3.1
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-oauth2-server latest
    // optional ecosystem: starter-actuator / starter-otel / starter-governance
)
```

**main.go** — mount, login seam, and the resource-side validator:

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
    "go-spring.org/cloud/security"
    "go-spring.org/spring/gs"
    StarterOAuth2Server "go-spring.org/starter-oauth2-server"
)

const secret = "example-shared-secret" // = spring.oauth2.server.secret

func main() {
    // The app builds its single mux around the injected *AuthServer.
    gs.Provide(func(as *StarterOAuth2Server.AuthServer) *gs.HttpServeMux {
        // The resource-owner login seam: a real app checks its session here.
        as.UserAuthFunc = func(*http.Request) (string, []string, bool) {
            return "alice", []string{"admin"}, true
        }

        validator := hmacValidator{secret: []byte(secret)}

        mux := http.NewServeMux()
        // Authorization server: /oauth2/authorize, /oauth2/token, /oauth2/jwks.
        mux.Handle("/oauth2/", http.StripPrefix("/oauth2", as.Handler()))

        // Resource server: authenticate before authorize (ordered chain).
        mux.Handle("/api/me", security.Chain(
            security.Authenticate(validator, true))(
            http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                a, _ := security.FromContext(r.Context())
                fmt.Fprintf(w, "hello %s", a.Principal.Subject)
            })))
        mux.Handle("/api/admin", security.Chain(
            security.Authenticate(validator, true), security.Authorize("admin"))(
            http.HandlerFunc(func(w http.ResponseWriter, *http.Request) {
                w.Write([]byte("admin ok"))
            })))
        return &gs.HttpServeMux{Handler: mux}
    })
    gs.Run()
}

// hmacValidator is the resource side: verifies HS256 tokens with the shared
// secret and maps scope+roles claims to authorities (implements
// security.TokenValidator).
type hmacValidator struct{ secret []byte }

func (v hmacValidator) Validate(_ context.Context, token string) (*security.Authentication, error) {
    claims := jwt.MapClaims{}
    tok, err := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"})).
        ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return v.secret, nil })
    if err != nil || !tok.Valid {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    subject, _ := claims["sub"].(string)
    authorities := append(claimStrings(claims["scope"]), claimStrings(claims["roles"])...)
    return &security.Authentication{
        Principal: security.Principal{Subject: subject, Claims: claims},
        Authorities: authorities, Authenticated: true,
    }, nil
}
```

(`claimStrings` normalizes a space-delimited or array claim — copy it from
example/example.go.)

**conf/app.properties** — the complete, commented surface:

```properties
# --- activation (the only switch) ---------------------------------------------
spring.oauth2.server.enabled=true

# --- token content / lifetimes ------------------------------------------------
# iss claim; pin it on the resource server if you validate it.
spring.oauth2.server.issuer=https://issuer.example.com
spring.oauth2.server.access-token-ttl=1h
spring.oauth2.server.refresh-token-ttl=24h
spring.oauth2.server.code-ttl=1m

# --- signing key: exactly ONE source (here HMAC) ------------------------------
spring.oauth2.server.secret=example-shared-secret
# (asymmetric alternative: private-key / private-key-file; public half at /jwks)

# --- registered clients (config data, not beans) ------------------------------
# A public client (SPA): no secret, PKCE mandatory.
spring.oauth2.server.clients.spa.public=true
spring.oauth2.server.clients.spa.redirect-uris=http://127.0.0.1:9090/callback
spring.oauth2.server.clients.spa.scopes=read,write

# A confidential service client restricted to client_credentials.
spring.oauth2.server.clients.svc.secret=svc-secret
spring.oauth2.server.clients.svc.scopes=read
spring.oauth2.server.clients.svc.grant-types=client_credentials

# --- optional ecosystem -------------------------------------------------------
# spring.actuator.addr=:9370
# spring.observability.service-name=demo        (starter-otel)
```

**Verify** (isomorphic to `example/check.sh`; the example self-asserts the full flow):

```bash
bash example/check.sh
```

Manual round trip (example in `-manual` mode, base http://127.0.0.1:9090):

```bash
# 1. Authorize (PKCE): 302 with code + state
V=$(openssl rand -hex 32)                       # use GenerateVerifier() semantics
C=$(printf %s "$V" | openssl dgst -sha256 -binary | basenc --base64url | tr -d '=')
curl -sD- -o/dev/null "http://127.0.0.1:9090/oauth2/authorize?response_type=code&client_id=spa&redirect_uri=http://127.0.0.1:9090/callback&scope=read+write&state=xyz&code_challenge=$C&code_challenge_method=S256" | grep -i '^location'

# 2. Exchange the code for tokens (fill CODE from the Location above)
curl -s http://127.0.0.1:9090/oauth2/token -d grant_type=authorization_code \
  -d code=CODE -d redirect_uri=http://127.0.0.1:9090/callback \
  -d client_id=spa -d code_verifier="$V"

# 3. Call the protected API with the access token
curl -i -H "Authorization: Bearer $ACCESS_TOKEN" http://127.0.0.1:9090/api/me

# 4. client_credentials for the confidential client
curl -s http://127.0.0.1:9090/oauth2/token -d grant_type=client_credentials \
  -d client_id=svc -d client_secret=svc-secret -d scope=read

# 5. JWKS (empty key set for HMAC signing)
curl -s http://127.0.0.1:9090/oauth2/jwks
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-oauth2-server
  └─ gs.Provide(newAuthServer, TagArg("${spring.oauth2.server}"))
         .Condition(OnProperty("spring.oauth2.server.enabled").HavingValue("true"))
        │
gs.Run()
  ├─ condition gate: enabled != true → no bean, starter inert
  ├─ config bind: ${spring.oauth2.server} → Config (value tags)
  ├─ construction: newSigner FAILS FAST on ambiguous keys
  │     (zero or both of secret / private-key(-file)) → boot error
  │     JWKS document precomputed; in-memory store allocated
  ├─ your mux provider runs: injects *AuthServer, sets UserAuthFunc,
  │     mounts Handler() (or individual handlers) on your server
  ├─ Run/serve: /authorize, /token, /jwks live next to your business routes
  └─ shutdown: signer and store hold no goroutine/closable → no destroy hook
```

The server opens **no listener** — it is a Contributor-form bean; the application owns the
HTTP server and the routing. There is no `gs.Server`, no port key in this starter.

### 2.2 One full auth walk (browser through protected call)

Authorization-code + PKCE for the public client `spa`, layer by layer:

1. **GET /oauth2/authorize** with `response_type=code`, `client_id`, `redirect_uri`,
   `scope`, `state`, `code_challenge`, `code_challenge_method=S256`:
   - unknown `client_id` → plain 400 "unknown client_id" (no redirect: nothing validated
     to redirect to);
   - `redirect_uri` not in the client's exact allow-list → plain 400 (open-redirect guard);
   - grant not allowed for the client / `response_type != code` / bad PKCE method /
     out-of-list scope → 302 back to `redirect_uri` with `error=…` and the echoed `state`;
   - public client without `code_challenge` → 302 `error=invalid_request`;
   - `UserAuthFunc` nil → 503 "authorization_code flow not enabled" (client_credentials
     still works); user not authenticated → 401;
   - success: a single-use code (32 random bytes, base64url) is stored with the grant
     context (client, redirect_uri, scopes, subject, authorities, PKCE challenge) and
     `code-ttl` expiry; 302 to `redirect_uri?code=…&state=…`.
2. **POST /oauth2/token** (`grant_type=authorization_code`): client identity from Basic
   auth or the form body; confidential secrets compared constant-time. The code is
   consumed atomically (single use), then client_id and redirect_uri must match the grant,
   then the PKCE verifier is hashed and compared constant-time against the stored
   challenge. Any failure → the RFC 6749 §5.2 error JSON (`invalid_client` 401 /
   `invalid_grant` 400), `Cache-Control: no-store`.
3. **issueTokens**: access token is a JWT (`sub`, `client_id`, `iat`, `exp`, `jti`,
   optional `iss`, `scope` when non-empty, `roles` for authorities not already scopes;
   header carries `kid`); refresh token is an opaque 32-byte value stored server-side
   with `refresh-token-ttl`. Response is the standard `access_token`/`token_type`/
   `expires_in`/`refresh_token`/`scope` JSON.
4. **Protected call**: `Authorization: Bearer <jwt>` hits your route; the resource-side
   validator (your code — `starter-security-jwt` or, as here, an inline
   `security.TokenValidator`) verifies signature/expiry and maps claims to authorities;
   `security.Authenticate` + `security.Authorize` enforce.
5. **Refresh**: `grant_type=refresh_token` rotates (single use — the old token dies even
   if the response is lost) and may only narrow the original scopes.

client_credentials skips steps 1-2's code machinery: confidential clients only, the client
is its own principal (`sub = client_id`, scopes double as authorities), and **no refresh
token** is issued.

### 2.3 Stores — what lives where, and for how long

Authorization codes and refresh tokens live in **process memory** (a plain map under a
mutex): expiry is checked lazily on read and a cheap sweep runs on every write, so there
is no background goroutine and no destroy hook. Both codes and refresh tokens are
single-use and deleted atomically on redemption. There is **no** `OnMissingBean` seam for
the store: it is constructed inside the server bean, not a replaceable bean — multi-node
deployments need a shared store, which is out of scope (see §6).

---

## 3. Per-key behavior reference

`enabled` is a condition key (it gates the bean; it is not bound into Config). All other
keys bind under `spring.oauth2.server.*`.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `enabled` | bool | false | Activation switch; `HavingValue("true")`. | Unset/false → starter inert, `/oauth2/*` routes 404 on your mux. |
| `issuer` | string | — | `iss` claim on every token; empty omits the claim. | Mismatch with a resource server that pins the issuer → all tokens rejected downstream. |
| `algorithm` | string | — | Pin the signing algorithm; empty picks by key source: HS256 (HMAC), RS256 (RSA), ES256/384/512 by curve (EC). HMAC accepts HS256/384/512; RSA accepts RS*/PS*; EC must match the key's curve. | Incompatible value (e.g. `RS256` with a `secret`) → boot fails fast (`errBadAlgForKey`). |
| `secret` | string | — | HMAC signing key; resource server shares it out of band. ⚠ xor with `private-key`/`private-key-file`. | Empty with no PEM → boot fails (`errNoSigningKey`). Both set → boot fails (`errBothSigning`). |
| `private-key` | string | — | Inline RSA/ECDSA PEM for asymmetric signing; public half published at /jwks. ⚠ xor with `secret`; if `private-key-file` is also set, the file silently wins over the inline value. | Unparsable PEM → boot fails ("neither a valid RSA nor ECDSA PEM"). |
| `private-key-file` | string | — | Path alternative to `private-key`; read at boot. | Unreadable path → boot fails with the explained read error. |
| `key-id` | string | `default` | `kid` header on tokens and JWKS entry; lets a rotating resource server pick the right key. | Two servers sharing a JWKS with the same `kid` → wrong key selected. |
| `access-token-ttl` | duration | 1h | Lifetime of issued access tokens (`exp`); also returned as `expires_in`. | Too long → stale authorization survives; too short → refresh churn. |
| `refresh-token-ttl` | duration | 24h | Lifetime of server-side refresh records; refresh rotates on every use. | Shorter than access-token-ttl makes refresh pointless; very long = long-lived grant. |
| `code-ttl` | duration | 1m | Authorization-code lifetime; codes are single-use. ⚠ keep short — the code is redeemed immediately. | Long window widens code-interception exposure (mitigated by PKCE for public clients). |
| `clients.<id>.secret` | string | — | Confidential client's credential; constant-time compare at /token. ⚠ meaningless with `public=true` (ignored). | Empty for a confidential client → secret auth always fails (`clientAuthenticated` returns false). |
| `clients.<id>.public` | bool | false | Public client (SPA/native): no secret, PKCE **mandatory** at /authorize. | Public client without PKCE → every authorization 302 `invalid_request`. |
| `clients.<id>.redirect-uris` | []string | — | Exact-match allow-list for the auth-code redirect. | Missing/unlisted URI → plain 400, no redirect. |
| `clients.<id>.scopes` | []string | — | Allow-list of grantable scopes; empty = no restriction. | Requested scope outside the list → `invalid_scope`. |
| `clients.<id>.grant-types` | []string | — | Allow-list of grants; empty allows all three (`authorization_code`, `client_credentials`, `refresh_token`). | Grant outside the list → `unauthorized_client`. |

⚠ Coupled: `secret` vs `private-key`/`private-key-file` (exactly one, boot-checked);
`algorithm` vs key source (compatibility checked); `public=true` vs PKCE (protocol-level
coupling); `issuer` vs the resource server's validation.

---

## 4. Verification & fault drills

All drills run against `cd example && go run . -manual` (base :9090), or are asserted
automatically by `bash example/check.sh`.

### 4.1 Happy path & token claims

```bash
# client_credentials then inspect the JWT (HS256 demo secret)
TOK=$(curl -s :9090/oauth2/token -d grant_type=client_credentials \
  -d client_id=svc -d client_secret=svc-secret -d scope=read | jq -r .access_token)
echo "$TOK" | cut -d. -f2 | basenc --base64url -d 2>/dev/null; echo
# → claims: sub=svc, client_id=svc, scope=read, jti, iat, exp, iss; no refresh_token
```

### 4.2 Token expiry

Set `spring.oauth2.server.access-token-ttl=2s`, mint a token, wait 3s:

```bash
curl -i -H "Authorization: Bearer $TOK" :9090/api/me   # 401 from your validator
```

### 4.3 Wrong client secret

```bash
curl -i :9090/oauth2/token -d grant_type=client_credentials \
  -d client_id=svc -d client_secret=WRONG
# 401 {"error":"invalid_client","error_description":"client authentication failed"}
```

### 4.4 PKCE failure (verifier mismatch)

Authorize with a challenge derived from verifier A, redeem with verifier B:

```bash
curl -s :9090/oauth2/token -d grant_type=authorization_code -d code=$CODE \
  -d redirect_uri=http://127.0.0.1:9090/callback -d client_id=spa \
  -d code_verifier=the-wrong-verifier
# 400 {"error":"invalid_grant","error_description":"PKCE verification failed"}
```

### 4.5 Single-use: code & refresh replay

Redeem a code twice — the second gets `invalid_grant` "code invalid or expired". Same for
a used refresh token (rotation). Expired codes (past `code-ttl`) also fail.

### 4.6 Public client blocked from client_credentials

```bash
curl -i :9090/oauth2/token -d grant_type=client_credentials -d client_id=spa
# 401 invalid_client — public clients cannot use this grant
```

### 4.7 /jwks and signing modes

- HMAC (`secret`): `curl :9090/oauth2/jwks` → `{"keys":[]}` — nothing to publish; the
  resource server shares the secret out of band.
- RSA/EC (`private-key-file`): the public key appears with `kid`, `use: sig`, `alg`.
- ⚠ Demo footgun (from DESIGN): with RSA and a JWKS URL pointing back at *this* process,
  an eagerly-bootstrapping validator can deadlock against the not-yet-serving server —
  ship HMAC for single-process demos.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "no signing key configured" | neither `secret` nor `private-key`/`private-key-file` | Set exactly one. |
| Boot fails "both HMAC secret and PEM private key" | both key sources set | Remove one. |
| Boot fails "algorithm is not compatible" | `algorithm` disagrees with the key source | Match them or leave empty (auto). |
| `/oauth2/*` 404 | `enabled` not true (starter inert) | Set `spring.oauth2.server.enabled=true`. |
| /authorize returns 503 | `UserAuthFunc` nil | Set it in your mux provider (auth-code flow only). |
| /authorize plain 400 instead of redirect | unknown `client_id` or unlisted `redirect_uri` | Register the client / list the URI exactly. |
| Public client always `invalid_request` at /authorize | missing `code_challenge` | PKCE is mandatory for `public=true`. |
| Token exchange `invalid_grant` "code does not match client/redirect_uri" | redeeming with a different client or redirect than the grant | Redeem with the same pair. |
| Refresh `invalid_grant` | token already used (rotation) or expired | Use the freshest refresh token; treat loss as re-login. |
| Works single-node, codes rejected behind LB | in-memory store is per-process | Out of scope; needs a shared store (see §6). |
| Resource server rejects every token | secret/issuer/algorithm mismatch between the two halves | Align key material and `issuer`. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 10 server + 5 per client (+1 condition key `enabled`) |
| Required | 2 (`enabled=true` + exactly one signing key) |
| Quickstart external deps | 0 |
| "Watch out" entries | 5 |

Design suspects (kept from previous USAGE/DESIGN, plus new ones):

- Codes and refresh tokens live in process memory — single-node only; multi-node needs a
  shared store (existing). The store is internal with no `OnMissingBean`/bean seam, so the
  DESIGN's "durable store starter could contribute a `CodeStore` bean later" has no actual
  extension point yet (new).
- `UserAuthFunc` is a mutable public field set after injection, not a config-driven or
  bean-driven seam (existing).
- /authorize authenticates the resource owner *inside* the request (no login redirect of
  its own): `UserAuthFunc` returning ok=false yields a bare 401, not an OAuth2 error
  redirect — SPAs must map that themselves (new, minor).
- No rate limiting / brute-force guard on /token (secret compare is constant-time, but
  attempts are unbounded) (new).
- `private-key` and `private-key-file` are silently resolved file-over-inline — no warning
  when both are set (new).
- Only `authorization_code`, `client_credentials`, `refresh_token`; no password/device/
  extension grants, no token introspection/revocation endpoints (scope boundary, restated
  for the audit ledger).
