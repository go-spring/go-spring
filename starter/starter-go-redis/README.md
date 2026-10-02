# starter-go-redis

[English](README.md) | [中文](README_CN.md)

`starter-go-redis` provides a Redis client wrapper based on go-redis for
Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-go-redis
```

## Quick Start

### 1. Import the `starter-go-redis` Package

```go
import _ "go-spring.org/starter-go-redis"
```

### 2. Configure the Redis Instance

Add Redis configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.go-redis.instances.main.addr=127.0.0.1:6379
```

### 3. Inject the Redis Instance

```go
import StarterGoRedis "go-spring.org/starter-go-redis"

type Service struct {
    Redis *StarterGoRedis.Client `autowire:"main"` // wraps redis.UniversalClient
}
```

### 4. Use the Redis Instance

```go
str, err := s.Redis.Get(r.Context(), "key").Result()
str, err := s.Redis.Set(r.Context(), "key", "value", 0).Result()
```

## Topologies (single / sentinel / cluster)

The `mode` property selects the Redis topology; it defaults to `single`, so
existing single-node configurations keep working unchanged.

### single (default)

Dials one node via `addr` (or a service name via discovery). The raw client is
`*redis.Client`.

```properties
spring.go-redis.instances.cache.addr=127.0.0.1:6379
# or resolve the address via service discovery:
# spring.go-redis.instances.cache.service-name=redis-main
```

### sentinel

Connects to the master group resolved through the sentinels. The raw client is
still `*redis.Client`, so injection and the command surface are identical to
single mode.

```properties
spring.go-redis.instances.cache.mode=sentinel
spring.go-redis.instances.cache.master-name=mymaster
spring.go-redis.instances.cache.sentinel-addrs=127.0.0.1:26379,127.0.0.1:26380
# spring.go-redis.instances.cache.sentinel-password=...   # auth to the sentinels themselves
```

### cluster

Seeds the client with the cluster entry nodes. The raw client is a
`*redis.ClusterClient`, but the wrapper hides that — the bean is registered as
the same **`*redis.Client` wrapper** (`*StarterGoRedis.Client`) every other mode
uses, so inject it the same way:

```go
type Service struct {
    Cluster *StarterGoRedis.Client `autowire:"cache"`
}
```

```properties
spring.go-redis.instances.cache.mode=cluster
spring.go-redis.instances.cache.addrs=127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002
# optional cluster tunables:
# spring.go-redis.instances.cache.max-redirects=3
# spring.go-redis.instances.cache.route-by-latency=true
# spring.go-redis.instances.cache.route-randomly=true
```

TLS, connection-pool sizing, timeouts, OTel pool metrics, and the fail-fast
startup `HealthCheck` all apply to every topology. The starter declares each
command's semantic identity (`observe.go`) and the resilience layer emits the
signals — the per-command span, the call-level and attempt-level duration
histograms, and the access log (tag `_app_redis_access`). Service discovery
(`service-name`) applies to **single mode only**: sentinel and cluster
self-discover their nodes, so combining `service-name` with those modes is
rejected at startup.

See the `sentinel` and `cluster` instances in
[app.properties](example/conf/app.properties) for a full working example, and
[docker-compose.yml](example/docker-compose.yml) for bringing up all three
topologies locally.

## Compatibility

Dragonfly and Kvrocks speak the Redis wire protocol, so this starter drives
them unchanged — point `addr` at the Dragonfly/Kvrocks endpoint. RESP-only
features are shared; features outside the protocol (Dragonfly's
multi-tenancy, Kvrocks' namespace) are out of scope and need a custom driver.

## Core Features

The [main.go](example/main.go) program demonstrates and asserts three core Redis operations:

* **String SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **INCR counter** — reset a key with `Del(...)` and then atomically increment it via `Incr(...)`.
* **EXPIRE + TTL** — attach an expiration with `Expire(...)` and inspect the remaining lifetime via `TTL(...)`.

## Advanced Features

* **Supports multiple Redis instances**: you can define multiple Redis instances in the configuration file and reference them by name.
* **Multiple topologies**: `mode` selects `single` (default), `sentinel`, or `cluster` — see the Topologies section
  above. Every topology registers the same wrapper bean, `*StarterGoRedis.Client`; the raw client it embeds is a
  `*redis.Client` (single/sentinel) or a `*redis.ClusterClient` (cluster).
* **Support Redis extensions**: implement the `Driver` interface to extend Redis functionality. `CreateClient`
  returns the wrapper (`*StarterGoRedis.Client`), built with `NewClient`; cluster support is an optional
  `ClusterDriver` interface, so a Driver that only builds single/sentinel clients stays valid. When several Driver
  beans coexist, an entry selects one by name:
  `spring.go-redis.instances.<name>.driver = <bean-name>` (empty = fall back to the family-wide `spring.<family>.default.driver`, then to the single Driver bean by type; naming a missing
  bean fails startup).
* **Startup connection validation (opt-in)**: set `ping=true` and after building the client the starter runs
  `HealthCheck` (a `Ping`), so a misconfigured address or unreachable server fails the boot instead of the first
  request. Off by default: a backend that is not up yet must not block startup.
* **Health check / readiness**: `HealthCheck(ctx, client)` is the readiness probe — the autowired `health.Indicator`
  delegates to it, and you can call it straight off the autowired client.
* **Connection-pool monitoring**: `client.PoolStats()` returns live pool counters (hits, misses, total/idle conns) for
  runtime monitoring.
* **TLS**: enable `tls.enabled` and provide `ca-file` (and `cert-file`/`key-file` for mutual TLS) to dial Redis over TLS;
  see the commented block in [app.properties](example/conf/app.properties).
* **Distributed cache backend**: `AsCache(client, codec)` adapts the client to `cloud/cache.Cache`, the shared (far)
  level of a multi-level cache. Values are serialized with the codec (nil defaults to JSON).
* **Distributed rate-limit counters**: `NewCounters(client)` returns a `resilience.Counters` backed by an atomic
  Lua token bucket. Contribute it as a bean and every executor the driver builds spends it, so one budget per scope
  is shared across replicas — with none contributed each executor keeps a private budget of its own.
