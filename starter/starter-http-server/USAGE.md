# starter-http-server Usage

[English](USAGE.md) | [中文](USAGE_CN.md)

Anchored on the self-verifying [example](example/example.go) (`./check.sh`
exits 0).

## 1. Quick start

No external dependencies. It decorates the framework's built-in server:

```
go get go-spring.org/starter-http-server
```

```properties
# conf/app.properties — the server itself is gs's, not this starter's
spring.http.server.addr=:9090
```

```go
gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle("/api/me", httpsvr.Chain(
        httpsvr.Authenticate(v, true),
        httpsvr.Authorize("orders:read"),
    )(http.HandlerFunc(me)))
    return &gs.HttpServeMux{Handler: mux}
})
```

The token comes from an injected `security.TokenValidator` (contributed by
`starter-oauth2-resource-server` or `starter-security-jwt` per configuration),
or from your own implementation.

## 2. Configuration

This starter has no configuration keys of its own. The server-side keys
(`spring.http.server.addr` / `readTimeout` / `writeTimeout` / `idleTimeout`)
belong to `gs`; see the spring documentation.

## 3. API

- `Chain(ms ...Middleware) Middleware` — composes middlewares, outermost first;
  the canonical order is `Chain(CORS, CSRF, Authenticate, Authorize)`.
- `Authenticate(v security.TokenValidator, required bool)` — with no token,
  `required=true` yields 401 and `false` passes through; an invalid token always
  yields 401.
- `Authorize(authorities ...string)` — anonymous yields 401; an authenticated
  caller missing any required authority yields 403; with no arguments it only
  requires an authenticated caller.
- `Observe() Middleware` — the server-side observability middleware (span +
  metrics + access log); opt-in, see §4.
- `CORS(CORSConfig)` / `CSRF(CSRFConfig)` — see the constraints in
  [cloud/security DESIGN](../../cloud/security/README.md) (wildcard ×
  credentials, double-submit cookie).

## 4. Operations

The security middlewares emit no logs or metrics; observability comes from the
server side and from `starter-governance`. The CSRF cookie/header names default
to `csrf_token` / `X-CSRF-Token`, matching the gin and echo shells.

`Observe()` is the one exception: it is an **opt-in** server-side observability
middleware. Unlike gin/echo/hertz, whose instrumentation is built in, this
package decorates the handler the application itself handed to the framework's
server — it has nowhere to install itself — so it is composed into the chain
like any other decorator:

```go
mux.Handle("/api/me", httpsvr.Chain(
    httpsvr.Observe(),
    httpsvr.CORS(cfg),
    httpsvr.Authenticate(v, true),
)(handler))
```

One request produces:

- a server span named `<METHOD> <path>`, with attributes `http.request.method`
  and `url.path`, plus `http.response.status_code` and `status` on the way out;
- two metrics: `http.server.request.duration` (Float64Histogram, seconds) and
  `http.server.active_requests` (Int64UpDownCounter), labelled
  `http.request.method` and `http.response.status_code`;
- one access-log line, tag `_app_http_server_access`
  (`log.RegisterAppTag("http_server", "access")`), with fields
  `http.request.method`, `url.path`, `http.response.status_code`, `status` and
  `duration_ms`; a 5xx is a Warn line.

The names deliberately match the gin/echo/hertz HTTP family, so an application
switching between the stdlib server and gin keeps the same dashboards. The span
and the metrics ride the OTel globals starter-otel installs; without it they are
no-ops and only the access log is written.

## 5. Design checklist

| Measure | Value |
|---------|-------|
| Own configuration keys | 0 |
| External dependencies | 0 |
| Beans | 0 (a pure library) |
