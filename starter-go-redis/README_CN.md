# starter-go-redis

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-go-redis` 提供了基于 go-redis 的 Redis 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 Redis。

## 安装

```bash
go get github.com/go-spring/starter-go-redis
```

## 快速开始

### 1. 引入 `starter-go-redis` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "github.com/go-spring/starter-go-redis"
```

### 2. 配置 Redis 实例

在项目的[配置文件](example/conf/app.properties)中添加 Redis 配置，比如：

```properties
spring.go-redis.main.addr=127.0.0.1:6379
```

### 3. 注入 Redis 实例

参见 [example.go](example/example.go) 文件。

```go
import "github.com/redis/go-redis/v9"

type Service struct {
    Redis *redis.Client `autowire:""`
}
```

### 4. 使用 Redis 实例

参见 [example.go](example/example.go) 文件。

```go
str, err := s.Redis.Get(r.Context(), "key").Result()
str, err := s.Redis.Set(r.Context(), "key", "value", 0).Result()
```

## 高级功能

* **支持多 Redis 实例**：可以在配置文件中定义多个 Redis 实例，并在项目中使用 name 进行引用。
* **支持 Redis 扩展**：可以通过实现 `Driver` 接口来扩展 Redis 功能，参见示例中的 `AnotherRedisDriver` 实现。
