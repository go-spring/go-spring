# starter-memcached

[English](README.md) | [中文](README_CN.md)

`starter-memcached` provides a Memcached client wrapper based on
[gomemcache](https://github.com/bradfitz/gomemcache), making it easy to
integrate and use Memcached in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-memcached
```

## Quick Start

### 1. Import the `starter-memcached` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-memcached"
```

### 2. Configure the Memcached Instance

Add Memcached configuration in your project’s [configuration file](example/conf/app.properties), for example:

```properties
spring.memcached.main.servers=127.0.0.1:11211
```

### 3. Inject the Memcached Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/bradfitz/gomemcache/memcache"

type Service struct {
    Memcached *memcache.Client `autowire:""`
}
```

### 4. Use the Memcached Instance

Refer to the [example.go](example/example.go) file.

```go
err := s.Memcached.Set(&memcache.Item{Key: "key", Value: []byte("value")})
item, err := s.Memcached.Get("key")
```

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three core Memcached operations:

* **String SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **INCR counter** — seed a key with `Set(...)` and then atomically increment it via `Increment(...)`.
* **DELETE + cache miss** — remove a key with `Delete(...)` and confirm a subsequent `Get(...)` returns `ErrCacheMiss`.

## Advanced Features

* **Supports multiple Memcached instances**: You can define multiple Memcached instances in the configuration file and
  reference them by name in your project.
* **Support Memcached extensions**: You can extend Memcached functionality by implementing the `Driver` interface — see
  the example implementation `AnotherMemcachedDriver`.
* **Startup connection validation (fail-fast)**: after building the client the starter issues a `Ping` against every
  configured server; an unreachable server fails the boot instead of the first request.
* **Service discovery**: set `service-name` (and optionally `discovery` to pick a registered backend, default `default`)
  instead of `servers`; the starter resolves the server list once through the registered `discovery.Discovery` backend
  at startup and shards keys across it. Because gomemcache hashes keys onto a fixed server set chosen at client creation,
  the resolve is **one-shot at boot** (fail-fast on empty/failed resolve) rather than a live watch — a changing cluster
  membership requires a restart. See [discovery.go](example/discovery.go) for a backend example.
* **Health check / readiness**: the client's `Ping()` probes all servers and is the readiness signal — call it straight
  off the autowired client.
* **Connection pool / timeouts**: `timeout` and `max-idle-conns` map to the client's per-server socket timeout and idle
  connection pool; both fall back to the driver defaults (100ms / 2) when left at 0.
* **Authentication**: the `bradfitz/gomemcache` driver does not implement SASL, so no auth fields are exposed. Restrict
  access at the network layer (VPC/security group) instead.

## Observability

`bradfitz/gomemcache` ships no official OpenTelemetry instrumentation, and there is no clean community equivalent that
hooks the client without wrapping every operation. Rather than bolt on a fragile wrapper, observability is intentionally
**not** built into this starter; instrument at the call site if you need tracing/metrics on cache access.
