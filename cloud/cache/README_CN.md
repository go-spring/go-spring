# cache
[English](README.md) | [中文](README_CN.md)

`cache` 是后端可插拔的 key/value 缓存抽象。缓存在配置里声明一次
(`spring.cache`),实现由后端 starter 提供(go-redis、redigo、bigcache、
memcached)。

## 快速开始

以 starter-go-redis 为例:

```properties
# 名为 "main" 的 redis client bean
spring.go-redis.instances.main.addr=127.0.0.1:6379
# 把它暴露成名为 "main" 的 cache.Cache bean
spring.cache.primary.driver=go-redis:main
```

其他驱动:`redigo:<pool>`、`bigcache:<instance>`、`memcached:<client>`。冒号
后的 beanID 指向要包装的后端 client bean,cache bean 也注册成同一个名字,
按名注入即可。`spring.cache.<key>` 只是迭代 key,缓存的身份就是 beanID。

## 使用

```go
type User struct{ Name string }

// 类型化访问。val 必须是指针,codec 在构造期固定(New(bc, WithCodec(...))),
// 默认 JSON。
err := c.Get(ctx, "user:42", &user)        // 不存在时返回 cache.ErrMiss
_  = c.Set(ctx, "user:42", user, 5*time.Minute)

// 少数格式不同的条目:同一个 WithCodec 选项,传给那一次调用而不是 New。
_ = c.Set(ctx, "icon:42", icon, 0, cache.WithCodec(gobCodec))
err = c.Get(ctx, "icon:42", &icon, cache.WithCodec(gobCodec))

// 原始字节,绕过 codec,调用方本来就持有字节时用。
b, err := c.GetBytes(ctx, "icon:42")       // 不存在时 (nil, cache.ErrMiss)
_  = c.SetBytes(ctx, "icon:42", png, 0)    // 非正 ttl = 不过期
_  = c.Delete(ctx, "user:42")              // 删不存在的 key 不算错
```

### 未命中与故障

key 不存在返回哨兵 `ErrMiss`,不是后端错误。各后端把自家的"key 不存在"
(redis.Nil、memcache.ErrCacheMiss、bigcache.ErrEntryNotFound)都归到它上面,
read-through 不用写后端相关的错误判断:

```go
err := c.Get(ctx, key, &v)
if errors.Is(err, cache.ErrMiss) {
    v, err = loadFromSource(ctx, key)      // 只有真未命中才回落数据源
    _ = c.Set(ctx, key, v, 5*time.Minute)
}
```

### TTL 语义

go-redis、redigo、memcached 支持逐条 ttl,非正数表示不过期。bigcache 不认
这个参数,用构造期设置的全局 `LifeWindow`。无法支持逐条 ttl 的后端忽略该参
数,并在自己的文档里说明,不会 panic。

## 实现一个后端

实现 `ByteCache`,三个方法就是远程客户端按 1:1 映射到原生 API 的原语。类型
化访问不归后端管:`Cache` 嵌入 `ByteCache` 后加一层 codec,原始方法原样提升。

```go
type ByteCache interface {
    GetBytes(ctx context.Context, key string) ([]byte, error) // 不存在时 (nil, ErrMiss)
    SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}
```

实现要求并发安全。要让后端对 `spring.cache` 可用,在 starter-cache 里
`RegisterDriver("my-backend", ...)` 注册一个 driver。driver 是 bean 构造工厂:
给它后端 client 的 bean 名,返回提供 `Cache` bean 的 module,本包和 driver 都
不 import 具体 client 类型。

换序列化格式不需要新后端。WithCodec 出现在哪就管到哪:传给 New 固定整个
缓存的格式(默认 JSON);传给某一次 Get/Set,覆盖与整体格式不同的个别条目。
codec 不匹配会在解码时报错,不会静默写坏数据。

## 边界

- 不做进程内 `Memory`、`MultiLevel`、aspect 桥接。进程内这一层用 bigcache,
  其他适配由调用方自己写。
- 不做击穿保护、异步刷新、负缓存。这些是调用方的策略,不进接口。
- 本包不依赖容器。driver 注册表和 `spring.cache` module 在 starter-cache,
  import 哪个后端 starter,接线就由它激活。

## 安装

```
go get go-spring.org/cloud
```
