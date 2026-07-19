# health
[English](README.md) | [中文](README_CN.md)

`health` is a framework-agnostic, zero-dependency abstraction for component
health checks. A component that can report its own health (database pool,
cache client, message-queue connection, ...) implements the `Indicator`
interface and is exported as a bean; a collector (e.g. `starter-actuator`)
autowires all of them for readiness, startup, and liveness probes.

## Features

- Single required interface `Indicator { HealthName() string;
  CheckHealth(ctx) error }`.
- `Status` verdicts: `StatusUp` / `StatusDown`.
- Kubernetes probe groups: `GroupLiveness`, `GroupReadiness`, `GroupStartup`.
- Optional `Grouped` interface for indicators that only want to contribute
  to specific probes; the safe default (readiness + startup) applies when
  it is not implemented.
- `GroupsOf` / `InGroup` helpers so collectors filter indicators per probe.

## Installation

```
go get go-spring.org/stdlib
```

## Usage

Expose a component's health with no dependency on the collector:

```go
import (
    "context"

    "go-spring.org/gs"
    "go-spring.org/stdlib/health"
)

type redisHealth struct {
    name   string
    client *redis.Client
}

func (r *redisHealth) HealthName() string { return "redis:" + r.name }

func (r *redisHealth) CheckHealth(ctx context.Context) error {
    return r.client.Ping(ctx).Err()
}

func newRedisHealth(name string, c *redis.Client) health.Indicator {
    return &redisHealth{name: name, client: c}
}

func init() {
    gs.Provide(newRedisHealth, gs.ValueArg("cache"), gs.TagArg("cache")).
        Export(gs.As[health.Indicator]())
}
```

Restrict to startup only (e.g. a bootstrap dependency):

```go
func (r *redisHealth) HealthGroups() []health.Group { return []health.Group{health.GroupStartup} }
```
