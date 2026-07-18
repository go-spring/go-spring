# starter-go-redis

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-go-redis` 提供了基于 go-redis 的 Redis 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 Redis。

## 安装

```bash
go get go-spring.org/starter-go-redis
```

## 快速开始

### 1. 引入 `starter-go-redis` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-go-redis"
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

## 拓扑（单机 / 哨兵 / 集群）

`mode` 配置项用于选择 Redis 拓扑，默认 `single`，因此原有的单机配置无需改动即可继续工作。

### single（默认）

通过 `addr`（或服务发现的 `service-name`）连接单个节点，bean 类型为 `*redis.Client`。

```properties
spring.go-redis.cache.addr=127.0.0.1:6379
# 或通过服务发现解析地址：
# spring.go-redis.cache.service-name=redis-main
```

### sentinel（哨兵）

连接由哨兵解析出的主节点组，bean 类型仍是 `*redis.Client`，注入方式与命令集与单机完全一致。

```properties
spring.go-redis.cache.mode=sentinel
spring.go-redis.cache.master-name=mymaster
spring.go-redis.cache.sentinel-addrs=127.0.0.1:26379,127.0.0.1:26380
# spring.go-redis.cache.sentinel-password=...   # 向哨兵自身认证的密码
```

### cluster（集群）

用集群入口节点作为种子地址，bean 类型为 **`*redis.ClusterClient`**（与单机不同），需按此类型注入：

```go
type Service struct {
    Cluster *redis.ClusterClient `autowire:"cache"`
}
```

```properties
spring.go-redis.cache.mode=cluster
spring.go-redis.cache.addrs=127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002
# 可选的集群调优项：
# spring.go-redis.cache.max-redirects=3
# spring.go-redis.cache.route-by-latency=true
# spring.go-redis.cache.route-randomly=true
```

TLS、连接池大小、超时、OTel 埋点以及启动期 fail-fast `Ping` 对三种拓扑均生效。服务发现（`service-name`）**仅对单机模式生效**：
哨兵与集群自带节点发现，若在这两种模式下同时设置 `service-name`，启动即报错。

完整可运行示例见 [app.properties](example/conf/app.properties) 中的 `sentinel` 与 `cluster` 实例，
本地拉起三种拓扑参见 [docker-compose.yml](example/docker-compose.yml)。

## 核心功能

[example.go](example/example.go) 示例程序演示并断言了三项 Redis 核心操作：

* **字符串 SET/GET**：通过 `Set(...)` 写入值，再通过 `Get(...)` 读回。
* **INCR 计数器**：先通过 `Del(...)` 复位，再使用 `Incr(...)` 原子自增。
* **EXPIRE + TTL**：使用 `Expire(...)` 为 key 设置过期时间，并通过 `TTL(...)` 查询剩余存活时间。

## 高级功能

* **支持多 Redis 实例**：可以在配置文件中定义多个 Redis 实例，并在项目中使用 name 进行引用。
* **多拓扑支持**：`mode` 可选 `single`（默认）、`sentinel`、`cluster`，详见上文"拓扑"一节。集群实例暴露为
  `*redis.ClusterClient`，单机/哨兵为 `*redis.Client`。
* **支持 Redis 扩展**：可以通过实现 `Driver` 接口来扩展 Redis 功能，参见示例中的 `AnotherRedisDriver` 实现。集群支持是
  可选的 `ClusterDriver` 接口，因此已有的自定义 Driver 无需改动即可继续编译。
* **启动期连接校验（fail-fast）**：创建客户端后会执行一次 `Ping`，地址配置错误或服务不可达时启动即失败，而非等到首次请求。
* **健康检查 / readiness**：go-redis 客户端自带 `Ping(ctx)`，可直接在注入的客户端上调用做健康探测。
* **连接池运行时监控**：`client.PoolStats()` 返回连接池实时计数（命中、未命中、总连接/空闲连接）。
* **TLS**：开启 `tls.enabled` 并提供 `ca-file`（双向 TLS 再加 `cert-file`/`key-file`）即可通过 TLS 连接 Redis，
  详见 [app.properties](example/conf/app.properties) 中的注释示例。
