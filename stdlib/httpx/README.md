# httpx
[English](README.md) | [中文](README_CN.md)

`httpx` is the runtime assembler behind Go-Spring's declarative HTTP client — the
OpenFeign / `@HttpExchange` equivalent. Because Go has no runtime proxy, the call
sites are produced by `gs-http-gen`; a generated client only holds an
`*http.Client`, and `httpx.NewTransport` builds that client's `http.RoundTripper`.

## Features

- Single seam: `http.RoundTripper`, the same seam already used by `resilience`
  and `otelhttp`.
- Discovery + load balancing: when a `ServiceName` is set, wires a
  `discovery.LiveDialer` and a `loadbalance.Pool` (round-robin, least-conn,
  consistent-hash, weighted, zone-aware) with optional outlier ejection.
- Direct addressing: with only `Addr`, rewrites every request to that host — the
  generated client's `Target` need not be set.
- Optional `resilience` executor wrapping the whole chain, so a retry re-enters
  the balancer and picks a fresh endpoint, and the breaker keys on the logical
  service name.
- Fails fast at wiring time when a discovery backend or load-balancing strategy
  is misconfigured.

## Usage

```go
import "go-spring.org/stdlib/httpx"

rt, closeFn, err := httpx.NewTransport(httpx.Config{
    ServiceName: "user-svc",     // omit for direct-address mode
    Discovery:   "redis",
    Balancer:    "round_robin",
    Base:        otelhttp.NewTransport(http.DefaultTransport),
})
if err != nil {
    log.Fatal(err)
}
defer closeFn()

client := &http.Client{Transport: rt}
```

For direct addressing, set `Addr` and leave `ServiceName` empty; `httpx` will
rewrite every request's host to `Addr`. Trace propagation is layered on top by
passing an instrumented `Base` (a starter concern; `httpx` itself imports no
observability library).

See `starter/starter-http-client` for the bean-oriented wrapper.
