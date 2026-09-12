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
you discover peers with [starter-registry-k8s](../starter-registry-k8s) — the family's
discovery-only backend — and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
Consul. RPC-framework provider registration stays framework-native and is out of
scope (see [starter/DESIGN §3](../../DESIGN.md)).

## Model

Configuration is **named blocks**: each
`spring.registry.consul.<name>.*` block describes ONE Consul agent/cluster and
becomes ONE backend bean named `consul.<name>` that implements BOTH the write
side (`discovery.Registrar`) and the read side (`discovery.Discovery`), sharing
the block's client. There is no default/unnamed block.

Registration itself is owned by the [starter-registry](../starter-registry)
core (imported transitively): its single `registryServer` `gs.Server` collects
the registrar beans of **every** configured center — across backends — and
registers the instance into all of them once the app is ready, deregisters from
all on shutdown, and broadcasts `UpdateWeight`. Configure two blocks (consul +
consul, or consul + etcd, ...) plus `spring.registry.service-name` and you get
dual/multi registration with zero extra config.

The starter opens no port: the exported `gs.Server` exists purely so
registration plugs into the server lifecycle — the instance is published **once
the application is ready** and deregistered **as shutdown begins** (via
`PreStop`), so discovery stops handing it out before it actually stops serving.
That ordering is what makes a rolling restart lossless.

## Installation

```bash
go get go-spring.org/starter-registry-consul
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-consul"
```

### 2. Configure the Consul agents and the instance

```properties
# One named block per Consul agent; "main" is the block name (your choice).
spring.registry.consul.main.address=127.0.0.1:8500
spring.registry.consul.main.ttl=15s
spring.registry.consul.main.deregister-critical-after=1m

# A second block = a second center = dual registration (zero extra config).
# spring.registry.consul.dr.address=10.9.0.1:8500

# The instance to advertise (backend-agnostic, shared by every center).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is registered into **every** configured
agent and kept alive by a TTL heartbeat; on shutdown it is deregistered
everywhere. Each block's bean is also a discovery backend named `consul.<name>`
(see below), so a client elsewhere — or this process itself — resolves it by
service name through the shared connection.

## Configuration

### Registering (this instance)

Connection, bound per block under `spring.registry.consul.<name>`:

| Key | Default | Description |
| --- | --- | --- |
| `address` | (required) | Consul HTTP API address; setting a block activates it. |
| `scheme` | `http` | `http` or `https`. |
| `datacenter` | (empty) | Datacenter to register into; empty uses the agent's. |
| `token` | (empty) | ACL token. |
| `namespace` | (empty) | Consul Enterprise namespace. |
| `ttl` | `15s` | TTL health check; the starter heartbeats at half this interval. |
| `deregister-critical-after` | `1m` | Consul drops the instance if its check stays critical this long (e.g. after a crash). |

Each block becomes ONE backend bean `consul.<name>` — one client, one startup
probe, one lifecycle; both halves of the naming idiom share them, so read and
write can never diverge. **Registration activates only when `service-name` is
set**; a consumer-only app omits that key and registers nothing.

Instance, bound under `spring.registry` (backend-agnostic, shared by every
center — switching registry backends is a blank-import swap, not a config
migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `100` | Load-balancing weight stored with the instance. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |

### Discovering (other instances)

Discovery needs **no configuration of its own**: each block's bean IS a
`cloud/discovery` backend under the bean name `consul.<name>`. Client starters
cite that name:

```properties
spring.registry.consul.main.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# discovery: cite the block's bean name, nothing to configure
spring.http-client.backends.users.discovery=consul.main
```

The discovery bean is lazy — a pure provider that never resolves anything
never pays for it. A **pure consumer** configures ONLY connection blocks and
registers nothing: registration activates only when
`spring.registry.service-name` is set.

```properties
# consumer-only app: connection blocks, no service-name/addr, registers nothing
spring.registry.consul.main.address=127.0.0.1:8500
spring.http-client.backends.users.discovery=consul.main
```

Multi-agent discovery is now just multiple blocks: discover from `consul.dr`
while registering into `consul.main` and `consul.dr` — or from a block you
never register into at all.

Resolve asks Consul for **passing** instances only, so unhealthy instances
never enter the snapshot. Freshness is a background Consul blocking query
(index-based long poll) per resolved service: the first Resolve seeds a cache,
the blocking query keeps it current, later Resolves are in-memory reads. The
advertised passing weight becomes the endpoint weight; an optional `scheme`
meta key carries transport selection, mirroring the etcd/nacos adapters.

## How It Works

- For each block the starter builds ONE backend bean (`consul.<name>`) holding
  the agent's client and both halves of the naming idiom. It probes the agent
  (`Catalog().Services`) so an unreachable one fails startup, once per block.
- The `registryServer` from the [starter-registry](../starter-registry) core
  (transitively imported) collects every backend's registrar — across all
  backends — and waits for readiness, then `Register`s the instance into every
  center with a Consul **TTL health check**. It passes the check immediately
  and keeps it passing on a background heartbeat at half the TTL.
- On shutdown `PreStop` deregisters the instance from every center (stopping
  the heartbeats and removing them from Consul) before the pre-stop delay, so
  discovery removes it while in-flight requests keep being served. `Stop`
  deregisters again as an idempotent fallback.

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
