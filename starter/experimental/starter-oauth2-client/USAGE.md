# starter-oauth2-client Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `tokensource.go`, `authcode.go`,
`trace.go`) and the runnable [example/](example/). **OAuth2 protocol semantics are
[x/oauth2's own documentation](https://pkg.go.dev/golang.org/x/oauth2) and the
[RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749) grant definitions** — everything
below is go-spring's increment: binding, beans, tracing, resilience wiring.

**Activation**: two independent prefix groups. Any `spring.oauth2.client.<name>.*` key
activates the client-credentials group (multi-instance: one `<name>` = one `*http.Client` +
one `*TokenSource`); any `spring.oauth2.authcode.<name>.*` key registers one `*oauth2.Config`
per entry. No `enabled` key for either.

---

## 1. Complete worked project

A service that calls a protected downstream API with a client-credentials bearer token,
with retry/breaker resilience and OTel tracing on every hop. The example repo ships exactly
this; the tree below is the canonical layout.

```
demo/
├── go.mod
├── example.go            (or main.go + service.go)
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    golang.org/x/oauth2          v0.36.0
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-oauth2-client latest
    go-spring.org/starter-governance    latest  // optional: real resilience policies
    go-spring.org/starter-otel          latest  // optional: real trace export
)
```

**example.go** — the application's entire surface (mirrors example/example.go):

```go
package main

import (
    "io"
    "net/http"

    "go-spring.org/spring/gs"
    "golang.org/x/oauth2"

    _ "go-spring.org/starter-governance"
    StarterOAuth2Client "go-spring.org/starter-oauth2-client"
)

// Service consumes the OAuth2-backed HTTP client. Both beans are registered by
// the starter under the group name "downstream" (see conf/app.properties), so
// they are injected by that name.
type Service struct {
    // Ready-to-use client: fetches and refreshes the bearer token, attaches it
    // to every request, retries/breaks via the governance executor.
    Client *http.Client `autowire:"downstream"`
    // Raw token source for non-HTTP call sites (gRPC metadata, WebSocket).
    // NOTE: not resilience-wrapped — see §2.3.
    TokenSrc *StarterOAuth2Client.TokenSource `autowire:"downstream"`
    // Authorization-code config for the interactive login flow, if any.
    OAuth *oauth2.Config `autowire:"login"`
}

func main() {
    svr := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()) // root-reachable
    _ = svr
    // ... use s.Client.Get("https://api.example.com/resource") from a handler
    gs.Run()
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- client-credentials instance "downstream" (activates the group) -----------
spring.oauth2.client.downstream.client-id=demo-client
spring.oauth2.client.downstream.client-secret=demo-secret
spring.oauth2.client.downstream.token-url=https://auth.example.com/oauth/token
spring.oauth2.client.downstream.scopes=read,write
spring.oauth2.client.downstream.auth-style=header
spring.oauth2.client.downstream.timeout=5s
spring.oauth2.client.downstream.endpoint-params.audience=https://api.example.com

# --- authorization_code instance "login" (activates the authcode group) -------
spring.oauth2.authcode.login.client-id=web-client
spring.oauth2.authcode.login.client-secret=web-secret
spring.oauth2.authcode.login.auth-url=https://auth.example.com/oauth/authorize
spring.oauth2.authcode.login.token-url=https://auth.example.com/oauth/token
spring.oauth2.authcode.login.redirect-url=https://app.example.com/callback
spring.oauth2.authcode.login.scopes=openid,profile

# --- governance (resilience for the *http.Client transport) ------------------
# Same resource label as the client: oauth2:<client-id>.
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.max-retries=3
govern.default.error-threshold=10
govern.default.attempt-timeout=2s

# --- observability (starter-otel, optional) ----------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
```

**Verify** (isomorphic to `example/check.sh`, which runs the same assertions in-process —
the example spins up its own fake token endpoint on :9401 and a protected resource on :9402,
so no external IdP is needed):

```bash
bash example/check.sh
# Expected output includes:
#   Response from protected resource: hello from protected resource
#   Token from TokenSource: demo-access-token
#   Resilience: recovered after 3 attempts: recovered after retries
```

Manual mode keeps the process up for your own curl experiments:

```bash
cd example && go run . -manual
curl -s http://127.0.0.1:9401/oauth/token -d 'grant_type=client_credentials' \
  -d client_id=demo-client -d client_secret=demo-secret   # the fake token endpoint
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-oauth2-client
  ├─ gs.Module(OnProperty("spring.oauth2.client"))           
  │     └─ per <name>: Provide(newClient).Name(<name>).Destroy(destroyClient)
  │                      Provide(newTokenSource) via gs.Group
  └─ gs.Group("${spring.oauth2.authcode}", newAuthCodeConfig)
        │
gs.Run()
  ├─ config bind (per instance): value tags + expr validation
  │     client group: client-id/client-secret/token-url must be non-empty
  │     authcode group: NO expr validation — empty values bind silently ⚠
  ├─ bean wiring: *http.Client, *TokenSource, *oauth2.Config registered under
  │     (type, name); your struct's `autowire:"<name>"` picks them up
  ├─ construction: newClient wraps the transport (otel → resilience); nothing
  │     dials yet — the first token fetch is lazy, on the first request
  ├─ Run/serve: your code runs; tokens fetch/refresh on demand
  └─ shutdown: Destroy(destroyClient) closes the resilience roundtripper
        (plain clients fail the io.Closer assertion and release nothing)
```

`OnProperty("spring.oauth2.client")` is a prefix check: any `spring.oauth2.client.<name>.<k>`
key fires the module, and each sub-map entry under the prefix becomes one instance
(`conf.BindEach`). That is why there is no `enabled` key.

### 2.2 What is wired where — the two beans, precisely

For one `spring.oauth2.client.<name>` entry the starter builds **two beans sharing the
name** (bean identity is type + name):

| Bean | Token machinery | Tracing | Resilience | Destroy |
|------|-----------------|---------|------------|---------|
| `*http.Client` | `clientcredentials.Config.Client(...)` — lazy fetch, auto-refresh, bearer header on every request | yes: transport base is `otelhttp`, one span per token fetch and per downstream request | yes: transport wrapped in `resilience.NewRoundTripper` keyed `oauth2:<client-id>` | yes (closes transport) |
| `*TokenSource` | `clientcredentials.Config.TokenSource(...)` — same lazy fetch/refresh, returns raw tokens | partially: the context passed to the source carries the otel client (token-endpoint requests are traced) | **no** — no executor wrap, no retry/breaker/limiter | no (nothing closable) |

Design rationale (from source comments): the client wraps the transport so the bearer token
is attached *before* the resilience layer runs and "each protected attempt is a complete
request" in the source comments — retry re-executes a fully authorized request. The TokenSource
serves call sites that inject the token themselves (gRPC metadata, WebSocket handshake) and
doubles as an observability handle (`Peek`/`Valid`/`Expiry` read the cached token without a
round-trip). The asymmetry is a known design suspect (§6).

`*oauth2.Config` (authcode group) is plain configuration — no transport, no tracing, no
resilience. You call `AuthCodeURL(state)` and `Exchange(ctx, code)` yourself; those use
`http.DefaultClient` unless you pass a context carrying `oauth2.HTTPClient`.

### 2.3 One request, layer by layer

`s.Client.Get("https://api.example.com/resource")` with governance on:

1. `client.Timeout` bounds the whole request (`timeout` key, if > 0).
2. `oauth2.Transport` (from `clientcredentials`) checks its cached token; on miss it POSTs
   `grant_type=client_credentials` to `token-url` — credentials placed per `auth-style`
   (auto-probe / Basic header / body params), `endpoint-params` merged into the body.
3. That token POST itself goes through the otel-wrapped base client — exactly one span,
   and it is retried/broken **only** insofar as the base client is governed (it is not:
   the executor wraps the outer transport, not the token exchange — see §4.4 drill).
4. The access token is cached; the `Authorization: Bearer <token>` header is set on the
   outbound request *before* the base transport runs.
5. `resilience.NewRoundTripper` executes the request through the executor resolved for
   resource label `oauth2:<client-id>` — retry / circuit breaker / rate limit policy from
   the governance center; transparent no-op when governance is off. The executor is
   additionally wrapped by `resilience.WrapExecutor` so trips/rejects/retries emit
   span + counter + histogram + access log.
6. `otelhttp` emits the client span (method/url); without starter-otel these are no-ops.
7. Response unwinds; on 401 the oauth2 layer does not retry (client_credentials has no
   refresh token) — the next call re-fetches.

---

## 3. Per-key behavior reference

### 3.1 `spring.oauth2.client.<name>.*` — 7 keys

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `client-id` | string | — | Client identifier sent to the token endpoint (per `auth-style`). | Empty → **boot fails** (expr `$ != ''`). |
| `client-secret` | string | — | Client credential; in Basic header or body per `auth-style`. | Empty → boot fails (expr). |
| `token-url` | string | — | Token endpoint for the client-credentials grant. | Empty → boot fails (expr). Wrong URL → first request fails with the token-endpoint error. |
| `scopes` | []string | — | Comma list requested with the token. | Scope rejected by the IdP → token fetch error at first request, not at boot. |
| `endpoint-params.<k>` | map[string]string | — | Extra body params on the token request (Auth0 `audience`, Azure `resource`). One sub-key per param. | Wrong param → token-endpoint error at request time. |
| `auth-style` | string | `auto` | `auto` \| `header` \| `params` — how credentials are sent . `auto` lets x/oauth2 probe once and cache. | IdP requiring Basic with `params` set → 401 on every token fetch. |
| `timeout` | duration | 0 | Applied twice: to the otel base client (bounds the token fetch) *and* to the returned `*http.Client` (bounds each downstream request). 0 = no timeout. ⚠ large value + governance `attempt-timeout`: the per-attempt bound bites first. | 0 → a hung token endpoint or downstream hangs forever. |

### 3.2 `spring.oauth2.authcode.<name>.*` — 6 keys

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `client-id` | string | — | Client identifier embedded in `AuthCodeURL` output. | No expr validation — empty binds silently; the redirect to /authorize fails with the IdP's `invalid_client`-family error. |
| `client-secret` | string | — | Used by `Exchange` to authenticate the token request. | Same silent-bind hazard; token exchange 401. |
| `auth-url` | string | — | Authorization endpoint in the built `*oauth2.Config`. | Empty → `AuthCodeURL` returns a malformed URL; runtime failure only. |
| `token-url` | string | — | Token endpoint used by `Exchange`. | Empty → `Exchange` fails at runtime. |
| `redirect-url` | string | — | Callback URL; must match what the IdP has registered for the client. | Mismatch → IdP rejects at /authorize (exact-match). |
| `scopes` | []string | — | Comma list requested during authorization. | Rejected scope → authorization error redirect. |

Both groups bind only these keys; there is no `enabled`, no `name` key, no TLS block —
bring your own transport by injecting `*TokenSource` instead of the `*http.Client`.

---

## 4. Verification & fault drills

### 4.1 Happy path (smoke)

```bash
bash example/check.sh    # asserts: 200 from protected resource, token value,
                         # Peek/Valid on the cache, AuthCodeURL shape, retry count
```

### 4.2 Token fetch & refresh observation

After a successful call, the TokenSource caches the token:

```go
tok, _ := s.TokenSrc.Token()   // fetch-or-refresh, caches on success
s.TokenSrc.Peek()              // no round-trip; nil before first Token()
s.TokenSrc.Valid()             // false once expired
s.TokenSrc.Expiry()            // zero time before first fetch
```

Drill: set the example's fake endpoint's `expires_in` to `1` and issue two calls spaced
2s apart — the second triggers a refresh (one extra POST to `token-url` in the otel spans
or access log).

### 4.3 Wrong credentials / wrong token URL

Point `token-url` at a real IdP with a wrong `client-secret`: every downstream request
fails immediately with x/oauth2's token-fetch error. The failure is per-request (lazy
fetch), never at boot — the starter deliberately binds fast and fetches lazily.

