# starter-governance-sentinel

[English](README.md) | [中文](README_CN.md)

`starter-governance-sentinel` registers [alibaba/sentinel-golang][sentinel] as the
production driver for the resilience framework defined in
[`cloud/resilience`](../../cloud/resilience). Blank-import it
and name it in the
governance document (`spring.resilience.driver=sentinel`) — every client that resolves its
executor through the governance center then gets adaptive rate limiting,
circuit breaking, and bulkhead isolation on top of the same neutral `ClientPolicy`,
with no per-client key and no code change.

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it contributes exactly one bean — the
`sentinel`-named `resilience.Driver` that [`cloud/resilience`](../../cloud/resilience)
injects when `spring.resilience.driver=sentinel` — and opens no port.
`sentinel.InitDefault`
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
`resilience.Driver` — the resilience manager injects that named bean, so nothing
else registers or looks anything up.

### 2. Select it in app config

The driver is a static, startup choice — one key for the whole process, not per
client: `spring.resilience.driver` names the backend, and every client that
resolves its executor through the resilience manager runs on it. It is a plain
`app.properties` key (the backend is fixed for the process's lifetime — switching
it drops every executor, so it takes a restart), unlike the `spring.governance.*`
policy keys, which live in the governance document — its own system (see
[`cloud/governance/README.md`](../../cloud/governance/README.md)):

```properties
# app.properties: pick the backend (static)
spring.resilience.driver=sentinel

# governance document: policy (hot-reloadable)
spring.governance.client.default.max-retries=3
spring.governance.client.default.error-threshold=10
spring.governance.client.default.attempt-timeout=1s
```

### 3. Or drive it directly

```go
import "go-spring.org/cloud/resilience"

exec, _ := starter_governance_sentinel.NewSentinelDriver().NewExecutor("sentinel:test", resilience.ClientPolicy{
    RateLimit:      100,
    ErrorThreshold: 10,
    OpenDuration:   30 * time.Second,
    MaxRetries:     3,
    Timeout:        time.Second,
})

// client-side transport
client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, nil)}
```

See [`example/`](example) for a self-contained smoke that asserts the composed
rate limit + breaker + retry seams end to end (no docker required).

## Policy mapping

The neutral `resilience.ClientPolicy` / `ServerPolicy` translate through one
shared mapping (rules.go — both engines go through it, so the directions cannot
drift) onto sentinel rules per service, loaded lazily on the first entry:

| Policy knob | Sentinel rule | Neutral error on trip |
| --- | --- | --- |
| `RateLimit` + `Window` | flow `Threshold` (**normalized per window**: per-second → per `StatIntervalInMs`) + `StatIntervalInMs` | `ErrRateLimited` |
| `RateLimitMaxWait` | flow `Throttling` + `MaxQueueingTimeMs` (queue instead of reject) | `ErrRateLimited` |
| `RateLimitWarmUpDuration` / `WarmUpColdFactor` | flow `WarmUp` + `WarmUpPeriodSec` / `WarmUpColdFactor` (warm-up and queueing are exclusive — sentinel allows WarmUp+Reject only; warm-up wins) | `ErrRateLimited` |
| `breaker-strategy=error-rate` | breaker `ErrorRatio` | `ErrCircuitOpen` |
| `breaker-strategy=slow-call` (+ slow pair) | breaker `SlowRequestRatio` + `MaxAllowedRtMs` | `ErrCircuitOpen` |
| `breaker-strategy=error-count` | breaker `ErrorCount` (exact) | `ErrCircuitOpen` |
| `breaker-strategy=consecutive` (default) | breaker `ErrorCount` — **approximation**: sentinel has no consecutive-run breaker; write `error-count` for an exact mapping | `ErrCircuitOpen` |
| `HalfOpenRequests` | `ProbeNum` | — |
| `OpenDuration` | `RetryTimeoutMs` | — |
| `MaxConcurrent` | isolation `Concurrency` | `ErrBulkheadFull` |
| `MaxRetries` | retry loop (outside sentinel) | last attempt's error |
| `AttemptTimeout` / `MaxDuration` | ctx deadline (outside sentinel) | `context.DeadlineExceeded` |

**Legitimate ignores** (sentinel v1.0.4 has no counterpart; documented, never
faked): `Burst`, `Algorithm` (no sliding-window/burst knobs on flow), and
`BulkheadQueueDuration` (isolation cannot queue — the default driver does
implement the wait).

Sentinel's system / hotspot / cluster rules are future work: they are inbound-
or parameter-dimension primitives with no policy fields to arrive through yet;
when fields appear, they land in the server executor.

## Default driver

`cloud/resilience` ships a zero-dependency `default` driver for tests and
lightweight setups. Import this starter for production-grade throttling and
breaking; stick with `default` if you don't need it.

## Design Notes

* **This starter is the engine, not the seam.** It decides nothing about *where*
  resilience applies — the adapter does: HTTP clients wrap through
  `resilience.NewRoundTripper`, and inbound admission middlewares are built by
  the protocol starters. On its own it protects no call.
* **Rules install lazily, keyed by service name.** Sentinel keys everything by
  service, so a service's policy → rule translation happens on its first
  `Entry`, guarded by a mutex; a service that is never called never gets rules.
* **Sentinel is pinned to v1.0.4.** Later minors reshuffled the flow /
  circuit-breaker rule struct fields; the pin keeps an upstream regression
  surfacing here rather than at every adapter.
