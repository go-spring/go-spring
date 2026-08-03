# health
[English](README.md) | [中文](README_CN.md)

`health` is a framework-agnostic, zero-dependency abstraction for component
health checks. A component that can report its own health (database pool,
cache client, message-queue connection, ...) implements the `Indicator`
interface and is exported as a bean; a collector (e.g. `starter-actuator`)
autowires all of them for readiness, startup, and liveness probes.

## Features

- `Indicator` interface: `HealthName`, `CheckHealth`, `HealthGroups`,
  `IsCritical` - identity, the check, probe routing, and severity.
- `Status` verdicts: `StatusUp` / `StatusDown`.
- Kubernetes probe groups: `GroupLiveness`, `GroupReadiness`, `GroupStartup`.
- `NewIndicator` factory with `WithGroups` (declare probe groups) and
  `NonCritical` (report without failing the aggregate) options. Without
  options the indicator is critical and declares no explicit groups; the
  collector applies its default routing (readiness + startup, never liveness).

## Installation

```
go get go-spring.org/spring
```

## Usage

Expose a component's health with no dependency on the collector:

```go
import (
    "context"

    "github.com/redis/go-redis/v9"
    "go-spring.org/gs"
    "go-spring.org/spring/cloud/actuator/health"
)

func newRedisHealth(name string, client *redis.Client) health.Indicator {
    return health.NewIndicator("redis:"+name, func(ctx context.Context) error {
        return client.Ping(ctx).Err()
    })
}

func init() {
    gs.Provide(newRedisHealth, gs.ValueArg("cache"), gs.TagArg("cache")).
        Export(gs.As[health.Indicator]())
}
```

Contribute to startup only (a bootstrap dependency):

```go
health.NewIndicator("redis:"+name, probe, health.WithGroups(health.GroupStartup))
```

Mark an optional cache non-critical - reported but never takes the pod out of
rotation:

```go
health.NewIndicator("redis:"+name, probe, health.NonCritical())
```
