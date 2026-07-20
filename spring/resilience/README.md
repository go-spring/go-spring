# resilience
[English](README.md) | [中文](README_CN.md)

`resilience` is a framework-agnostic, zero-dependency abstraction for
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
  production driver `sentinel` lives in `starter/starter-resilience`.
- Three seams for opt-in adaptation:
  - `NewRoundTripper` — HTTP client `http.RoundTripper` (widest coverage).
  - `NewDialer` — connection-level `DialFunc` (matches
    `discovery.LiveDialer.DialContext`).
  - `NewHandler` — inbound HTTP admission; 429 / 503 on rejection.
- `Fallback(ctx, exec, resource, fn, degrade)` — graceful degradation
  helper that composes with any executor.
- Standalone `RateLimiter` + `LimiterDriver` registry (built-in token
  bucket and sliding window; a Redis-backed limiter lives in
  `starter-go-redis` for a globally shared budget).

## Installation

```
go get go-spring.org/stdlib
```

## Usage

Guard an HTTP client:

```go
import (
    "net/http"

    "go-spring.org/spring/resilience"
)

drv, _ := resilience.MustGetDriver("default")
exec, _ := drv.NewExecutor(resilience.Policy{
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

Combine with `spring/discovery` at the dial layer:

```go
ld, _ := discovery.NewClientDialer(ctx, "default", "orders")
dial  := resilience.NewDialer(ld.DialContext, exec, "orders")
```

Rate-limit inbound requests:

```go
handler := resilience.NewHandler(mux, exec, func(r *http.Request) string { return r.URL.Path })
```

Standalone RateLimiter (distributed budget via a starter):

```go
ldrv, _ := resilience.MustGetLimiter("default") // or "redis" from starter-go-redis
limiter, _ := ldrv.NewRateLimiter(resilience.LimitPolicy{Rate: 100, Burst: 100})
if ok, _ := limiter.Allow(ctx, "tenant:42"); !ok { /* reject */ }
```
