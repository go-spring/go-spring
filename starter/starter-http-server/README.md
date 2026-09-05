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
