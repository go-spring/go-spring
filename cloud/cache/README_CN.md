# cache

[English](README.md) | [中文](README_CN.md)

`cache` 是后端可插拔的 key/value 缓存抽象。缓存在配置里声明一次
（`spring.cache`），实现由后端 starter 提供（go-redis、redigo、bigcache、
memcached）。

## 快速开始

以 starter-go-redis 为例：

```properties
# 名为 "main" 的 redis client bean
spring.go-redis.instances.main.addr=127.0.0.1:6379
# 把它暴露成名为 "main" 的 cache.Cache bean
spring.cache.primary.driver=go-redis:main
```

其他驱动：`redigo:<pool>`、`bigcache:<instance>`、`memcached:<client>`。冒号
后的 beanID 指向要包装的后端 client bean，cache bean 也注册成同一个名字，
按名注入即可。`spring.cache.<key>` 只是迭代 key，缓存的身份就是 beanID。

## 使用

```go
type User struct{ Name string }

// 类型化访问。val 必须是指针，codec 在构造期固定（New(bc, WithCodec(...))），
// 默认 JSON。
err := c.Get(ctx, "user:42", &user)        // 不存在时返回 cache.ErrMiss
_  = c.Set(ctx, "user:42", user, 300)      // ttl 单位为秒

// 少数格式不同的条目：同一个 WithCodec 选项，传给那一次调用而不是 New。
_ = c.Set(ctx, "icon:42", icon, 0, cache.WithCodec(gobCodec))
err = c.Get(ctx, "icon:42", &icon, cache.WithCodec(gobCodec))

// 原始字节，绕过 codec，调用方本来就持有字节时用。
b, err := c.GetBytes(ctx, "icon:42")       // 不存在时 (nil, cache.ErrMiss)
_  = c.SetBytes(ctx, "icon:42", png, 0)    // 非正 ttl = 不过期
_  = c.Delete(ctx, "user:42")              // 删不存在的 key 不算错
```

### 未命中与故障

key 不存在返回哨兵 `ErrMiss`，不是后端错误。各后端把自家的"key 不存在"
（redis.Nil、memcache.ErrCacheMiss、bigcache.ErrEntryNotFound）都归到它上面，
read-through 不用写后端相关的错误判断：

```go
err := c.Get(ctx, key, &v)
if errors.Is(err, cache.ErrMiss) {
    v, err = loadFromSource(ctx, key)      // 只有真未命中才回落数据源
    _ = c.Set(ctx, key, v, 300)
}
```

### TTL 语义

ttl 是整秒数，非正数表示不过期。支持逐条 ttl 的后端（go-redis、redigo、
memcached）都会应用它。bigcache 做不到——它按构造期设置的全局 `LifeWindow`
过期——因此忽略该参数，并在自己的文档里说明，不会 panic。

## 可观测

`New` 会给每个后端套上观测装饰器，所有操作（类型化或原样提升）都在
`go-spring.org/cloud/cache` 的 meter/tracer 下记录同一套信号：

- `cache.operation.total` —— 按 `operation` × 互斥 `status` 计数
  （`get`：`hit`/`miss`/`error`；`set`/`delete`：`ok`/`error`）。按 status 求和
  即执行的操作数；命中率 = `rate(get.hit) / rate(get.hit + get.miss)`。
- `cache.operation.duration` —— 同一套 `operation` × `status` 维度上的直方图。
  在这里记而不只靠后端客户端，是因为后者的 `db.client.operation.duration`
  分不出命中与未命中，而这两种的时延分布本来就不同。
- 不自建 span。装饰器通过框架的 span 属性载体贡献 `cache.operation` 与 `cache.key`，
  它们会落到下层 executor 开启的那个 span 上。status 不在其中：未命中要等调用返回才
  知道，那时 span 已经结束了——它只作为指标维度存在。

观测单位是字节层：取到字节即为 hit，哪怕上层 codec 随后解码失败。key 永不进
指标（基数无界），只进 span。不记日志：缓存调用太频繁，逐调用日志只是噪音，
后端错误继续由各后端自己的插桩上报。

## 实现一个后端

实现 `ByteCache`，三个方法就是远程客户端按 1:1 映射到原生 API 的原语。类型
化访问不归后端管：`Cache` 嵌入 `ByteCache` 后加一层 codec，原始方法原样提升。

