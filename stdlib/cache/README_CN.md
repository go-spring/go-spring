# cache
[English](README.md) | [中文](README_CN.md)

`cache` 是零依赖、框架无关的 key/value 缓存抽象。缓存关注点一次声明,后端
可自由切换(进程内、Redis、memcached、bigcache、多级)而不动业务代码。

## 特性

- `Cache` 接口:`Get` / `Set` / `Delete`,值为 `any`。
- 包级驱动注册表(`Register` / `Get` / `MustGet`),按名选后端。
- `Memory` —— 零依赖并发安全的进程内缓存,按条目过期,保留具体 Go 类型
  (不序列化)。
- `ByteStore` + `Codec` + `FromByteStore` —— 字节型远端后端(Redis / memcached
  / bigcache)共用的唯一 seam,`JSONCodec` 是默认编解码。
- `MultiLevel` —— 近→远层级,读时回填、写/删除扇出。
- `Key` / `Namespace` —— 冒号连接的复合 key 助手。
- `AsStore` —— 把 `Cache` 桥到 `aspect.Store`,任一已注册后端都能给 `Cache`
  拦截器用。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

```go
import (
    "context"
    "time"

    "go-spring.org/stdlib/cache"
)

// 进程内缓存。
c := cache.NewMemory()
_ = c.Set(ctx, "user:42", &User{Name: "Ada"}, 5*time.Minute)
v, ok, _ := c.Get(ctx, "user:42")
```

近端 Memory 与 starter 注册的远端组合成多级缓存:

```go
remote := cache.MustGet("redis")               // starter-go-redis 注册
local  := cache.NewMemory()
c      := cache.NewMultiLevel(30*time.Second, local, remote)
```

接入 aspect `Cache` 拦截器:

```go
import (
    "go-spring.org/stdlib/aspect"
    "go-spring.org/stdlib/cache"
)

userKey := cache.Namespace("user")
chain := aspect.NewChain(
    aspect.Cache(
        cache.AsStore(ctx, c),
        func(jp *aspect.Joinpoint) string { return userKey(jp.Method) },
        5*time.Minute,
    ),
)
```
