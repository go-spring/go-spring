# starter-redigo

[English](README.md) | [中文](README_CN.md)

`starter-redigo` provides a Redis client wrapper based on redigo for
Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-redigo
```

## Quick Start

### 1. Import the `starter-redigo` Package

```go
import _ "go-spring.org/starter-redigo"
```

### 2. Configure the Redis Instance

Add Redis configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.redigo.main.addr=127.0.0.1:6379
```

### 3. Inject the Redis Instance

```go
import StarterRedigo "go-spring.org/starter-redigo"

type Service struct {
    Redis *StarterRedigo.Pool `autowire:"main"`
}
```

### 4. Use the Redis Instance

```go
c := s.Redis.Get() // borrow a pooled connection
defer c.Close()
str, err := redis.String(c.Do("GET", "key"))
_, err = c.Do("SET", "key", "value")
```

## Core Features

The [example.go](example/example.go) file demonstrates the following core Redis features:

* **String SET/GET**: store a string value with `SET` and retrieve it with `GET`.
* **INCR counter**: atomically increment an integer counter with `INCR`.
* **EXPIRE + TTL**: attach a time-to-live to a key with `EXPIRE` and inspect it with `TTL`.

## Advanced Features

* **Supports multiple Redis instances**: you can define multiple Redis instances in the configuration file and reference them by name.
* **Support Redis extensions**: implement the `Driver` interface to extend Redis functionality — see the
  example implementation `AnotherRedisDriver`.
* **Startup connection validation (opt-in)**: set `startup-ping=true` to borrow a connection and
  `PING` at boot; with the default `false`, a misconfigured address surfaces on the first command
  (redigo pools are lazy).
* **Service discovery**: set `service-name` (and optionally `discovery` to pick a registered backend, default `default`)
  instead of `addr`; a `Resolver` resolves the service through the registered `discovery.Discovery` backend and dials a
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
instrumentation. The starter therefore emits its own: every pooled command goes through a module-local observe layer
(client span with `db.system`/`db.operation`/`db.statement` attributes, the `db.client.operation.duration` histogram,
and an access log tagged `_app_redigo_access`). All three are no-ops unless `starter-otel` installs providers.
