# starter-registry-etcd

[English](README.md) | [中文](README_CN.md)

`starter-registry-etcd` registers the **current instance** into an etcd cluster —
the provider-side counterpart to Go-Spring's client-side discovery
(`stdlib/discovery`). It is the Go-Spring equivalent of Spring Cloud's
`ServiceRegistry` registration direction, backed by etcd leases.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-discovery-k8s](../starter-discovery-k8s) and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
etcd. RPC-framework provider registration stays framework-native and is out of
scope (see [starter/DESIGN §3](../DESIGN.md)).

## Archetype

Global / infrastructure (see [starter/DESIGN §2.4](../DESIGN.md)): it opens no
port. It exports a `gs.Server` so registration plugs into the server lifecycle —
the instance is published **once the application is ready** and deregistered
**as shutdown begins** (via `PreStop`), so discovery stops handing it out before
it actually stops serving. That ordering is what makes a rolling restart
lossless.

## Installation

```bash
go get go-spring.org/starter-registry-etcd
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-etcd"
```

### 2. Configure the etcd cluster and the instance

```properties
# etcd cluster (setting the endpoints activates the starter).
spring.registry.etcd.endpoints=127.0.0.1:2379
spring.registry.etcd.ttl=15s
spring.registry.etcd.key-prefix=/services/

# The instance to advertise (backend-agnostic).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is written under a **lease** and kept alive
by a background keep-alive; on shutdown the lease is revoked and the key removed.
If the process dies the lease expires after roughly `ttl` and etcd deletes the
key on its own. A client elsewhere resolves it by reading the same key prefix.

## Configuration

Connection, bound under `spring.registry.etcd`:

| Key | Default | Description |
| --- | --- | --- |
| `endpoints` | (required) | etcd cluster nodes; setting them activates the starter. |
| `username` | (empty) | Auth username; empty for anonymous clusters. |
| `password` | (empty) | Auth password. |
| `dial-timeout` | `5s` | Bounds the initial connect and the startup probe. |
| `ttl` | `15s` | Lease duration; the registrar keeps it alive while up. Rounded up to whole seconds. |
| `key-prefix` | `/services/` | Prepended to every key so apps can share a cluster. |
| `tls.*` | (off) | Optional client TLS (`enabled`, `cert-file`, `key-file`, `ca-cert-file`). |
| `name` | `default` | Name this registrar is published under in the `stdlib/discovery` registrar registry. |

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
`<key-prefix><service-name>/<id>`, so a discovery backend reading the same prefix
can reconstruct an `Endpoint`.

## How It Works

- During the container's bean-registration phase the starter builds an etcd
  `discovery.Registrar` and puts it in the `stdlib/discovery` registrar registry
  under `name`. It probes the cluster (a `Status` call) so an unreachable etcd
  fails startup. A company can register its own `Registrar` under a different
  name and point `spring.registry.backend` at it.
- The exported `gs.Server` resolves the backend by `backend`, waits for
  readiness, then `Register`s the instance: it grants a **lease**, writes the key
  under that lease, and keeps the lease alive with a background keep-alive.
- On shutdown `PreStop` deregisters the instance (stops the keep-alive and
  revokes the lease, deleting the key) before the pre-stop delay, so discovery
  removes it while in-flight requests keep being served. `Stop` deregisters again
  as an idempotent fallback. If the process crashes, the lease expires and etcd
  removes the key automatically.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts an etcd node, boots [example](example/main.go) (which
registers, reads the keys back, then SIGTERMs itself so the deregister path
runs), and asserts the instance appeared. It is skipped gracefully without
Docker.
