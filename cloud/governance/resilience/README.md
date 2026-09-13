# resilience
[English](README.md) | [中文](README_CN.md)

`resilience` is a framework-agnostic client-fault-tolerance abstraction for
client-side fault tolerance: rate limiting, circuit breaking, bulkhead
isolation, retry, per-attempt timeout, and fallback. Client starters plug
the single `Executor` seam into their own request hook (HTTP RoundTripper,
Redis Hook, GORM plugin, ...); a distributed rate limiter uses the parallel
`RateLimiter` seam.

## Features

- `Policy` fields: `RateLimit` / `Burst`, `ErrorThreshold` / `OpenDuration`,
  `MaxConcurrent`, `MaxRetries`, `Timeout`.
- Neutral rejection errors: `ErrRateLimited`, `ErrCircuitOpen`,
  `ErrBulkheadFull`.
- Bundled `"default"` driver — in-process, zero dependencies. Recommended
  production driver `sentinel` lives in `starter/starter-governance-sentinel`.
- Two client seams for opt-in adaptation:
  - `NewRoundTripper` — HTTP client `http.RoundTripper` (widest coverage).
  - `NewDialer` — connection-level `DialFunc` (matches
    a dial closure over a round-robin pick pool).
- Inbound admission is NOT built here: protocol starters build their own
  middleware on the `resilience.ExecutorFor` seam (429 / 503 on rejection;
  see starter-gin / starter-grpc admission).
- `Fallback(ctx, exec, resource, fn, degrade)` — graceful degradation
  helper that composes with any executor.
- Standalone `RateLimiter` + `LimiterDriver` seam (built-in token bucket
  and sliding window; a Redis-backed limiter lives in `starter-go-redis`
  for a globally shared budget).

## Pluggable backends

This package holds **no registry**. A backend is a bean named after itself
and exported as `Driver` (or `LimiterDriver`); the consumer collects them
into a name-keyed directory and resolves the configured name through
`resilience.Resolve`. The bundled backends answer to `"default"` without a
bean, via `NewDefaultDriver()` / `NewDefaultLimiterDriver()`.

```go
// contribute a backend (typically from a starter's init)
gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
    Name("sentinel").
    Export(gs.As[resilience.Driver]())
```

The `Export` is load-bearing: gs indexes beans by their exact type, so a
concrete driver without it is invisible to the directory.

> **Breaking change (2026-09-13).** The package-level registries were
> removed: `RegisterDriver`, `GetDriver`, `NewExecutor(name, policy)`,
> `RegisterLimiter`, `GetLimiter`. Migrate as follows — `GetDriver("x")` →
> the `"x"`-named bean (resolved through `resilience.Resolve` against the
> injected directory); `NewExecutor("default", p)` →
> `NewDefaultDriver().NewExecutor(p)`; `RegisterDriver("x", d)` →
> contribute the bean as above. `starter-go-redis/experimental`'s
> `RegisterLimiterDriver(name, client)` became
> `NewLimiterDriver(client)` (contribute the returned value as a bean).
> `govern.driver` and every config key are unchanged.

## Installation

```
go get go-spring.org/cloud
```

## Usage

Guard an HTTP client:

```go
import (
    "net/http"

    "go-spring.org/cloud/governance/resilience"
)

exec, _ := resilience.NewDefaultDriver().NewExecutor(resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 5,
    MaxRetries:     2,
    Timeout:        2 * time.Second,
})

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec,
        func(r *http.Request) string { return r.URL.Host }),
}
```

Combine with `cloud/discovery` at the dial layer:

```go
ld, _ := discovery.NewClientDialer(ctx, "default", "orders")
dial  := resilience.NewDialer(ld.DialContext, exec, "orders")
```

Standalone RateLimiter (distributed budget via a starter):

```go
limiter, _ := resilience.NewDefaultLimiterDriver().NewRateLimiter(
    resilience.LimitPolicy{Rate: 100, Burst: 100})
if ok, _ := limiter.Allow(ctx, "tenant:42"); !ok { /* reject */ }
```
