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
spring.nats.instances.config-bus.url=nats://127.0.0.1:4222
```

### 3. Configure the bus (optional)

All keys live under `spring.config.bus`:

| Key             | Default                | Description                                                                 |
|-----------------|------------------------|-----------------------------------------------------------------------------|
| `subject`       | `spring.config.refresh`| NATS subject that refresh events are published to and subscribed from.      |
| `nats-instance` | `config-bus`           | Name of the `spring.nats.instances.*` connection used as the transport.     |
| `watch-prefixes`| (empty)                | Comma-separated prefixes; when set, only broadcasts whose prefix overlaps one of these (or full-fleet broadcasts) trigger a refresh on this instance. |
| `origin`        | (host name)            | Publisher label carried in each broadcast's `RefreshEvent.Origin` and in the producer span, for telling "this instance refreshed" from "some instance refreshed". |

### 4. Broadcast a refresh

Inject the bus via `autowire:"configBus"` and publish:

```go
type Service struct {
    Bus *StarterConfigBus.ConfigBus `autowire:"configBus"`
}

// Full-fleet refresh: every subscriber reloads. The ctx carries your trace, so
// a refresh triggered from an HTTP handler shows up as part of that request.
_ = svc.Bus.Publish(ctx, "")

// Scoped refresh: prefix-scoped subscribers may opt out.
_ = svc.Bus.Publish(ctx, "db")
```

Every instance subscribing to the subject re-runs `RefreshProperties`, so all
bound `gs.Dync` fields update live. See [example](example/example.go) for the
full broadcast to refresh flow.

## How It Works

- On startup the `ConfigBus` bean is created eagerly (it exports `gs.Rooter`
  under the name `configBus`) and subscribes to `spring.config.bus.subject` on
  the configured NATS connection.
- `Publish(ctx, prefix)` sends a small JSON `RefreshEvent{prefix, origin}` on the
  subject. An empty prefix means a full-fleet refresh; a non-empty prefix lets
  prefix-scoped subscribers opt out (an event applies when its prefix overlaps a
  watched prefix in either direction, so a `db` watcher reacts to a `db.pool`
  event and vice versa). The ctx parents the producer span on your trace.
- On receipt each subscriber calls the framework's process-level
  `gs.RefreshProperties()` facade, which reloads all configuration sources and
  re-binds every `gs.Dync` field via a two-phase, atomic commit.
- The bus does not own the NATS connection: it injects a `*StarterNats.Conn` by
  instance name and leaves lifecycle and close to `starter-nats`.

## Design Notes

**Signals only, never configuration content.** The bus tells subscribers
"reload from your own sources now"; the config center or local files remain the
single source of truth. A malformed message is warn-logged and dropped; an empty
payload is a valid full-fleet refresh. Applications must never rely on message
bodies as configuration.

**Refresh failures never unsubscribe the instance.** A failed `RefreshProperties`
is counted (`refresh_error`), logged, and returned to the transport so the
consumer span is marked failed — but the subscription stays up. One broken
refresh must not silently drop an instance out of the fleet.

**The named root object is load-bearing.** `gs.Rooter` is `any`, so the bean is
registered under the explicit name `configBus` (never `__default__`); that name
is also the autowire handle for `Publish` callers.

**Broadcasting configuration content — rejected.** It would make the bus a
second source of truth and race with each subscriber's own config-center watch.

**Other transports (Kafka/Redis) — deferred.** NATS was chosen first because a
Go-Spring app is likely to already run it; a second transport can be a peer
starter with the same subject/prefix model.

**JetStream durable subscriptions — deliberately not used.** A missed broadcast
is recoverable: the instance's own remote config watcher observes the underlying
change on its next tick, and admins can always republish. Durability would add
operational cost without changing the correctness model.

## Observability

A broadcast is fire-and-forget — core NATS gives the publisher no
acknowledgement — so nothing but the subscriber can report whether a refresh
actually happened. The bus therefore reports one outcome per event, driving both
a metric and a log line so the two can never disagree:

| Metric | Outcome values |
| --- | --- |
| `config.bus.events` | `refreshed` · `ignored_prefix` · `malformed` · `refresh_error` |
| `config.bus.refresh.duration` | `refreshed` · `refresh_error` |
| `config.bus.publishes` | `ok` · `error` |

The outcomes are exclusive, so `config.bus.events` summed over `outcome` is the
number of broadcasts received — there is no separate counter that could
double-count. `refresh_error` is the one worth alerting on: the signal arrived
and was honored, but the property reload failed, leaving the instance on stale
configuration.

Traces come from the transport: `Publish` opens a producer span (parented on your
`ctx`) and injects the W3C context into the message header, and the subscription's
consumer span continues it, so one broadcast appears as a single trace spanning
the publisher and every subscriber. Both are no-ops without starter-otel.

Health: the bus registers a `health.Indicator` named `config-bus:configBus`,
which reports whether the subscription is still active. That is deliberately not
a connectivity check — a subscription survives a reconnect, so this is false only
when the listener is genuinely dead, the case a NATS connectivity probe cannot
see and the one that leaves an instance silently stuck on stale configuration.
Connection-level health is starter-nats's own indicator, with its own per-instance
`health.enabled` switch.

### Log tag

Runtime logs from this module carry the tag `_app_config_bus` (config bus). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.config_bus.type=Logger
logger.config_bus.level=WARN
logger.config_bus.tag=_app_config_bus
```
