# starter-oauth2-client

[English](README.md) | [中文](README_CN.md)

`starter-oauth2-client` provides an OAuth2 *client-credentials* HTTP client
based on [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2), making
it easy for a Go-Spring application to call a protected downstream service. The
resulting `*http.Client` fetches and refreshes the bearer token automatically,
so callers use it like an ordinary HTTP client.

## Installation

```bash
go get go-spring.org/starter-oauth2-client
```

## Quick Start

### 1. Import the `starter-oauth2-client` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-oauth2-client"
```

### 2. Configure the OAuth2 Client

Add configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.oauth2.client.downstream.client-id=demo-client
spring.oauth2.client.downstream.client-secret=demo-secret
spring.oauth2.client.downstream.token-url=https://auth.example.com/oauth/token
spring.oauth2.client.downstream.scopes=read,write
spring.oauth2.client.downstream.auth-style=header
spring.oauth2.client.downstream.timeout=5s
```

### 3. Inject the HTTP Client

The starter registers one `*http.Client` per configuration key, so inject it by
that name. Refer to the [example.go](example/example.go) file.

```go
type Service struct {
    Client *http.Client `autowire:"downstream"`
}
```

### 4. Use the HTTP Client

Refer to the [example.go](example/example.go) file. The token is obtained on the
first request and attached as `Authorization: Bearer <token>` automatically.

```go
resp, err := s.Client.Get("https://api.example.com/resource")
```

## Core Features

The [example.go](example/example.go) program starts an in-process OAuth2 token
endpoint and a protected resource server, then demonstrates and asserts:

* **Automatic token fetch** — the injected client mints a token via the
  client-credentials grant on the first request.
* **Transparent bearer injection** — the token is attached to the downstream
  request, which returns `200 OK` from the protected endpoint.

## Advanced Features

* **Supports multiple OAuth2 clients**: define several clients in the
  configuration file (each with its own credentials) and reference them by name.
* **Configurable auth style**: `auth-style` selects how credentials are sent to
  the token endpoint — `auto` (default), `header` (HTTP Basic), or `params`
  (request body).
* **`*TokenSource` bean**: alongside the `*http.Client`, the starter also
  registers a `*StarterOAuth2Client.TokenSource` for every
  `spring.oauth2.client.<name>` entry under the SAME name. It satisfies
  `oauth2.TokenSource`, so it drops into anything expecting one. Inject it when you
  need the raw bearer token (for example, to attach it to gRPC metadata):

  ```go
  import StarterOAuth2Client "go-spring.org/starter-oauth2-client"

  type Service struct {
      TokenSrc *StarterOAuth2Client.TokenSource `autowire:"downstream"`
  }

  // tok, err := s.TokenSrc.Token()
  // use tok.AccessToken
  ```

  Beyond `Token()`, it exposes the cached token's status for observability without
  forcing a fetch:

  | Method | Description |
  | --- | --- |
  | `Token()` | Returns a valid token, fetching/refreshing as needed, and caches it. |
  | `Peek()` | Returns the most recently observed token, or `nil` if none fetched yet. |
  | `Valid()` | Reports whether a token has been fetched and is not expired. |
  | `Expiry()` | Returns the expiry of the last observed token, or the zero time. |

* **Extra token-endpoint parameters (`endpoint-params`)**: some providers
  require additional parameters at the token endpoint, such as Auth0's
  `audience` or Azure AD's `resource`. Declare them as a map — the map key
  becomes the parameter name:

  ```properties
  spring.oauth2.client.downstream.endpoint-params.audience=https://api.example.com
  ```

## Observability

Both the token-endpoint exchange and the downstream business requests are
instrumented for distributed tracing. The starter wraps the underlying
`*http.Client` transport with
[`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp),
so every outbound request emits a client span through the OpenTelemetry globals
that [`starter-otel`](../starter-otel) installs.

This is a zero-config opt-in that mirrors go-redis's `redisotel` hooks: import
`starter-otel` and spans flow automatically; without it the OTel globals are
no-ops, so no spans are produced and no request bytes change. Instrumentation
happens at a single point — the transport shared by the token fetch and the
returned client — so each token fetch and each downstream call yields exactly
one span (no double counting).

### Authorization Code Grant

For interactive user-login / redirect flows, use the separate configuration
prefix `spring.oauth2.authcode.<name>`. The starter registers one
`*oauth2.Config` per entry, injectable by name.

```properties
spring.oauth2.authcode.login.client-id=web-client
spring.oauth2.authcode.login.client-secret=web-secret
spring.oauth2.authcode.login.auth-url=https://auth.example.com/oauth/authorize
spring.oauth2.authcode.login.token-url=https://auth.example.com/oauth/token
spring.oauth2.authcode.login.redirect-url=https://app.example.com/callback
spring.oauth2.authcode.login.scopes=openid,profile
```

Inject and use:

```go
import "golang.org/x/oauth2"

type LoginService struct {
    OAuth *oauth2.Config `autowire:"login"`
}

// url := s.OAuth.AuthCodeURL("state")       // redirect the user here
// tok, err := s.OAuth.Exchange(ctx, code)   // swap the callback code for a token
```
