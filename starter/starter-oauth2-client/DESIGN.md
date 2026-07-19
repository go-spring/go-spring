# starter-oauth2-client Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-oauth2-client` is a **Contributor**-archetype starter (see
[starter/DESIGN.md](../DESIGN.md) §2.3) that publishes ready-to-use
`*http.Client` and `oauth2.TokenSource` beans that transparently obtain
and refresh OAuth2 tokens on the caller's behalf. It opens no port; the
resulting client is injected wherever the application talks to a
protected upstream.

## 1. Responsibilities & Boundaries

- **In scope (v1):** the *client* side of OAuth2. Two grants ship:
  `client_credentials` (`*http.Client` + `oauth2.TokenSource` dual
  beans, `EndpointParams` for Auth0 audience / Azure resource) and
  `authorization_code` (`*oauth2.Config`).
- **Out of scope:** resource-server / token *validation* — that lives
  in `starter-security-jwt`. The split point for a shared validation
  core (stdlib vs. web-framework-bound) is deferred until a real
  requirement lands.

## 2. Key Decisions

- **Multi-instance only, via `gs.Group`.** `${spring.oauth2.client}`
  produces one named `*http.Client` per entry; adding an upstream is a
  pure-config change. `destroy` is `nil` because `*http.Client` has no
  `Close()`.
- **Same prefix, two bean types.** The same `${spring.oauth2.client}`
  group also registers a matching `oauth2.TokenSource` bean under each
  name (bean-uniqueness is *type + name*). This gives gRPC metadata,
  WebSocket handshake, and other "raw token" call sites the underlying
  source without another config prefix.
- **`authorization_code` under a distinct prefix.**
  `${spring.oauth2.authcode}` publishes `*oauth2.Config`. Different
  grants are different capabilities — separate prefixes keep them
  independently configurable and prevent field bleed between
  client-credentials fields and auth-code fields.
- **`EndpointParams` as `map[string]string`.** `conf` supports map
  binding with `${...:=}` as the empty default, converted to `url.Values`
  before handing to `clientcredentials.Config.EndpointParams`. This is
  what makes Auth0 `audience` / Azure `resource` work without a
  code change.
- **Resilience is layered here, not below.** The starter integrates
  `stdlib/resilience` as the outermost transport wrap (retry can
  re-pick, breaker keys by logical name). Enabling it is a config-only
  switch on the same instance.

## 3. Constraints

- **Fail-fast on missing token URL / client id.** Empty required fields
  raise a clear boot error rather than half-working defaults.
- **`golang.org/x/oauth2` is the only external dep** — internal
  spring / log / stdlib deps resolve through `go.work`. Pulling
  `x/oauth2` inside the corporate network required
  `GOPROXY=https://goproxy.cn,direct` at one point; the module is a
  standard-library-adjacent package but not vendorable through the
  internal proxy.

## 4. Trade-offs / Alternatives Rejected

- **A default singleton `*http.Client` — rejected.** Same family rule
  §2.2 for client starters: dual registration was error-prone and
  conditional-singleton semantics were opaque.
- **Merging `client_credentials` and `authorization_code` into one
  bean type — rejected.** They produce different types (`*http.Client`
  vs `*oauth2.Config`) and have different fields; separate prefixes
  keep both surfaces sharp.
- **Resource-server validation in this starter — rejected.** It
  belongs on the *server* side (`starter-security-jwt` + `stdlib/security`),
  not muxed into the client's config surface.
