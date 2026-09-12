# starter-registry-etcd

[English](README.md) | [中文](README_CN.md)

`starter-registry-etcd` registers the **current instance** into an etcd cluster —
the provider-side counterpart to Go-Spring's client-side discovery
(`cloud/discovery`) — **and** provides the consumer-side etcd discovery
backend, so one starter serves both halves of the naming idiom. It is the
Go-Spring equivalent of Spring Cloud's `ServiceRegistry` + `DiscoveryClient`,
backed by etcd leases.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-registry-k8s](../starter-registry-k8s) — the family's
discovery-only backend — and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
etcd. RPC-framework provider registration stays framework-native and is out of
scope (see [starter/DESIGN §3](../DESIGN.md)).

## Model

Configuration is **named blocks**: each
`spring.registry.etcd.<name>.*` block describes ONE etcd cluster and becomes
ONE backend bean named `etcd.<name>` that implements BOTH the write side
(`discovery.Registrar`) and the read side (`discovery.Discovery`), sharing the
block's client and key prefix. There is no default/unnamed block.

Registration itself is owned by the [starter-registry](../starter-registry)
core (imported transitively): its single `registryServer` `gs.Server` collects
the registrar beans of **every** configured center — across backends — and
registers the instance into all of them once the app is ready, deregisters from
all on shutdown, and broadcasts `UpdateWeight`. Configure two blocks (etcd +
etcd, or etcd + zookeeper, ...) plus `spring.registry.service-name` and you get
dual/multi registration with zero extra config — the Dubbo-style default.

The starter opens no port: the exported `gs.Server` exists purely so
registration plugs into the server lifecycle — the instance is published **once
the application is ready** and deregistered **as shutdown begins** (via
`PreStop`), so discovery stops handing it out before it actually stops serving.
That ordering is what makes a rolling restart lossless.

## Installation

```bash
go get go-spring.org/starter-registry-etcd
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-etcd"
```

### 2. Configure the etcd clusters and the instance

```properties
# One named block per etcd cluster; "main" is the block name (your choice).
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.registry.etcd.main.ttl=15s
spring.registry.etcd.main.key-prefix=/services/

# A second block = a second center = dual registration (zero extra config).
# spring.registry.etcd.dr.endpoints=10.9.0.1:2379

# The instance to advertise (backend-agnostic, shared by every center).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is written into **every** configured
cluster under a **lease** and kept alive by a background keep-alive; on shutdown
the leases are revoked and the keys removed. If the process dies the leases
expire after roughly `ttl` and etcd deletes the keys on its own. A client
elsewhere resolves it by reading the same key prefix.

## Configuration

### Registering (this instance)

Connection, bound per block under `spring.registry.etcd.<name>`:

| Key | Default | Description |
| --- | --- | --- |
| `endpoints` | (required) | etcd cluster nodes; setting a block activates it. |
| `username` | (empty) | Auth username; empty for anonymous clusters. |
| `password` | (empty) | Auth password. |
| `dial-timeout` | `5s` | Bounds the initial connect and the startup probe. |
| `ttl` | `15s` | Lease duration; the registrar keeps it alive while up. Rounded up to whole seconds. |
| `key-prefix` | `/services/` | Prepended to every key so apps can share a cluster. |
| `tls.*` | (off) | Optional client TLS (`enabled`, `cert-file`, `key-file`, `ca-file`). |

Each block becomes ONE backend bean `etcd.<name>` — one client, one startup
probe, one lifecycle; both halves of the naming idiom share them, so read and
write can never diverge. **Registration activates only when `service-name` is
set**; a consumer-only app omits that key and registers nothing.

Instance, bound under `spring.registry` (describes the instance itself, shared
by every configured center, independent of the registry backend):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `100` | Load-balancing weight stored with the instance. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |

The instance is stored as JSON (`service_name`, `addr`, `weight`, `metadata`) at
`<key-prefix><service-name>/<id>`, so a discovery backend reading the same prefix
can reconstruct an `Endpoint`.

### Discovering (other instances)

Discovery needs **no configuration of its own**: each block's bean IS a
`cloud/discovery` backend under the bean name `etcd.<name>`. Client starters
cite that name:

```properties
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.registry.etcd.dr.endpoints=10.9.0.1:2379
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# discovery: cite the block's bean name, nothing to configure
spring.http-client.backends.users.discovery=etcd.main
```

The discovery bean is lazy — a pure provider that never resolves anything
never pays for it. A **pure consumer** configures ONLY connection blocks and
registers nothing: registration activates only when
`spring.registry.service-name` is set.

```properties
# consumer-only app: connection blocks, no service-name/addr, registers nothing
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.http-client.backends.users.discovery=etcd.main
```

Multi-cluster discovery is now just multiple blocks: discover from `etcd.dr`
while registering into `etcd.main` and `etcd.dr` — or from a block you never
register into at all.

Health is
derived from key liveness: an instance key only exists while its lease is alive,
so every key found is a live instance — no probing protocol is needed. An
optional `scheme` metadata key on the registered instance carries transport
selection, mirroring the nacos adapter.

## How It Works

- For each block the starter builds ONE backend bean (`etcd.<name>`) holding
  the cluster's client and both halves of the naming idiom. It probes the
  cluster (a `Status` call) so an unreachable etcd fails startup, once per
  block.
- The `registryServer` from the [starter-registry](../starter-registry) core
  (transitively imported) collects every backend's registrar — across all
  backends — and waits for readiness, then `Register`s the instance into every
  center: it grants a **lease** per center, writes the key under that lease,
  and keeps the lease alive with a background keep-alive. If a lease dies
  server-side, the registrar re-registers with backoff — the entry comes back
  without operator action.
- On shutdown `PreStop` deregisters the instance from every center (stops the
  keep-alives and revokes the leases, deleting the keys) before the pre-stop
  delay, so discovery removes it while in-flight requests keep being served.
  `Stop` deregisters again as an idempotent fallback. If the process crashes,
  the leases expire and etcd removes the keys automatically.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts an etcd node, boots [example](example/example.go) (which
registers, reads the keys back, then SIGTERMs itself so the deregister path
runs), and asserts the instance appeared. It is skipped gracefully without
Docker.


### Runtime weight adjustment

`Server.UpdateWeight(ctx, weight)` re-advertises this instance with a new
weight without deregistering: consumers (loadbalance pools) pick the new value
up on their next discovery snapshot — one refresh cycle. A weight of 0
drains the instance (no traffic, still registered), which is the standard
zero-downtime rotation step before shutdown. Operators can also edit the
registered weight directly at the registry; the effect is identical.
### Log tag

Runtime logs from this module carry the tag `_app_registry_etcd` (etcd registry). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.registry_etcd.type=Logger
logger.registry_etcd.level=WARN
logger.registry_etcd.tag=_app_registry_etcd
```
