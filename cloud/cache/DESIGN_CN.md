# cache 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`cache` 是 data 层的缓存抽象:所有后端都实现的 `ByteCache` 接口,由套了 codec 的
`Cache` 结构体外壳包裹,加上驱动注册表和把具名后端接成 bean 的配置驱动 module。

## 1. 职责与边界

- **做:** 定义 `Cache` 接口;提供 `Codec`/`JSONCodec` 序列化 seam 与
  `ResolveCodec` 默认;运行 `spring.cache` module 读配置并派发给已注册驱动;暴露
  `RegisterDriver`/`GetDriver` 注册表。
- **不做:**
  - 不提供具体后端。Redis / redigo / bigcache / memcached 的适配各自在 starter
    里,每个注册一个驱动、包裹自己的 client bean。
  - 本包不做进程内 `Memory`、不做 `MultiLevel`、不做 aspect 桥接。(更早的
    experimental 版本有,为聚焦字节原生后端已移除。)
  - 不做击穿保护、异步刷新、负缓存——这些是调用方的策略,不属于通用接口。

## 2. 关键抽象与缝隙

- **`ByteCache` 是字节原生的后端契约。** `GetBytes`/`SetBytes`/`Delete` 是远端
  后端(Redis、memcached、bigcache)与各自 client 一一映射的裸原语;后端只实现
  这三个。
- **`Cache` 是套 codec 的结构体外壳。** `Cache` 嵌入 `ByteCache`,在其上加
  `Get`/`Set`,通过可插拔 `Codec`(默认 `JSONCodec`)在裸字节上做
  marshal/unmarshal;`ByteCache` 的方法原样提升,已持有字节的调用方直接用。
  `ResolveCodec` 把默认收口到唯一需要它的地方(`Cache` 方法),后端不再重写。
- **`ErrMiss` 把 miss 和故障分开。** 后端把原生的"key 不存在"
  (redis.Nil、memcache.ErrCacheMiss、bigcache.ErrEntryNotFound)映射成 `ErrMiss`;
  其它错误才是真故障。调用方只在 `ErrMiss` 时回源。
- **Driver 是 bean 构造工厂。** `Driver func(beanID string) gs.ModuleFunc` 接收
  后端 client 的 bean 名,返回提供 `Cache` bean 的 module——所以本包永远不 import
  具体 client 类型。
- **配置派发。** 本包自带的 `init` module 读 `spring.cache`,解析每条的
  `driver = "<driver>:<beanID>"`,查驱动,调 `driver(beanID)`。beanID 既选定后端
  client,又作为产出 cache bean 的名字。

## 3. 不变量

- 注册表允许 init 期填充、运行期并发读,单个 `sync.RWMutex` 守护。
- `ByteCache` 实现必须并发安全。
- 无法遵守 per-entry TTL 的后端必须**静默忽略** `ttl` 参数并写明(bigcache 用全局
  `LifeWindow`)——绝不 panic。
- `driver` 解析要求驱动名和 beanID 都非空;否则作为配置错误从 module 返回,而非
  panic。

## 4. 权衡与放弃的方案

- **后端契约(`ByteCache`)与类型化外壳(`Cache`)分离。** 后端只实现三个字节
  原生方法;codec 逻辑只存在于 `Cache` 结构体方法一处,不逐个后端重复。`Cache`
  嵌入 `ByteCache`,裸方法原样提升——调用方看到一个完整表面的类型,后端实现最窄
  的那个。代价——将来若有"存活值"的进程内后端就得序列化——可接受(进程内用
  bigcache)。
- **codec 放在实例级,而非调用级。** codec 在构造期固定(`New(bc, WithCodec(...))`,
  默认 JSON),不是逐调用参数:cache 的格式是 cache 自身的属性,codec 不匹配会在
  decode 时显式报错而非静默损坏。需要混存多种格式的 cache 退回提升的裸方法
  `GetBytes`/`SetBytes`,在调用点显式给 codec。
- **cache bean 按后端 beanID 命名。** 配置槽(`spring.cache.X`)只是遍历键,bean
  名是 beanID 后缀,把 cache 身份耦到后端 client。有意为之:一个 client → 一个
  cache 名,重复接线在 init 期 panic,而非静默覆盖。
- **本包不做 aspect 桥接。** 旧的 `AsStore` 把本包耦到 `aspect` 及其 `Store` 契约。
  去掉以保持本包聚焦;aspect 适配器若需要,应归 aspect 或调用方。
