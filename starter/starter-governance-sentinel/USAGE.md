# Usage — starter-governance-sentinel

This module makes sentinel-golang a **rate-limit algorithm** of the outbound
resilience engine. It is not a backend selection and not a second executor.

## 1. Import

```go
import _ "go-spring.org/starter-governance-sentinel"
```

At import the module:

- calls `sentinel.InitDefault()` — a broken environment panics on boot;
- registers a `ratelimit.Provider` exporting the algorithm name `sentinel`.

## 2. Select the algorithm

In the governance document (hot-reloadable):

```properties
spring.governance.rate-limit.services[0].service=orders
spring.governance.rate-limit.services[0].action=-
spring.governance.rate-limit.services[0].limiter=sentinel
spring.governance.rate-limit.services[0].rate-limit=100
spring.governance.rate-limit.services[0].window=1s
```

`limiter` is the algorithm name. Any provider in the process can export it;
`sentinel` is the one this module adds. Choosing it changes only the flow
decision — the engine still applies timeout, retry, breaker and matching.

Programmatically:

```go
fact, _ := ratelimit.NewFactory([]ratelimit.Provider{
	ratelimit.NewLocalProvider(),
	StarterGovernanceSentinel.Provider{},
})
mgr := resilience.NewManager(fact)
mgr.ApplyRateLimit(resilience.RateLimitSettings{Default: ratelimit.Policy{
	Limiter:   StarterGovernanceSentinel.LimiterAlgorithm,
	RateLimit: 100,
}})
exec := mgr.ExecutorFor("orders")
```

## 3. What maps

| Neutral knob | Sentinel rule field |
| --- | --- |
| `RateLimit` | `Threshold` (per second, normalized to `StatIntervalInMs`) |
| `Window` | `StatIntervalInMs` |
| `MaxWait` | `ControlBehavior = Throttling`, `MaxQueueingTimeMs` |
| `WarmUp` / `ColdFactor` | `TokenCalculateStrategy = WarmUp` (+ period / cold factor) |

Warm-up wins over queueing: sentinel pairs WarmUp only with Reject.

Not mapped (and not faked): `Burst` (sentinel has no burst), the local
algorithm kind (this provider IS the algorithm), and every non-flow stage
(timeout / retry / breaker / matching belong to the engine).

## 4. Troubleshooting

**`ratelimit: unknown algorithm "sentinel"`** — the module is not linked. The
provider is registered by the sentinel starter's `init`; a blank import (or an
import of a package that links it) is required.

**`panic: starter-governance-sentinel: sentinel init failed`** — sentinel's own
environment is broken (typically a bad `SENTINEL_*` setting). Fix the
environment; the framework refuses to boot with a half-initialized SDK.

**The limit looks shared across two rules of one service** — the engine scopes a
counter by rule identity, not by service alone. Give each rule its own `action`
(or work on the default rule, whose scope IS the service).

**`Burst` has no effect** — expected: sentinel's flow rule has no burst term.
Use `RateLimit` + `Window` (and `MaxWait` for queueing).

## 5. Tests

- `limiter_test.go` drives the provider end to end through the resilience
  engine: a policy naming `sentinel` admits its budget then returns the engine's
  neutral `ErrRateLimited`.
- The container test proves the import-time registration reaches the process
  limiter factory.
