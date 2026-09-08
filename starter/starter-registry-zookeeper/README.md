# starter-registry-zookeeper

[English](README.md) | [中文](README_CN.md)

`starter-registry-zookeeper` registers the **current instance** into a ZooKeeper
ensemble — the provider-side counterpart to Go-Spring's client-side discovery
(`cloud/discovery`) — **and** provides the consumer-side ZooKeeper discovery
backend, so one starter serves both halves of the naming idiom. It is the
Go-Spring equivalent of Spring Cloud's `ServiceRegistry` + `DiscoveryClient`,
backed by ephemeral znodes.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-discovery-k8s](../starter-discovery-k8s) and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
ZooKeeper. RPC-framework provider registration stays framework-native and is out
of scope (see [starter/DESIGN §3](../../DESIGN.md)).

## Archetype

Global / infrastructure (see [starter/DESIGN §2.4](../../DESIGN.md)): it opens no
port. It exports a `gs.Server` so registration plugs into the server lifecycle —
the instance is published **once the application is ready** and deregistered
**as shutdown begins** (via `PreStop`), so discovery stops handing it out before
it actually stops serving. That ordering is what makes a rolling restart
lossless.

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
# ZooKeeper ensemble (setting the servers activates the starter).
spring.registry.zookeeper.servers=127.0.0.1:2181
spring.registry.zookeeper.session-timeout=10s
spring.registry.zookeeper.base-path=/services

# The instance to advertise (backend-agnostic).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is written as an **ephemeral znode** whose
lifetime is tied to the client session; on shutdown the node is deleted. If the
process dies the session expires and ZooKeeper removes the node on its own. A
client elsewhere resolves it by listing the same base path.

## Configuration

### Registering (this instance)

Connection, bound under `spring.registry.zookeeper`:

| Key | Default | Description |
| --- | --- | --- |
| `servers` | (required) | Ensemble members; setting them activates the starter. |
| `session-timeout` | `10s` | Session timeout; ephemeral nodes survive as long as the session. |
| `base-path` | `/services` | Persistent parent znode under which service directories are created. |
| `username` | (empty) | Digest-auth username; enables auth when set. |
| `password` | (empty) | Digest-auth password. |
| `discovery-name` | `zookeeper` | Derives a discovery backend bean for this same ensemble under that label — one config block serves both halves (shared connection). Empty disables the derived backend. |

Instance, bound under `spring.registry` (backend-agnostic — switching registry
backends is a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `0` | Load-balancing weight stored with the instance. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |

The instance is stored as JSON (`service_name`, `addr`, `weight`, `metadata`) at
`<base-path>/<service-name>/<id>`, so a discovery backend listing the same base
path can reconstruct an `Endpoint`.

### Discovering (other instances)

The consumer half is a `cloud/discovery` backend bean. Two ways to get one:

**Derived from the center (dual-role apps).** Setting
`spring.registry.zookeeper` alone derives a backend bean labeled
`discovery-name` (default `zookeeper`) on the same shared connection — one
ensemble config, both halves:

```properties
spring.registry.zookeeper.servers=127.0.0.1:2181
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# clients then cite: <client>.discovery=zookeeper
```

**Standalone blocks (other ensembles / pure consumers).** One block per
ZooKeeper ensemble under `spring.discovery.zookeeper.<name>`; a client starter
cites the name:

```properties
spring.discovery.zookeeper.prod.servers=127.0.0.1:2181
spring.discovery.zookeeper.prod.base-path=/services
```

| Key | Default | Description |
| --- | --- | --- |
| `servers` | (empty) | Ensemble members; empty INHERITS the `spring.registry.zookeeper` center connection (shared session). |
| `session-timeout` | `10s` | Bounds the initial connect and the startup probe of a standalone block. |
| `base-path` | `/services` | Must match the registering applications' base path. |
| `username` / `password` | (empty) | Digest-auth credentials; leave empty for an open ensemble. |

`base-path` must match the registrars' base path or nothing resolves. Health is
derived from znode liveness: an instance is an ephemeral znode that ZooKeeper
removes when the registrar's session dies, so every node found is a live
instance — no probing protocol is needed. An optional `scheme` metadata key on
the registered instance carries transport selection, mirroring the etcd adapter.

## How It Works

- During bean construction the starter builds the shared center connection for
  ${spring.registry.zookeeper} (probing the ensemble — an `Exists` call blocks
  until the session connects — so an unreachable ZooKeeper fails startup),
  builds the ZooKeeper registrar on top of it, and wires it into the exported
  `gs.Server`.
- The exported `gs.Server` waits for readiness, then `Register`s the instance:
  it creates the persistent parent directories on demand and writes the
  instance as an **ephemeral** leaf znode.
- On shutdown `PreStop` deregisters the instance (deletes the znode) before the
  pre-stop delay, so discovery removes it while in-flight requests keep being
  served. `Stop` deregisters again as an idempotent fallback. If the process
  crashes, the session expires and ZooKeeper removes the node automatically.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts a ZooKeeper node, boots [example](example/example.go) (which
registers, lists the znodes back, then SIGTERMs itself so the deregister path
runs), and asserts the instance appeared. It is skipped gracefully without
Docker.


### Runtime weight adjustment

`Server.UpdateWeight(ctx, weight)` re-advertises this instance with a new
weight without deregistering: consumers (loadbalance pools) pick the new value
up on their next discovery snapshot — one Watch push cycle. A weight of 0
drains the instance (no traffic, still registered), which is the standard
zero-downtime rotation step before shutdown. Operators can also edit the
registered weight directly at the registry; the effect is identical.
