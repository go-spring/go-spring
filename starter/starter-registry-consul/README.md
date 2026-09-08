# starter-registry-consul

[English](README.md) | [中文](README_CN.md)

`starter-registry-consul` registers the **current instance** into a Consul
service registry — the provider-side counterpart to Go-Spring's client-side
discovery (`cloud/discovery`) — **and** provides the consumer-side Consul
discovery backend, so one starter serves both halves of the naming idiom. It is
the Go-Spring equivalent of Spring Cloud's `ServiceRegistry` + `DiscoveryClient`,
backed by Consul TTL health checks.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-discovery-k8s](../starter-discovery-k8s) and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
Consul. RPC-framework provider registration stays framework-native and is out of
scope (see [starter/DESIGN §3](../../DESIGN.md)).

## Archetype

Global / infrastructure (see [starter/DESIGN §2.4](../../DESIGN.md)): it opens no
port. It exports a `gs.Server` so registration plugs into the server lifecycle —
the instance is published **once the application is ready** and deregistered
**as shutdown begins** (via `PreStop`), so discovery stops handing it out before
it actually stops serving. That ordering is what makes a rolling restart
lossless.

## Installation

```bash
go get go-spring.org/starter-registry-consul
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-consul"
```

### 2. Configure the Consul agent and the instance

```properties
# Consul agent (setting the address activates the starter).
spring.registry.consul.address=127.0.0.1:8500
spring.registry.consul.ttl=15s
spring.registry.consul.deregister-critical-after=1m

# The instance to advertise (backend-agnostic).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is registered and kept alive by a TTL
heartbeat; on shutdown it is deregistered. The same `spring.registry.consul`
block also derives a discovery backend bean named `consul` (see below), so a
client elsewhere — or this process itself — resolves it by service name through
the shared connection.

## Configuration

### Registering (this instance)

Connection, bound under `spring.registry.consul`:

| Key | Default | Description |
| --- | --- | --- |
| `address` | (required) | Consul HTTP API address; setting it activates the starter. |
| `scheme` | `http` | `http` or `https`. |
| `datacenter` | (empty) | Datacenter to register into; empty uses the agent's. |
| `token` | (empty) | ACL token. |
| `namespace` | (empty) | Consul Enterprise namespace. |
| `ttl` | `15s` | TTL health check; the starter heartbeats at half this interval. |
| `deregister-critical-after` | `1m` | Consul drops the instance if its check stays critical this long (e.g. after a crash). |
| `discovery-name` | `consul` | Derives a discovery backend bean for this same agent under that label — one config block serves both halves (shared client). Empty disables the derived backend. |

Instance, bound under `spring.registry` (backend-agnostic — switching registry
backends is a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `0` | Load-balancing weight; `0` uses Consul's default. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |

### Discovering (other instances)

The consumer half is a `cloud/discovery` backend bean. Two ways to get one:

**Derived from the center (dual-role apps).** Setting `spring.registry.consul`
alone derives a backend bean labeled `discovery-name` (default `consul`) on the
same shared client — one agent config, both halves:

```properties
spring.registry.consul.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# clients then cite: <client>.discovery=consul
```

**Standalone blocks (other agents / pure consumers).** One block per Consul
agent under `spring.discovery.consul.<name>`; a client starter cites the name:

```properties
spring.discovery.consul.prod.address=127.0.0.1:8500
spring.discovery.consul.prod.tag=v2
```

| Key | Default | Description |
| --- | --- | --- |
| `address` | (empty) | Consul HTTP API address; empty INHERITS the `spring.registry.consul` center connection (shared client). |
| `scheme` | `http` | `http` or `https`. |
| `datacenter` | (empty) | Datacenter to query; empty uses the agent's. |
| `token` | (empty) | ACL token. |
| `namespace` | (empty) | Consul Enterprise namespace. |
| `tag` | (empty) | Consul service tag narrowing every query (`Health().Service`'s tag argument). |

Resolve asks Consul for **passing** instances only, so unhealthy instances
never enter the snapshot. Freshness is a background Consul blocking query
(index-based long poll) per resolved service: the first Resolve seeds a cache,
the blocking query keeps it current, later Resolves are in-memory reads. The
advertised passing weight becomes the endpoint weight; an optional `scheme`
meta key carries transport selection, mirroring the etcd/nacos adapters.

## How It Works

- During bean construction the starter builds the shared Consul client for
  `spring.registry.consul` (probing the agent so an unreachable one fails
  startup) and derives both the registrar and the `discovery-name`-labeled
  discovery backend from it. The registrar is wired into the exported
  `gs.Server`.
- The exported `gs.Server` waits for readiness, then `Register`s the instance
  with a Consul **TTL health check**. It passes the check immediately and keeps
  it passing on a background heartbeat at half the TTL.
- On shutdown `PreStop` deregisters the instance (stopping the heartbeat and
  removing it from Consul) before the pre-stop delay, so discovery removes it
  while in-flight requests keep being served. `Stop` deregisters again as an
  idempotent fallback.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts a Consul dev agent, boots [example](example/example.go) (which
registers, resolves its own registration back through the derived discovery
backend, then SIGTERMs itself so the deregister path runs), and asserts the
instance appeared. It is skipped gracefully without Docker.


### Runtime weight adjustment

`Server.UpdateWeight(ctx, weight)` re-advertises this instance with a new
weight without deregistering: consumers (loadbalance pools) pick the new value
up on their next discovery snapshot — one Watch push cycle. A weight of 0
drains the instance (no traffic, still registered), which is the standard
zero-downtime rotation step before shutdown. Operators can also edit the
registered weight directly at the registry; the effect is identical.
