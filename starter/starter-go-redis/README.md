# starter-go-redis

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-go-redis` provides a Redis client wrapper based on go-redis,
making it easy to integrate and use Redis in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-go-redis
```

## Quick Start

### 1. Import the `starter-go-redis` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-go-redis"
```

### 2. Configure the Redis Instance

Add Redis configuration in your project’s [configuration file](example/conf/app.properties), for example:

```properties
spring.go-redis.main.addr=127.0.0.1:6379
```

### 3. Inject the Redis Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/redis/go-redis/v9"

type Service struct {
    Redis *redis.Client `autowire:""`
}
```

### 4. Use the Redis Instance

Refer to the [example.go](example/example.go) file.

```go
str, err := s.Redis.Get(r.Context(), "key").Result()
str, err := s.Redis.Set(r.Context(), "key", "value", 0).Result()
```

## Topologies (single / sentinel / cluster)

The `mode` property selects the Redis topology; it defaults to `single`, so
existing single-node configurations keep working unchanged.

### single (default)

Dials one node via `addr` (or a service name via discovery). The bean type is
`*redis.Client`.

```properties
spring.go-redis.cache.addr=127.0.0.1:6379
# or resolve the address via service discovery:
# spring.go-redis.cache.service-name=redis-main
```

### sentinel

Connects to the master group resolved through the sentinels. The bean type is
still `*redis.Client`, so injection and the command surface are identical to
single mode.

```properties
spring.go-redis.cache.mode=sentinel
spring.go-redis.cache.master-name=mymaster
spring.go-redis.cache.sentinel-addrs=127.0.0.1:26379,127.0.0.1:26380
# spring.go-redis.cache.sentinel-password=...   # auth to the sentinels themselves
```

### cluster

Seeds the client with the cluster entry nodes. The bean type is
**`*redis.ClusterClient`** — a distinct type — so inject it accordingly:

```go
type Service struct {
    Cluster *redis.ClusterClient `autowire:"cache"`
}
```

```properties
spring.go-redis.cache.mode=cluster
spring.go-redis.cache.addrs=127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002
# optional cluster tunables:
# spring.go-redis.cache.max-redirects=3
# spring.go-redis.cache.route-by-latency=true
# spring.go-redis.cache.route-randomly=true
```

TLS, connection-pool sizing, timeouts, OTel instrumentation, and the fail-fast
startup `Ping` all apply to every topology. Service discovery (`service-name`)
applies to **single mode only**: sentinel and cluster self-discover their nodes,
so combining `service-name` with those modes is rejected at startup.

See the `sentinel` and `cluster` instances in
[app.properties](example/conf/app.properties) for a full working example, and
[docker-compose.yml](example/docker-compose.yml) for bringing up all three
topologies locally.

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three core Redis operations:

* **String SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **INCR counter** — reset a key with `Del(...)` and then atomically increment it via `Incr(...)`.
* **EXPIRE + TTL** — attach an expiration with `Expire(...)` and inspect the remaining lifetime via `TTL(...)`.

## Advanced Features

* **Supports multiple Redis instances**: You can define multiple Redis instances in the configuration file and reference
  them by name in your project.
* **Multiple topologies**: `mode` selects `single` (default), `sentinel`, or `cluster` — see the Topologies section
  above. Cluster instances are exposed as `*redis.ClusterClient`; single/sentinel as `*redis.Client`.
* **Support Redis extensions**: You can extend Redis functionality by implementing the `Driver` interface — see the
  example implementation `AnotherRedisDriver`. Cluster support is an optional `ClusterDriver` interface, so existing
  custom drivers keep compiling unchanged.
* **Startup connection validation (fail-fast)**: after building the client the starter issues a `Ping`; a misconfigured
  address or unreachable server fails the boot instead of the first request.
* **Health check / readiness**: the go-redis client exposes `Ping(ctx)` for readiness probes — call it straight off the
  autowired client.
* **Connection-pool monitoring**: `client.PoolStats()` returns live pool counters (hits, misses, total/idle conns) for
  runtime monitoring.
* **TLS**: enable `tls.enabled` and provide `ca-file` (and `cert-file`/`key-file` for mutual TLS) to dial Redis over TLS;
  see the commented block in [app.properties](example/conf/app.properties).
* **Distributed cache backend**: `AsCache(client, codec)` adapts the client to `spring/cache.Cache`, the shared (far)
  level of a multi-level cache. Values are serialized with the codec (nil defaults to JSON).
* **Global rate limiter**: `NewRateLimiter(client, resilience.LimitPolicy{...})` returns a `resilience.RateLimiter`
  backed by an atomic Lua token bucket, so every replica shares one budget — in contrast to the per-replica builtin.
