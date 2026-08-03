# cache
[English](README.md) | [中文](README_CN.md)

`cache` 是后端可插拔的 key/value 缓存抽象。缓存关注点在配置里
(`spring.cache`)声明一次,由接进来 的 starter 决定实际后端。

## 特性

- 唯一的 `Cache` 接口,所有后端都实现它:类型化的 `Get`/`Set`(值通过可插拔
  `Codec`、默认 `JSONCodec` 跨越 字节/any 边界)+ 裸字节的 `GetBytes`/`SetBytes`
  (调用方已持有字节时用)。
- `ErrMiss` —— key 不存在是哨兵错误,与后端故障区分,调用方只在真正 miss 时才
  回源。
- 驱动注册表(`RegisterDriver`/`GetDriver`),与 discovery / resilience 的 driver
  习惯一致;空名、nil、重复注册在 init 期 panic。
- 配置驱动的 module:在 `spring.cache` 下,每条的
  `driver = "<driver>:<beanID>"` 选一个已注册驱动及其包裹的后端 bean。

## 导入

```
go get go-spring.org/spring
```

```go
import "go-spring.org/spring/data/cache"
```

## 配置

starter 既创建后端 client bean,又注册自己的 cache 驱动。以 starter-go-redis 为例:

```properties
# 名为 "main" 的 redis client bean
spring.go-redis.main.addr=127.0.0.1:6379
# 把它暴露成名为 "main" 的 cache.Cache bean
spring.cache.primary.driver=go-redis:main
```

其它驱动:`redigo:<pool>`、`bigcache:<instance>`、`memcached:<client>`。冒号后的
beanID 指定要包裹哪个后端 client bean;cache bean 就以该 beanID 注册,故按这个名字
注入。

## 用法

```go
type User struct{ Name string }

// 类型化 —— val 必须是指针;默认 JSON 编解码。
err := c.Get(ctx, "user:42", &user)        // 不存在返回 cache.ErrMiss
_  = c.Set(ctx, "user:42", user, 5*time.Minute)

// 裸字节 —— 绕过 codec。
b, err := c.GetBytes(ctx, "icon:42")       // 不存在返回 (nil, cache.ErrMiss)
_  = c.SetBytes(ctx, "icon:42", png, 0)    // ttl 非正 = 永不过期
```

`ttl` 语义因后端而异:go-redis / redigo / memcached 遵守 per-entry ttl(非正 =
永不过期);bigcache 忽略它,改用构造时设的全局 `LifeWindow`。
