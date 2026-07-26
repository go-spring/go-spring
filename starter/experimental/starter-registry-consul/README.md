# starter-registry-consul

[English](README.md) | [中文](README_CN.md)

`starter-registry-consul` registers the **current instance** into a Consul
service registry — the provider-side counterpart to Go-Spring's client-side
discovery (`spring/discovery`). It is the Go-Spring equivalent of Spring Cloud's
`ServiceRegistry` / `@EnableDiscoveryClient` registration direction.

Use it for **VM / bare-metal / hybrid** deployments where the platform does not
register instances for you. In **pure Kubernetes** you would not use this
starter at all: the platform already registers every Pod behind a Service, so
you discover peers with [starter-discovery-k8s](../starter-discovery-k8s) and
register nothing.

This starter publishes a **plain instance** (any transport — HTTP, gRPC, ...) to
Consul. RPC-framework provider registration stays framework-native and is out of
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
heartbeat; on shutdown it is deregistered. A client elsewhere resolves it by
service name through a `discovery.Discovery` backend for the same registry.

## Configuration

Connection, bound under `spring.registry.consul`:

| Key | Default | Description |
| --- | --- | --- |
| `address` | (required) | Consul HTTP API address; setting it activates the starter. |
| `scheme` | `http` | `http` or `https`. |
| `datacenter` | (empty) | Datacenter to register into; empty uses the agent's. |
| `token` | (empty) | ACL token. |
| `namespace` | (empty) | Consul Enterprise namespace. |
| `name` | `default` | Name this registrar is published under in the `spring/discovery` registrar registry. |
| `ttl` | `15s` | TTL health check; the starter heartbeats at half this interval. |
| `deregister-critical-after` | `1m` | Consul drops the instance if its check stays critical this long (e.g. after a crash). |

Instance, bound under `spring.registry` (backend-agnostic — switching registry
backends is a blank-import swap, not a config migration):

| Key | Default | Description |
| --- | --- | --- |
| `service-name` | (required) | Logical name to publish; the same name clients resolve. |
| `addr` | (required) | Connectable `host:port` advertised to clients. |
| `id` | (empty) | Instance id override; empty derives a stable one from `service-name` + `addr`. |
| `weight` | `0` | Load-balancing weight; `0` uses Consul's default. |
| `metadata.*` | (none) | Arbitrary key/value attributes stored with the instance. |
| `backend` | `default` | Which registrar backend to publish to, by its registry name. |

## How It Works

- During the container's bean-registration phase the starter builds a Consul
  `discovery.Registrar` and puts it in the `spring/discovery` registrar registry
  under `name` — mirroring how `starter-discovery-k8s` registers discovery
  backends. A company can register its own `Registrar` under a different name and
  point `spring.registry.backend` at it.
- The exported `gs.Server` resolves the backend by `backend`, waits for
  readiness, then `Register`s the instance with a Consul **TTL health check**. It
  passes the check immediately and keeps it passing on a background heartbeat at
  half the TTL.
- On shutdown `PreStop` deregisters the instance (stopping the heartbeat and
  removing it from Consul) before the pre-stop delay, so discovery removes it
  while in-flight requests keep being served. `Stop` deregisters again as an
  idempotent fallback.

## Smoke Test

[example/check.sh](example/check.sh) runs the unit tests, then — if Docker is
available — starts a Consul dev agent, boots [example](example/main.go) (which
registers, reads the catalog back, then SIGTERMs itself so the deregister path
runs), and asserts the instance appeared. It is skipped gracefully without
Docker.
