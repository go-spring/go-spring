# httpx
[English](README.md) | [中文](README_CN.md)

`httpx` is the runtime assembler behind Go-Spring's declarative HTTP client — the
OpenFeign / `@HttpExchange` equivalent. Because Go has no runtime proxy, the call
sites are produced by `gs-http-gen`; a generated client only holds an
`*http.Client`, and `httpx.NewTransport` builds that client's `http.RoundTripper`.

## Features

- Single seam: `http.RoundTripper`, the same seam already used by `resilience`
  and `otelhttp`.
- Built-in observability: the base transport is wrapped with `otelhttp` (client
  span + trace propagation) and an active resilience executor is wrapped with
  fault + observe (outcome-classified metrics, access log gated by
  `Observability`). With no OTel SDK registered these are no-ops.
- Discovery + load balancing: when a `ServiceName` is set, wires a discovery
  `Resolver` and a `loadbalance.Pool` (round-robin, least-conn, consistent-hash,
  weighted, zone-aware) with optional outlier suspension.
- Direct addressing: with only `Addr`, rewrites every request to that host — the
  generated client's `Target` need not be set.
- Governance by default: when no explicit executor is supplied, one is resolved
  from the centralized governance authority under `Resource` (derived from
  ServiceName/Addr) — process-wide `govern.*` rules apply with hot-reload, plus
  an error-rate `min-requests=5` floor.
- TLS: the `tls.*` certificate surface (client pair, CA bundle, server name,
  insecure escape hatch) is built into the base transport.
- Optional `resilience` executor wrapping the whole chain, so a retry re-enters
  the balancer and picks a fresh endpoint, and the breaker keys on the logical
  service name.
- Fails fast at wiring time when a discovery backend or load-balancing strategy
  is misconfigured.

## Usage

```go
import "go-spring.org/cloud/httpx"

rt, closeFn, err := httpx.NewTransport(httpx.Config{
    ServiceName: "user-svc",     // omit for direct-address mode
    Discovery:   "redis",
    Balancer:    "round_robin",
})
if err != nil {
    log.Fatal(err)
}
defer closeFn()

client := &http.Client{Transport: rt}
```

For direct addressing, set `Addr` and leave `ServiceName` empty; `httpx` will
rewrite every request's host to `Addr`. `Base` (when set) is the raw underlying
transport — a TLS-configured clone, a custom dialer — tracing is layered on top
of it by `httpx` itself.

See `starter/starter-http-client` for the bean-oriented wrapper.
