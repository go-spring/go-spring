# starter-redigo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-redigo` provides a Redis client wrapper based on redigo,
making it easy to integrate and use Redis in Go-Spring applications.

## Installation

```bash
go get github.com/go-spring/starter-redigo
```

## Quick Start

### 1. Import the `starter-redigo` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "github.com/go-spring/starter-redigo"
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

## Advanced Features

* **Supports multiple Redis instances**: You can define multiple Redis instances in the configuration file and reference
  them by name in your project.
* **Support Redis extensions**: You can extend Redis functionality by implementing the `Driver` interface — see the
  example implementation `AnotherRedisDriver`.
