# starter-registry-nacos

[English](README.md) | [中文](README_CN.md)

`starter-registry-nacos` adapts Nacos as a service registry: each configured
Nacos server becomes ONE backend bean that serves both halves of the naming
idiom — the write side (registering the **current instance** into Nacos) and
the read side (a `cloud/discovery` backend for resolving other instances). It
is the Go-Spring equivalent of Spring Cloud Alibaba's `nacos-discovery`, and
the registrar counterpart to [starter-config-nacos](../starter-config-nacos)'s
config role (the two are separate starters with separate config prefixes).

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-registry-k8s](../starter-registry-k8s) — the family's
discovery-only backend — and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
Nacos. RPC-framework provider registration stays framework-native and is out of
scope (see [starter/DESIGN §3](../../DESIGN.md)).

## Named Blocks, One Bean Per Center

Configuration is **named blocks**: one `spring.registry.nacos.<name>.*` block
per Nacos server. There is no default or unnamed block — the name is part of
the address. Each block becomes ONE backend bean named `nacos.<name>`
implementing BOTH `discovery.Registrar` (write) and `discovery.Discovery`
(read), sharing the block's client and namespace/group/cluster:

```properties
spring.registry.nacos.main.server=10.0.0.1:8848
spring.registry.nacos.dr.server=10.0.2.1:8848   # a second center, bean nacos.dr
```

Two blocks = one registration fanned out to two Nacos servers (Dubbo-style
default multi-write); blocks across backends (nacos + zookeeper, ...) mix
freely in one app.

## Registration Is Owned by starter-registry

Importing this starter imports the registration core
([starter-registry](../starter-registry)) transitively: a single `registryServer`
`gs.Server` collects ALL registrar beans — across ALL backends — and registers
the instance into EVERY configured center once the app is ready, deregisters
from all of them as shutdown begins (via `PreStop`), and broadcasts
`UpdateWeight`. Any registration failure fails startup, so the centers can
never split consumers' views.

Registration intent is `spring.registry.service-name` (plus
`spring.registry.addr`), shared by all centers. A pure consumer app configures
connection blocks only and registers nothing.

## Installation

```bash
go get go-spring.org/starter-registry-nacos
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-nacos"
```

### 2. Configure the Nacos server and the instance

```properties
# One Nacos server = one named block (setting server activates the block).
spring.registry.nacos.main.server=127.0.0.1:8848
spring.registry.nacos.main.group=DEFAULT_GROUP
spring.registry.nacos.main.cluster=DEFAULT

# The instance to advertise (backend-agnostic, shared by every center).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is registered as an **ephemeral** Nacos
instance in every configured block and kept alive by the SDK's own heartbeat;
on shutdown it is deregistered everywhere.

## Discovery

Consumers cite the **bean name** of a block — no discovery-specific keys exist.
The bean is lazy on the read side: an app that never cites it never pays for
the discovery half.

```properties
spring.registry.nacos.main.server=127.0.0.1:8848
spring.http-client.backends.users.discovery=nacos.main
```

Read and write share the block's namespace/group/cluster, so they can never
diverge.

## Configuration

Connection, bound per block under `spring.registry.nacos.<name>`:

| Key | Default | Description |
| --- | --- | --- |
| `server` | (required) | Nacos server `host:port`; setting it activates the block. |
| `namespace` | (empty) | Namespace id to register into; empty uses `public`. |
| `group` | `DEFAULT_GROUP` | Service group; clients must resolve within the same group. |
| `cluster` | `DEFAULT` | Nacos cluster name the instance belongs to. |
| `username` | (empty) | Auth username; empty for anonymous clusters. |
| `password` | (empty) | Auth password. |
| `timeout-ms` | `5000` | Per-call timeout, including the startup probe. |

Instance, bound under `spring.registry` (backend-agnostic — switching registry
backends is a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (empty) | Logical name to publish; setting it is the registration intent. |
| `addr` | (required when registering) | Connectable `host:port` advertised to clients. Nacos identifies instances by ip:port, so no id key applies. |
| `weight` | `100` | Load-balancing weight; a negative value is normalized to `1` at write time, `0` drains. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |

## How It Works

- Each block's backend bean is constructed eagerly: it builds the naming
  client and probes the server (a service listing), so a misconfigured or
  unreachable Nacos fails startup once per block.
- The `registryServer` from the starter-registry core collects every backend's
  registrar, waits for readiness, then `Register`s the instance as
  **ephemeral** into each center. The Nacos SDK keeps it alive with its own
  background heartbeat, and Nacos drops it automatically if the process dies
  without deregistering.
- On shutdown `PreStop` deregisters from every center before the pre-stop
  delay, so discovery removes the instance while in-flight requests keep being
  served. `Stop` deregisters again as an idempotent fallback.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts a Nacos standalone server, boots [example](example/example.go)
(which registers, reads the naming service back, then SIGTERMs itself so the
deregister path runs), and asserts the instance appeared. It is skipped
gracefully without Docker.

### Runtime weight adjustment

`Server.UpdateWeight(ctx, weight)` (on the `registryServer` bean) re-advertises
this instance with a new weight in EVERY center without deregistering:
consumers (loadbalance pools) pick the new value up on their next discovery
snapshot. A weight of 0 drains the instance (no traffic, still registered),
which is the standard zero-downtime rotation step before shutdown. Operators
can also edit the registered weight directly at a registry; the effect is
identical there.

### Log tag

Runtime logs from this module carry the tag `_app_registry_nacos` (nacos registry). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.registry_nacos.type=Logger
logger.registry_nacos.level=WARN
logger.registry_nacos.tag=_app_registry_nacos
```
