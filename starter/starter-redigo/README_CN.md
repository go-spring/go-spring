# starter-redigo

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-redigo` 提供了基于 redigo 的 Redis 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 Redis。

## 安装

```bash
go get go-spring.org/starter-redigo
```

## 快速开始

### 1. 引入 `starter-redigo` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-redigo"
```

### 2. 配置 Redis 实例

在项目的[配置文件](example/conf/app.properties)中添加 Redis 配置，比如：

```properties
spring.redigo.main.addr=127.0.0.1:6379
```

### 3. 注入 Redis 实例

参见 [example.go](example/example.go) 文件。

```go
import "github.com/gomodule/redigo/redis"

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

## 核心功能

[example.go](example/example.go) 文件演示了以下核心 Redis 功能：

* **字符串 SET/GET**：使用 `SET` 存储字符串值，使用 `GET` 读取。
* **INCR 计数器**：使用 `INCR` 对整型计数器进行原子自增。
* **EXPIRE + TTL**：使用 `EXPIRE` 为键设置过期时间，使用 `TTL` 查询剩余存活时间。

## 高级功能

* **支持多 Redis 实例**：可以在配置文件中定义多个 Redis 实例，并在项目中使用 name 进行引用。
* **支持 Redis 扩展**：可以通过实现 `Driver` 接口来扩展 Redis 功能，参见示例中的 `AnotherRedisDriver` 实现。
* **启动期连接校验（fail-fast）**：创建连接池后会借出一个连接执行 `PING`，地址配置错误或服务不可达时启动即失败，而非等到首次请求。
* **服务发现**：配置 `service-name`（可选 `discovery` 指定已注册的后端，默认 `default`）替代 `addr`；`LiveDialer` 通过注册的
  `discovery.Discovery` 后端解析服务，并在连接池每次新建连接时拨向一个存活实例。配合 `conn-max-lifetime`，池内连接会平滑
  切换到更新后的地址而无需重建连接池。关闭时 starter 会停止后台 watch。该范式与 `starter-go-redis` 对齐，后端示例参见
  [discovery.go](example/discovery.go)。
* **健康检查 / readiness**：借出连接执行 `PING` 即可做健康探测。
* **连接池运行时监控**：`pool.Stats()` 返回连接池实时计数（活跃/空闲连接）。
* **TLS**：开启 `tls.enabled` 并提供 `ca-file`（双向 TLS 再加 `cert-file`/`key-file`）即可通过 TLS 连接 Redis。
  TLS 字段布局与 `starter-go-redis` 一致，两个 starter 切换只需改 import。

## 可观测

与 `starter-go-redis`（使用官方 `redisotel` 钩子）不同，redigo 没有官方的 OpenTelemetry 埋点，社区也没有一个能在不包装每条
命令的前提下干净接入连接层的等价方案。为避免引入脆弱的包装层，本 starter **有意不内置**可观测。需要对缓存访问做 tracing/metrics
的应用，建议改用 `starter-go-redis`，或在调用侧自行埋点。
