# starter-redigo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-redigo` provides a Redis client wrapper based on redigo,
making it easy to integrate and use Redis in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-redigo
```

## Quick Start

### 1. Import the `starter-redigo` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-redigo"
```

### 2. Configure the Redis Instance

Add Redis configuration in your project’s [configuration file](example/conf/app.properties), for example:

```properties
spring.redigo.main.addr=127.0.0.1:6379
```

### 3. Inject the Redis Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/gomodule/redigo/redis"

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

The [example.go](example/example.go) file demonstrates the following core Redis features:

* **String SET/GET**: store a string value with `SET` and retrieve it with `GET`.
* **INCR counter**: atomically increment an integer counter with `INCR`.
* **EXPIRE + TTL**: attach a time-to-live to a key with `EXPIRE` and inspect it with `TTL`.

## Advanced Features

* **Supports multiple Redis instances**: You can define multiple Redis instances in the configuration file and reference
  them by name in your project.
* **Support Redis extensions**: You can extend Redis functionality by implementing the `Driver` interface — see the
  example implementation `AnotherRedisDriver`.
* **Startup connection validation (fail-fast)**: after building the pool the starter borrows a connection and issues a
  `PING`; a misconfigured address or unreachable server fails the boot instead of the first request.
* **Service discovery**: set `service-name` (and optionally `discovery` to pick a registered backend, default `default`)
  instead of `addr`; a `LiveDialer` resolves the service through the registered `discovery.Discovery` backend and dials a
  live endpoint for every new pool connection. Combined with `conn-max-lifetime`, pooled connections recycle onto updated
  addresses without rebuilding the pool. On shutdown the starter stops the background watch. This mirrors
  `starter-go-redis`; see [discovery.go](example/discovery.go) for a backend example.
* **Health check / readiness**: borrow a connection and run `PING` for readiness probes.
* **Connection-pool monitoring**: `pool.Stats()` returns live pool counters (active/idle connections) for runtime
  monitoring.
* **TLS**: enable `tls.enabled` and provide `ca-file` (and `cert-file`/`key-file` for mutual TLS) to dial Redis over TLS.
  The TLS field layout matches `starter-go-redis`, so switching between the two starters only changes the import.

## Observability

Unlike `starter-go-redis` (which uses the official `redisotel` hooks), redigo ships no official OpenTelemetry
instrumentation, and there is no clean community equivalent that hooks the connection without wrapping every command.
Rather than bolt on a fragile wrapper, observability is intentionally **not** built into this starter. Applications that
need tracing/metrics on cache access should prefer `starter-go-redis`, or instrument at the call site.
