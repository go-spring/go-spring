# starter-registry-nacos

[English](README.md) | [中文](README_CN.md)

`starter-registry-nacos` registers the **current instance** into a Nacos naming
service — the provider-side counterpart to Go-Spring's client-side discovery
(`stdlib/discovery`). It is the Go-Spring equivalent of Spring Cloud Alibaba's
`nacos-discovery` registration direction, and the registrar counterpart to
[starter-config-nacos](../starter-config-nacos)'s config role (the two are
separate starters with separate config prefixes).

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-discovery-k8s](../starter-discovery-k8s) and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
Nacos. RPC-framework provider registration stays framework-native and is out of
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
go get go-spring.org/starter-registry-nacos
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-nacos"
```

### 2. Configure the Nacos server and the instance

```properties
# Nacos server (setting the address activates the starter).
spring.registry.nacos.server=127.0.0.1:8848
spring.registry.nacos.group=DEFAULT_GROUP
spring.registry.nacos.cluster=DEFAULT

# The instance to advertise (backend-agnostic).
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

That is all: on startup the instance is registered as an **ephemeral** Nacos
instance and kept alive by the SDK's own heartbeat; on shutdown it is
deregistered. A client elsewhere resolves it by service name through a
`discovery.Discovery` backend for the same registry.

## Configuration

Connection, bound under `spring.registry.nacos`:

| Key | Default | Description |
| --- | --- | --- |
| `server` | (required) | Nacos server `host:port`; setting it activates the starter. |
| `namespace` | (empty) | Namespace id to register into; empty uses `public`. |
| `group` | `DEFAULT_GROUP` | Service group; clients must resolve within the same group. |
| `cluster` | `DEFAULT` | Nacos cluster name the instance belongs to. |
| `username` | (empty) | Auth username; empty for anonymous clusters. |
| `password` | (empty) | Auth password. |
| `timeout-ms` | `5000` | Per-call timeout, including the startup probe. |
| `name` | `default` | Name this registrar is published under in the `stdlib/discovery` registrar registry. |

Instance, bound under `spring.registry` (backend-agnostic — switching registry
backends is a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Accepted for parity; unused by Nacos, which identifies an instance by `ip:port`. |
| `weight` | `0` | Load-balancing weight; `0` falls back to Nacos's default of `1`. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |
| `backend` | `default` | Which registrar backend to publish to, by its registry name. |

## How It Works

- During the container's bean-registration phase the starter builds a Nacos
  `discovery.Registrar` and puts it in the `stdlib/discovery` registrar registry
  under `name` — mirroring how `starter-discovery-k8s` registers discovery
  backends. It probes the server (a service listing) so an unreachable Nacos
  fails startup. A company can register its own `Registrar` under a different
  name and point `spring.registry.backend` at it.
- The exported `gs.Server` resolves the backend by `backend`, waits for
  readiness, then `Register`s the instance as **ephemeral**. The Nacos SDK keeps
  it alive with its own background heartbeat, and Nacos drops it automatically if
  the process dies without deregistering.
- On shutdown `PreStop` deregisters the instance before the pre-stop delay, so
  discovery removes it while in-flight requests keep being served. `Stop`
  deregisters again as an idempotent fallback.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts a Nacos standalone server, boots [example](example/main.go)
(which registers, reads the naming service back, then SIGTERMs itself so the
deregister path runs), and asserts the instance appeared. It is skipped
gracefully without Docker.
