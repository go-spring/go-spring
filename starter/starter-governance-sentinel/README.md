# starter-governance-sentinel

[English](README.md) | [中文](README_CN.md)

`starter-governance-sentinel` registers [alibaba/sentinel-golang][sentinel] as the
production driver for the resilience framework defined in
[`cloud/governance/resilience`](../../cloud/governance/resilience). Blank-import it
alongside [`starter-governance`](../starter-governance) and name it in the
governance document (`govern.driver=sentinel`) — every client that resolves its
executor through the governance center then gets adaptive rate limiting,
circuit breaking, and bulkhead isolation on top of the same neutral `Policy`,
with no per-client key and no code change.

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it contributes exactly one bean — the
`sentinel`-named `resilience.Driver` that [`starter-governance`](../starter-governance)
collects into the driver directory — and opens no port. `sentinel.InitDefault`
runs at import time so a broken environment fails loudly on boot rather than on
first use.

[sentinel]: https://github.com/alibaba/sentinel-golang

## Installation

```bash
go get go-spring.org/starter-governance-sentinel
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-governance-sentinel"
```

The `init` function calls `sentinel.InitDefault()` (panicking on failure) and
then contributes the backend as a bean named `sentinel`, exported as
`resilience.Driver` — the container is the driver directory, so nothing else
registers or looks anything up.

### 2. Select it in the governance document

The driver is chosen once for the whole process, not per client: the governance
document names it in `govern.driver`, and every client that resolves its
executor through the governance center picks it up. The `govern.*` keys live in
the governance document — its own system, not `app.properties` (see
[`cloud/governance/README.md`](../../cloud/governance/README.md)):

```properties
govern.enabled=true
govern.driver=sentinel
govern.default.max-retries=3
govern.default.error-threshold=10
govern.default.attempt-timeout=1s
```

### 3. Or drive it directly

```go
import "go-spring.org/cloud/governance/resilience"

exec, _ := starter_governance_sentinel.NewSentinelDriver().NewExecutor(resilience.Policy{
    RateLimit:      100,
    ErrorThreshold: 10,
    OpenDuration:   30 * time.Second,
    MaxRetries:     3,
    Timeout:        time.Second,
})

// client-side transport
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, nil)}

// client-side dial
dial := resilience.NewDialer(baseDialer, exec, "upstream")
```

See [`example/`](example) for a self-contained smoke that asserts the `Dialer`
and the composed rate limit + breaker + retry seams end to end (no docker
required).

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
entry check, since sentinel models neither. Sentinel block reasons are
mapped onto the neutral sentinels so callers depend only on
`cloud/governance/resilience`.

## Default driver

`cloud/governance/resilience` ships a zero-dependency `default` driver for tests and
lightweight setups. Import this starter for production-grade throttling and
breaking; stick with `default` if you don't need it.
