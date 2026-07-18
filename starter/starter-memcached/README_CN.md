# starter-memcached

[English](README.md) | [中文](README_CN.md)

`starter-memcached` 提供了基于 [gomemcache](https://github.com/bradfitz/gomemcache) 的 Memcached 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 Memcached。

## 安装

```bash
go get go-spring.org/starter-memcached
```

## 快速开始

### 1. 引入 `starter-memcached` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-memcached"
```

### 2. 配置 Memcached 实例

在项目的[配置文件](example/conf/app.properties)中添加 Memcached 配置，比如：

```properties
spring.memcached.main.servers=127.0.0.1:11211
```

### 3. 注入 Memcached 实例

参见 [example.go](example/example.go) 文件。

```go
import "github.com/bradfitz/gomemcache/memcache"

type Service struct {
    Memcached *memcache.Client `autowire:""`
}
```

### 4. 使用 Memcached 实例

参见 [example.go](example/example.go) 文件。

```go
err := s.Memcached.Set(&memcache.Item{Key: "key", Value: []byte("value")})
item, err := s.Memcached.Get("key")
```

## 核心功能

[example.go](example/example.go) 示例程序演示并断言了三项 Memcached 核心操作：

* **字符串 SET/GET**：通过 `Set(...)` 写入值，再通过 `Get(...)` 读回。
* **INCR 计数器**：先通过 `Set(...)` 播种，再使用 `Increment(...)` 原子自增。
* **DELETE + 缓存未命中**：通过 `Delete(...)` 删除 key，并确认随后的 `Get(...)` 返回 `ErrCacheMiss`。

## 高级功能

* **支持多 Memcached 实例**：可以在配置文件中定义多个 Memcached 实例，并在项目中使用 name 进行引用。
* **支持 Memcached 扩展**：可以通过实现 `Driver` 接口来扩展 Memcached 功能，参见示例中的 `AnotherMemcachedDriver` 实现。
* **启动期连接校验（fail-fast）**：创建客户端后会对每个配置的 server 执行一次 `Ping`，服务不可达时启动即失败，而非等到首次请求。
* **健康检查 / readiness**：客户端的 `Ping()` 会探测所有 server，可直接在注入的客户端上调用做健康探测。
* **连接池 / 超时**：`timeout` 与 `max-idle-conns` 对应客户端每 server 的 socket 超时和空闲连接池，为 0 时回退到驱动默认值（100ms / 2）。
* **认证**：`bradfitz/gomemcache` 驱动未实现 SASL，因此不暴露认证字段，请在网络层（VPC/安全组）限制访问。
