# starter-http-server
[English](README.md) | [中文](README_CN.md)

Security middleware kit for the framework's built-in stdlib HTTP server
(`gs.SimpleHttpServer`, configured by `spring.http.server.*`). It contributes
no server and no beans of its own — it supplies the `net/http` decorator
family for authentication, authorization, CORS and CSRF, on top of the shared
identity model in [`cloud/security`](../../cloud/security).

| Middleware | What it does |
| --- | --- |
| `Chain(ms...)` | Composes middlewares, outermost-first |
| `Authenticate(v, required)` | Verifies the bearer token via `security.TokenValidator`, attaches the `Authentication` to the request context |
| `Authorize(authorities...)` | Gates a route on authorities (401 anonymous / 403 lacking) |
| `CORS(cfg)` | Adds `Access-Control-Allow-*` headers, answers preflights |
| `CSRF(cfg)` | Double-submit-cookie CSRF defence for browser flows |
| `Observe()` | Span, request metrics and access log per request — opt-in, see Observability |

## Quick Start

```go
import (
    "net/http"

    "go-spring.org/cloud/security"
    "go-spring.org/spring/gs"
    httpsvr "go-spring.org/starter-http-server"
)

gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle("/api/me", httpsvr.Chain(
        httpsvr.Authenticate(v, true),
        httpsvr.Authorize("orders:read"),
    )(http.HandlerFunc(meHandler)))
    return &gs.HttpServeMux{Handler: mux}
})
```

For method-level checks inside a service, use `security.Require` from the
core package. For the gin and echo equivalents of these middlewares, see
`starter-gin` and `starter-echo`. A runnable, self-verifying example lives in
[example](example/example.go).

## Observability

`Observe()` is the server-side observability middleware. It is **opt-in**, unlike the
gin/echo/hertz starters where instrumentation is built in: this package decorates whatever
handler the application passes to the framework's own server, so it has no place to install
itself. Compose it like any other decorator:

```go
mux.Handle("/api/me", httpsvr.Chain(
    httpsvr.Observe(),
    httpsvr.CORS(cfg),
    httpsvr.Authenticate(v, true),
)(handler))
```

One request produces:

* a server span named `<METHOD> <path>`, carrying `http.request.method`, `url.path` and, on
  exit, `http.response.status_code` plus `status`;
* `http.server.request.duration` (Float64Histogram, seconds) and
  `http.server.active_requests` (Int64UpDownCounter), labelled `http.request.method` and
  `http.response.status_code`;
* one access-log line, tag `_app_http_server_access`
  (`log.RegisterAppTag("http_server", "access")`), carrying `http.request.method`,
  `url.path`, `http.response.status_code`, `status` and `duration_ms`. A 5xx is a Warn
  line; the span and the line agree on `status`.

The names are the HTTP family's — the same ones gin, echo and hertz use — so an application
that switches between the stdlib server and gin keeps the same dashboards. The span and the
metrics ride the OTel globals `starter-otel` installs; without it they are no-ops and only
the access log is emitted.
