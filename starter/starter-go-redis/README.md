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

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three core Redis operations:

* **String SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **INCR counter** — reset a key with `Del(...)` and then atomically increment it via `Incr(...)`.
* **EXPIRE + TTL** — attach an expiration with `Expire(...)` and inspect the remaining lifetime via `TTL(...)`.

## Advanced Features

* **Supports multiple Redis instances**: You can define multiple Redis instances in the configuration file and reference
  them by name in your project.
* **Support Redis extensions**: You can extend Redis functionality by implementing the `Driver` interface — see the
  example implementation `AnotherRedisDriver`.
* **Startup connection validation (fail-fast)**: after building the client the starter issues a `Ping`; a misconfigured
  address or unreachable server fails the boot instead of the first request.
* **Health check / readiness**: the go-redis client exposes `Ping(ctx)` for readiness probes — call it straight off the
  autowired client.
* **Connection-pool monitoring**: `client.PoolStats()` returns live pool counters (hits, misses, total/idle conns) for
  runtime monitoring.
* **TLS**: enable `tls.enabled` and provide `ca-file` (and `cert-file`/`key-file` for mutual TLS) to dial Redis over TLS;
  see the commented block in [app.properties](example/conf/app.properties).
