# starter-governance-sentinel

[English](README.md) | [中文](README_CN.md)

`starter-governance-sentinel` lets [alibaba/sentinel-golang][sentinel] answer the
**rate-limit decision** of the outbound resilience engine.

It contributes a `ratelimit.Provider` bean exporting the algorithm name
`sentinel`. A governance policy selects it with `limiter: sentinel`, exactly
like the bundled `token-bucket`; the resilience engine keeps ownership of
timeout, retry, circuit breaking, rule matching, per-rule keys and metrics, so
there is no second executor and no duplicated pipeline.

[sentinel]: https://github.com/alibaba/sentinel-golang

## Installation

```bash
go get go-spring.org/starter-governance-sentinel
```

## Quick Start

```go
import _ "go-spring.org/starter-governance-sentinel"
```

Importing the module calls `sentinel.InitDefault()` (panicking on failure) and
registers the provider with the process limiter factory.

Select it from the governance document:

```properties
# governance document
spring.governance.rate-limit.services[0].service=orders
spring.governance.rate-limit.services[0].limiter=sentinel
spring.governance.rate-limit.services[0].rate-limit=100
```

Or programmatically:

```go
import (
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	StarterGovernanceSentinel "go-spring.org/starter-governance-sentinel"
)

fact, _ := ratelimit.NewFactory([]ratelimit.Provider{
	ratelimit.NewLocalProvider(),
	StarterGovernanceSentinel.Provider{},
})
mgr := resilience.NewManager(fact)
_ = mgr.ApplyRateLimit(resilience.RateLimitSettings{Default: ratelimit.Policy{
	Limiter:   StarterGovernanceSentinel.LimiterAlgorithm,
	RateLimit: 100,
}})
exec := mgr.ExecutorFor("orders")

client := &http.Client{Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, "orders", nil)}
```

An algorithm name no provider exports is rejected when the document is applied,
so a typo fails at the source boundary rather than at the first call.

## Policy mapping

| Neutral knob | Sentinel flow rule |
| --- | --- |
| `RateLimit` + `Window` | `Threshold` per `StatIntervalInMs` (normalized: per-second → per-window) |
| `MaxWait` | `Throttling` + `MaxQueueingTimeMs` (queue instead of reject) |
| `WarmUp` / `ColdFactor` | `WarmUp` token strategy; warm-up wins over queueing (sentinel pairs WarmUp with Reject only) |

Knobs a limiter legitimately cannot map are not faked: `Burst` has no sentinel
counterpart, and the rule's algorithm kind is `sentinel` rather than
`token-bucket`.

## Boundaries

- **Rate limiting only.** Timeout, retry, the circuit breaker, per-call rule
  matching (service/action/metadata) and the metrics/state plane all stay in the
  resilience engine.
- **No global selection.** The provider is selected per rule by algorithm name;
  there is no `spring.resilience.driver` key and no backend bean.
- **Per-rule budgets.** The engine passes the rule identity as the counter
  scope, so two rules of one service keep independent budgets (the default
  rule's scope is the service label).
- **Sentinel v1.0.4 pin.** Later minors reshuffled rule struct fields; the pin
  keeps the translation stable.

See [`example/`](example) for a self-contained smoke (no docker required).