```go
type ByteCache interface {
    GetBytes(ctx context.Context, key string) ([]byte, error) // 不存在时 (nil, ErrMiss)
    // ttlSeconds 是整秒数，非正数表示不过期。
    SetBytes(ctx context.Context, key string, val []byte, ttlSeconds int) error
    Delete(ctx context.Context, key string) error
}
```

实现要求并发安全。要让后端对 `spring.cache` 可用，在 starter-cache 里
`RegisterDriver("my-backend", ...)` 注册一个 driver。driver 是 bean 构造工厂：
给它后端 client 的 bean 名，返回提供 `Cache` bean 的 module，本包和 driver 都
不 import 具体 client 类型。

换序列化格式不需要新后端。WithCodec 出现在哪就管到哪：传给 New 固定整个
缓存的格式（默认 JSON）；传给某一次 Get/Set，覆盖与整体格式不同的个别条目。
codec 不匹配会在解码时报错，不会静默写坏数据。

## InnerCache 掏空模式

把客户端掏空成 `InnerCache` 链头（外壳内部经由的接口链条）的模式，只适用于
「原始客户端以具体类型交付」的组件。判据全在这里：

- **具体类型交付**（`*bigcache.BigCache`、memcached client）→ 掏空它，暴露一个
  `InnerCache` 接口，作为链唯一的缝。
- **本身就是接口，或带原生 hook**（go-redis 的 `UniversalClient`、grpc 的
  `ClientConn`、gorm Plugin、mongo `CommandMonitor`、kotel）→ 直接包装或直用 hook；
  用户自己就能套洋葱，再造缝是重复建设。

链条形状：

- 外壳嵌入链头（命令方法直接提升）+ 导出原始对象字段——**只读把手**，供链构造器 /
  Driver 组装，严禁重赋值或直接跑命令。外壳零功能知识；每层一职。
- `Release(releaseRaw bool)` 是透传协议：每层先释放本层资源、把标记原样传下层，只有
  链尾对标记行动，`true` 关实例。外壳自身的 `Destroy` 已删——`Close()` = 链头
  `Release(true)`，兼任 gs destroy。
- **重组** = `c.InnerClient = myLayer{c.InnerClient}`：零额外把手、零构造，层改写的
  key 自然流进身份 / 观测层，治理照常保护下层。早期「浅关 + 全量重组装」协议已废弃。
- 替换仍存活的观测头须先 `Release(false)` 撤旧注册，否则双倍上报。
- 进程内缓存两层：`ObsCache`（全部可观测，per-call 计数 + gauge，持原始
  `statObserver`；`Release` 撤注册）→ `RawCache`（纯适配器，丢 ctx，`Release(true)`
  才关实例）；RPC 类客户端三层：`ObsClient`（身份声明 `WithOperation(op, key)`）→
  `GuardClient`（治理，executor 的构造 / 使用 / 释放全在层内）→ `RawClient`。
- 命令面全量进接口；泛型守卫函数必须是自由函数（Go 方法不许类型参数）。
- 多轮被否的中间形态（别再提）：构造函数注入链（逼用户放弃默认 Driver）、obs 挂外壳
  字段（外壳不纯）、注册表按实例键控、`RawCache` 持注册（跨层污染）。

MQ 族通用教训：per-delivery consume 流水线（extract → declare → exec）与链条由外向内
方向相反，无法上链——保留融合 wrapper（`Client.execute` / `GuardedConsume`）；链条只收
**同步发布**命令，异步 produce 不上链；裸 bean + `sync.Map` 注册表形态的掏空 = 引入包装
实体、bean 类型变更（experimental 允许）。

## Cache 包装面

Cache 封装永不加 `Reset`（清空全部）类破坏性方法：

- `StarterBigCache.Cache.Reset()` 已删且永不回加，其他 cache starter 封装同。
- 透传只收内省类（`Len` / `Stats` / `Iterator` 等）；`Reset` / `FlushAll` 类一律不加。
  清空缓存是运维动作（重建实例 / 改配置），不是一次方法调用。（2026-10-04）

## 边界

- 不做进程内 `Memory`、`MultiLevel`、aspect 桥接。进程内这一层用 bigcache，
  其他适配由调用方自己写。
- 不做击穿保护、异步刷新、负缓存。这些是调用方的策略，不进接口。
- 本包不依赖容器。driver 注册表和 `spring.cache` module 在 starter-cache，
  import 哪个后端 starter，接线就由它激活。

## 安装

```
go get go-spring.org/cloud
```
