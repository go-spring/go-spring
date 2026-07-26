# starter-registry-zookeeper

[English](README.md) | [中文](README_CN.md)

`starter-registry-zookeeper` registers the **current instance** into a ZooKeeper
ensemble — the provider-side counterpart to Go-Spring's client-side discovery
(`spring/discovery`). It is the Go-Spring equivalent of Spring Cloud's
`ServiceRegistry` registration direction, backed by ephemeral znodes.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-discovery-k8s](../starter-discovery-k8s) and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
ZooKeeper. RPC-framework provider registration stays framework-native and is out
of scope (see [starter/DESIGN §3](../DESIGN.md)).

## Archetype

Global / infrastructure (see [starter/DESIGN §2.4](../DESIGN.md)): it opens no
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

Connection, bound under `spring.registry.zookeeper`:

| Key | Default | Description |
| --- | --- | --- |
| `servers` | (required) | Ensemble members; setting them activates the starter. |
| `session-timeout` | `10s` | Session timeout; ephemeral nodes survive as long as the session. |
| `base-path` | `/services` | Persistent parent znode under which service directories are created. |
| `username` | (empty) | Digest-auth username; enables auth when set. |
| `password` | (empty) | Digest-auth password. |
| `name` | `default` | Name this registrar is published under in the `spring/discovery` registrar registry. |

Instance, bound under `spring.registry` (backend-agnostic — switching registry
backends is a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `0` | Load-balancing weight stored with the instance. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |
| `backend` | `default` | Which registrar backend to publish to, by its registry name. |

The instance is stored as JSON (`service_name`, `addr`, `weight`, `metadata`) at
`<base-path>/<service-name>/<id>`, so a discovery backend listing the same base
path can reconstruct an `Endpoint`.

## How It Works

- During the container's bean-registration phase the starter connects to the
  ensemble, builds a ZooKeeper `discovery.Registrar`, and puts it in the
  `spring/discovery` registrar registry under `name`. It probes the ensemble (an
  `Exists` call blocks until the session connects) so an unreachable ZooKeeper
  fails startup. A company can register its own `Registrar` under a different
  name and point `spring.registry.backend` at it.
- The exported `gs.Server` resolves the backend by `backend`, waits for
  readiness, then `Register`s the instance: it creates the persistent parent
  directories on demand and writes the instance as an **ephemeral** leaf znode.
- On shutdown `PreStop` deregisters the instance (deletes the znode) before the
  pre-stop delay, so discovery removes it while in-flight requests keep being
  served. `Stop` deregisters again as an idempotent fallback. If the process
  crashes, the session expires and ZooKeeper removes the node automatically.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts a ZooKeeper node, boots [example](example/main.go) (which
registers, lists the znodes back, then SIGTERMs itself so the deregister path
runs), and asserts the instance appeared. It is skipped gracefully without
Docker.
