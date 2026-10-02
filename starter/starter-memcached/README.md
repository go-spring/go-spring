# starter-memcached

[English](README.md) | [中文](README_CN.md)

`starter-memcached` provides a Memcached client wrapper based on
[gomemcache](https://github.com/bradfitz/gomemcache) for Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-memcached
```

## Quick Start

### 1. Import the `starter-memcached` Package

```go
import _ "go-spring.org/starter-memcached"
```

### 2. Configure the Memcached Instance

Add Memcached configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.memcached.instances.main.servers=127.0.0.1:11211
```

### 3. Inject the Memcached Instance

```go
import StarterMemcached "go-spring.org/starter-memcached"

type Service struct {
    Memcached *StarterMemcached.Client `autowire:""`
}
```

### 4. Use the Memcached Instance

```go
err := s.Memcached.Set(ctx, &memcache.Item{Key: "key", Value: []byte("value")})
item, err := s.Memcached.Get(ctx, "key")
```

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three core Memcached operations:

* **String SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **INCR counter** — seed a key with `Set(...)` and then atomically increment it via `Increment(...)`.
* **DELETE + cache miss** — remove a key with `Delete(...)` and confirm a subsequent `Get(...)` returns `ErrCacheMiss`.

## Advanced Features

* **Supports multiple Memcached instances**: you can define multiple instances in the configuration file and reference them by name.
* **Support Memcached extensions**: implement the `Driver` interface to extend Memcached functionality — see
  the example implementation `AnotherMemcachedDriver`. When several Driver beans coexist, an entry selects
  one by name: `spring.memcached.instances.<name>.driver = <bean-name>` (empty = inject the single Driver bean by
  type; naming a missing bean fails startup).
* **Startup connection validation (opt-in)**: set `ping=true` and after building the client the starter runs
  `HealthCheck` (a `Ping` loop) against every configured server, so an unreachable server fails the boot instead of
  the first request. Off by default: a backend that is not up yet must not block startup.
* **Service discovery**: set `service-name` (and `discovery` to name the registered backend; there is no default backend)
  instead of `servers`; the starter resolves the server list once through the registered `discovery.Discovery` backend
  at startup and shards keys across it. Because gomemcache hashes keys onto a fixed server set chosen at client creation,
  the resolve is **one-shot at boot** (fail-fast on empty/failed resolve) rather than a live watch — a changing cluster
  membership requires a restart. For a topology that grows and shrinks dynamically, put a
serverless/proxy-style endpoint (a single stable address) in `servers` and let the proxy own
membership. See [discovery.go](example/discovery.go) for a backend example.
* **Health check / readiness**: `HealthCheck` probes all servers and is the readiness signal (the autowired
  `health.Indicator` delegates to it) — call it straight off the autowired client.
* **Connection pool / timeouts**: `timeout` and `max-idle-conns` map to the client's per-server socket timeout and idle
  connection pool; both fall back to the driver defaults (100ms / 2) when left at 0.
* **Authentication**: the `bradfitz/gomemcache` driver does not implement SASL, so no auth fields are exposed. Restrict
  access at the network layer (VPC/security group) instead.
* **Shared cache backend**: `AsCache(client, codec)` adapts the client to `cloud/cache.Cache` as the shared (far) level
  of a multi-level cache. Values are serialized with the codec (nil defaults to JSON); a cache miss maps to a plain miss.

## Observability

`bradfitz/gomemcache` ships no official OpenTelemetry instrumentation. The starter therefore declares what each
operation is (`observe.go`): the command name, the `db.system`/`db.operation` labels, and the key as `db.statement`
span/log detail. The signals themselves are emitted by the resilience layer — the one point on the executor chain that
sees a whole call, retries included — so beside the call-level `db.client.operation.duration` histogram every call
also reports an attempt-level `db.client.attempt.duration` one, and an access log tagged `_app_memcached_access`. All
of it is a no-op unless `starter-otel` installs providers.
