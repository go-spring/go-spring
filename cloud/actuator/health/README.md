# health

[English](README.md) | [中文](README_CN.md)

Component health-check contract. A component that can report its own health
(database pool, cache client, message-queue connection) builds an `Indicator`
value and exports it as a bean; a collector (e.g. `starter-actuator`)
autowires all of them and serves readiness / startup / liveness probes.

## Installation

```
go get go-spring.org/cloud
```

## Usage

Contribute a health check for a Redis client:

```go
import (
    "context"

    "github.com/redis/go-redis/v9"
    "go-spring.org/gs"
    "go-spring.org/cloud/actuator/health"
)

func newRedisHealth(name string, client redis.UniversalClient) *health.Indicator {
    return &health.Indicator{
        Name:  "redis:" + name,
        Probe: func(ctx context.Context) error { return client.Ping(ctx).Err() },
    }
}

func init() {
    gs.Provide(newRedisHealth, gs.ValueArg("cache"), gs.TagArg("cache"))
}
```

Participate in the startup probe only:

```go
&health.Indicator{
    Name:  "redis:" + name,
    Probe: probe,
    Groups: []health.Group{health.GroupStartup},
}
```

Mark an optional dependency non-critical: still reported, but a failure does
not take the pod out of rotation:

```go
&health.Indicator{Name: "redis:" + name, Probe: probe, Optional: true}
```

## Probe groups

Groups map onto Kubernetes container probes: liveness / readiness / startup.
An indicator that declares no groups is routed by the collector to
readiness + startup, never liveness, so a transient downstream outage does
not restart the pod. Liveness must be declared explicitly in `Groups`, and
only for self-checks, never for downstream resources.