### 4.4 Resilience-wrapped vs raw client (the asymmetry)

The example's `/api/flaky` (503 twice, then 200) recovers transparently through the
`*http.Client` (check.sh asserts exactly 3 attempts). Drill the negative side:

```go
// Raw token source has NO retry: mint the token, then call the flaky
// endpoint with a plain client using tok.AccessToken manually — you get 503.
tok, _ := s.TokenSrc.Token()
req, _ := http.NewRequest("GET", "http://127.0.0.1:9402/api/flaky", nil)
req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
// http.DefaultClient.Do(req) → 503 on the first two calls, no retry.
```

Also observable: with starter-otel + governance, breaker trips on the wrapped client emit
`oauth2`-labeled spans/metrics (`WrapExecutor(exec, "oauth2", ...)`); nothing equivalent
exists for `TokenSource`.

### 4.5 Governance hot-toggle

With `govern.source.file.path`, flip `max-retries=0` in the file while the example runs —
the same flaky call now fails fast to the caller (executor policy is resolved at call
time. No restart.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "expr … != ''" | empty `client-id`/`client-secret`/`token-url` on a client entry | Fill them or remove the entry. |
| Starter totally inert | no `spring.oauth2.client.*` key at all (prefix activation) | Add at least one instance block. |
| Injected client nil / "no bean" | autowire name mismatch (bean name = instance `<name>`) | Match `autowire:"<name>"` to the config key. |
| 401 from downstream, token fine | downstream expects different scheme or audience | Check `endpoint-params` (audience) and `scopes`. |
| Token fetch 401 | `auth-style` disagrees with the IdP | Try `header` (Basic) — most common requirement. |
| Token never refreshed though the IdP uses a short TTL | IdP returns no `expires_in` → x/oauth2 treats the token as non-expiring and caches it forever | IdP-side fix (always send `expires_in`). |
| No retry though governance on | using `*TokenSource` + own client (raw path, no executor) | Use the injected `*http.Client`, or wrap your transport yourself. |
| No spans | starter-otel absent | Import it; otelhttp is a silent no-op otherwise. |
| First request hangs indefinitely | `timeout=0` and token endpoint unresponsive | Set `timeout`. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 7 (client) + 6 (authcode) |
| Required | 3 (client group, expr-validated) |
| Quickstart external deps | 0 (example runs an in-process token endpoint) |
| "Watch out" entries | 4 |

Design suspects (kept from previous USAGE/DESIGN, plus new ones):

- `*TokenSource` skips the resilience wrapper while `*http.Client` has it — the same config
  entry yields two beans with different governance behavior (existing).
- The token-endpoint exchange itself is not governed: the executor wraps the outer
  transport only, so a flaky IdP fails the call with no retry (new — arguably correct,
  retrying auth is risky, but undocumented intent).
- Authcode group has no expr validation while the client group does — silent empty binds
  (new).
- `timeout` is applied to two different layers (token fetch + downstream request) with one
  key; the token fetch also consumes downstream budget (new).
- `*oauth2.Config` beans carry no tracing/resilience — `Exchange` uses DefaultClient unless
  the caller passes a context with `oauth2.HTTPClient` (new).
