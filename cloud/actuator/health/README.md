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

### Contribute a component health check

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

A few notes:

- `Name` is the key under which the component appears in the health report
  (e.g. `redis:cache`, `mysql:orders`); it must be unique within the
  application. For a multi-instance client, put the instance name in `Name`
  and let each instance contribute its own indicator, so the report shows
  each instance's status separately.
- `Probe` returns nil when healthy, or an error describing the failure. It
  must honor ctx deadline/cancellation — pass ctx to the underlying call
  (as in `client.Ping(ctx)` above) — otherwise a stuck dependency stalls the
  whole probe request.
- This package only reports; it does not expose. A collector such as
  `starter-actuator` autowires every `Indicator` bean and aggregates them,
  and decides what endpoint they surface and how often they are polled.

### Participate in selected probes only

For a downstream that must be reachable at startup but no longer gates
traffic afterwards (e.g. a warm-up data source), participate in the startup
probe only:

```go
&health.Indicator{
    Name:  "redis:" + name,
    Probe: probe,
    Groups: []health.Group{health.GroupStartup},
}
```

`Groups` declares which probe groups the indicator contributes to; combine
them freely, e.g. readiness + startup, or liveness + readiness.

### Optional dependencies

Mark an optional dependency (e.g. a pure acceleration cache) non-critical:
still reported — the failure stays visible in the report — but the aggregate
verdict is DEGRADED instead of DOWN and the pod is not taken out of rotation:

```go
&health.Indicator{Name: "redis:" + name, Probe: probe, Optional: true}
```

Symmetrically, when a required dependency fails, the aggregate verdict is
DOWN: the readiness probe fails and the pod is removed from Service
endpoints without being restarted — which is exactly why dependency checks
belong in readiness, not liveness.

## Probe groups

Groups map onto Kubernetes container probes: liveness / readiness / startup.
An indicator that declares no groups is routed by the collector to
readiness + startup, never liveness, so a transient downstream outage does
not restart the pod. Liveness must be declared explicitly in `Groups`, and
only for self-checks, never for downstream resources.
