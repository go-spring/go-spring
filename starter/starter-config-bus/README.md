# starter-config-bus

[English](README.md) | [中文](README_CN.md)

`starter-config-bus` adds a **configuration refresh bus** on top of an existing
[NATS](https://nats.io/) connection (from `starter-nats`). Blank-importing it
registers a `ConfigBus` bean that subscribes to a refresh subject and re-runs
the application-wide property refresh whenever a signal arrives — so a change
broadcast **once** refreshes **every** instance in the fleet, the Go equivalent
of Spring Cloud Bus's refresh broadcast.

It complements the remote config-center starters
(`starter-config-{nacos,etcd,consul}`): those already refresh a single instance
from their own watch, while the bus covers cross-instance broadcast and
refreshes triggered from outside the config center (for example a forced,
fleet-wide reload). The bus carries refresh **signals only** — never
configuration content, which stays with the config center or local files.

## Installation

```bash
go get go-spring.org/starter-config-bus
```

## Quick Start

### 1. Import the package (and starter-nats)

```go
import (
    _ "go-spring.org/starter-config-bus"
    _ "go-spring.org/starter-nats"
)
```

### 2. Point the bus at a NATS connection

Define a NATS instance whose name matches `spring.config.bus.nats-instance`
(default `config-bus`):

```properties
spring.nats.config-bus.url=nats://127.0.0.1:4222
```

### 3. Configure the bus (optional)

All keys live under `spring.config.bus`:

| Key             | Default                | Description                                                                 |
|-----------------|------------------------|-----------------------------------------------------------------------------|
| `subject`       | `spring.config.refresh`| NATS subject that refresh events are published to and subscribed from.      |
| `nats-instance` | `config-bus`           | Name of the `spring.nats.*` connection used as the transport.     |
| `watch-prefixes`| (empty)                | Comma-separated prefixes; when set, only broadcasts whose prefix overlaps one of these (or full-fleet broadcasts) trigger a refresh on this instance. |

### 4. Broadcast a refresh

Inject the bus via `autowire:"configBus"` and publish:

```go
type Service struct {
    Bus *StarterConfigBus.ConfigBus `autowire:"configBus"`
}

// Full-fleet refresh: every subscriber reloads.
_ = svc.Bus.Publish("")

// Scoped refresh: prefix-scoped subscribers may opt out.
_ = svc.Bus.Publish("db")
```

Every instance subscribing to the subject re-runs `RefreshProperties`, so all
bound `gs.Dync` fields update live. See [example](example/example.go) for the
full broadcast → refresh flow.

## How It Works

- On startup the `ConfigBus` bean is created eagerly (it exports `gs.Rooter`
  under the name `configBus`) and subscribes to `spring.config.bus.subject` on
  the configured NATS connection.
- `Publish(prefix)` sends a small JSON `RefreshEvent{prefix}` on the subject. An
  empty prefix means a full-fleet refresh; a non-empty prefix lets
  prefix-scoped subscribers opt out.
- On receipt each subscriber calls the framework's `PropertiesRefresher`, which
  reloads all configuration sources and re-binds every `gs.Dync` field via a
  two-phase, atomic commit.
