# starter-resilience

[English](README.md) | [中文](README_CN.md)

`starter-resilience` registers [alibaba/sentinel-golang][sentinel] as the
recommended driver for the resilience framework defined in
[`stdlib/resilience`](../../stdlib/resilience). Blank-import it and any starter
or user code that selects `driver=sentinel` — HTTP round-trippers, dialers,
inbound handlers, retry policies — gets adaptive rate limiting, circuit
breaking, and bulkhead isolation on top of the same neutral `Policy`.

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it registers no bean and opens no
port. `sentinel.InitDefault` runs at import time so a broken environment
fails loudly on boot rather than on first use.

[sentinel]: https://github.com/alibaba/sentinel-golang

## Installation

```bash
go get go-spring.org/starter-resilience
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-resilience"
```

The `init` function calls `sentinel.InitDefault()` (panicking on failure) and
then `resilience.RegisterDriver("sentinel", ...)`.

### 2. Point an adapter at the sentinel driver

Any starter or library built on `stdlib/resilience` selects a driver by name.
For example, `starter-oauth2-client` reads
`spring.http.client.<name>.resilience.driver`:

```properties
spring.http.client.default.resilience.driver=sentinel
spring.http.client.default.resilience.rate-limit=100
spring.http.client.default.resilience.error-threshold=10
spring.http.client.default.resilience.open-duration=30s
spring.http.client.default.resilience.max-retries=3
spring.http.client.default.resilience.timeout=1s
```

### 3. Or drive it directly

```go
import "go-spring.org/stdlib/resilience"

driver, _ := resilience.MustGetDriver("sentinel")
exec, _ := driver.NewExecutor(resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 10,
    OpenDuration:   30 * time.Second,
    MaxRetries:     3,
    Timeout:        time.Second,
})

// server-side admission
handler := resilience.NewHandler(mux, exec, func(*http.Request) string { return "hello" })

// client-side transport
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, nil)}

// client-side dial
dial := resilience.NewDialer(baseDialer, exec, "upstream")
```

See [`example/`](example) for a self-contained smoke that asserts the
`Handler`, `Dialer`, and composed rate limit + breaker + retry seams end to
end (no docker required).

## Policy mapping

The neutral `resilience.Policy` translates onto sentinel rules per resource,
loaded lazily on the first entry:

| `Policy` field   | Sentinel rule       | Neutral error on trip    |
| ---------------- | ------------------- | ------------------------ |
| `RateLimit`      | flow (Direct/Reject)| `ErrRateLimited`         |
| `ErrorThreshold` | circuit breaker     | `ErrCircuitOpen`         |
| `OpenDuration`   | breaker retry-after | —                        |
| `MaxConcurrent`  | isolation           | `ErrBulkheadFull`        |
| `MaxRetries`     | retry loop          | last attempt's error     |
| `Timeout`        | per-attempt ctx     | `context.DeadlineExceeded` |

`RateLimit`, `ErrorThreshold`, and `MaxConcurrent` become sentinel rules;
`MaxRetries` and `Timeout` are applied by the executor around sentinel's
entry check, since sentinel itself models neither. Sentinel block reasons are
mapped onto the neutral sentinels so callers depend only on
`stdlib/resilience`.

## Default driver

`stdlib/resilience` ships a zero-dependency `default` driver used for tests
and lightweight setups. Import this starter to get production-grade
throttling and breaking; keep `default` if you do not need it.
