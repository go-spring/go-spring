# starter-registry-zookeeper

[English](README.md) | [中文](README_CN.md)

`starter-registry-zookeeper` adapts ZooKeeper as a service registry: each configured
ensemble becomes ONE backend bean that serves both halves of the naming idiom — the write
side (registering the **current instance** as an ephemeral znode) and the read side (a
`cloud/discovery` backend for resolving other instances). It is the Go-Spring equivalent of
Spring Cloud's `ServiceRegistry` + `DiscoveryClient`, backed by ephemeral znodes.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not register
instances for you. In **pure Kubernetes** you would not use this starter at all: the
platform already registers every Pod behind a Service, so you discover peers with
[starter-registry-k8s](../starter-registry-k8s) — the family's
discovery-only backend — and register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to ZooKeeper.
RPC-framework provider registration stays framework-native and is out of scope (see
[starter/DESIGN §3](../../DESIGN.md)).

## Named Blocks, One Bean Per Center

Configuration is **named blocks**: one `spring.registry.zookeeper.<name>.*` block per
ZooKeeper ensemble. There is no default or unnamed block — the name is part of the address.
Each block becomes ONE backend bean named `zookeeper.<name>` implementing BOTH
`discovery.Registrar` (write) and `discovery.Discovery` (read), sharing the block's session
and base-path:

```properties
spring.registry.zookeeper.bz.servers=10.1.0.1:2181,10.1.0.2:2181
spring.registry.zookeeper.dr.servers=10.2.0.1:2181   # a second ensemble, bean zookeeper.dr
```

Two blocks = one registration fanned out to two ensembles; blocks across backends
(zookeeper + nacos, ...) mix freely in one app.

## Registration Is Owned by starter-registry

Importing this starter imports the registration core
([starter-registry](../starter-registry)) transitively: a single `registryServer` `gs.Server`
collects ALL registrar beans — across ALL backends — and registers the instance into EVERY
configured center once the app is ready, deregisters from all of them as shutdown begins
(via `PreStop`), and broadcasts `UpdateWeight`. Any registration failure fails startup, so
the centers can never split consumers' views.

Registration intent is `spring.registry.service-name` (plus `spring.registry.addr`), shared
by all centers. A pure consumer app configures connection blocks only and registers nothing.

## Installation

```bash
go get go-spring.org/starter-registry-zookeeper
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-zookeeper"
```

### 2. Configure the ZooKeeper ensemble and the instance

```properties
# One ensemble = one named block (setting servers activates the block).
spring.registry.zookeeper.main.servers=127.0.0.1:2181
spring.registry.zookeeper.main.session-timeout=10s
spring.registry.zookeeper.main.base-path=/services

# The instance to advertise (backend-agnostic, shared by every center).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is written into every configured block as an
**ephemeral znode** whose lifetime is tied to that block's session; on shutdown the nodes
are deleted. If the process dies the sessions expire and ZooKeeper removes the nodes on its
own.

## Discovery

Consumers cite the **bean name** of a block — no discovery-specific keys exist. The bean is
lazy on the read side: an app that never cites it never pays for the discovery half.

```properties
spring.registry.zookeeper.main.servers=127.0.0.1:2181
spring.http-client.backends.users.discovery=zookeeper.main
```

Read and write share the block's base-path, so they can never diverge. Health is derived
from znode liveness: an instance is an ephemeral znode that ZooKeeper removes when the
registrar's session dies, so every node found is a live instance — no probing protocol is
needed. An optional `scheme` metadata key on the registered instance carries transport
selection.

## Configuration

Connection, bound per block under `spring.registry.zookeeper.<name>`:

| Key | Default | Description |
| --- | --- | --- |
| `servers` | (required) | Ensemble members; setting them activates the block. |
| `session-timeout` | `10s` | Session timeout; ephemeral nodes survive as long as the session. |
| `base-path` | `/services` | Persistent parent znode under which service directories are created. |
| `username` | (empty) | Digest-auth username; enables auth when set. |
| `password` | (empty) | Digest-auth password. |

Instance, bound under `spring.registry` (backend-agnostic — switching registry backends is
a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (empty) | Logical name to publish; setting it is the registration intent. |
| `addr` | (required when registering) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `100` | Load-balancing weight stored with the instance. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |

The instance is stored as JSON (`service_name`, `addr`, `weight`, `metadata`) at
`<base-path>/<service-name>/<id>`, so a discovery backend listing the same base path can
reconstruct an `Endpoint`.

## How It Works

- Each block's backend bean is constructed eagerly: it dials the ensemble (probing it — an
  `Exists` call blocks until the session connects — so an unreachable ZooKeeper fails
  startup) and owns the block's session, both halves sharing it.
- The `registryServer` from the starter-registry core collects every backend's registrar,
  waits for readiness, then `Register`s the instance into each center: it creates the
  persistent parent directories on demand and writes the instance as an **ephemeral** leaf
  znode.
- On shutdown `PreStop` deregisters from every center (deletes the znodes) before the
  pre-stop delay, so discovery removes the instance while in-flight requests keep being
  served. `Stop` deregisters again as an idempotent fallback. If the process crashes, the
  sessions expire and ZooKeeper removes the nodes automatically.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is available —
starts a ZooKeeper node, boots [example](example/example.go) (which registers, lists the
znodes back, then SIGTERMs itself so the deregister path runs), and asserts the instance
appeared. It is skipped gracefully without Docker.

### Runtime weight adjustment

`Server.UpdateWeight(ctx, weight)` (on the `registryServer` bean) re-advertises this
instance with a new weight in EVERY center without deregistering: consumers (loadbalance
pools) pick the new value up on their next discovery snapshot — one watch push cycle. A
weight of 0 drains the instance (no traffic, still registered), which is the standard
zero-downtime rotation step before shutdown. Operators can also edit the registered weight
directly at a registry; the effect is identical there.
