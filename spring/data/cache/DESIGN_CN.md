# cache 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`cache` 是 data 层的缓存抽象:所有后端都实现的唯一 `Cache` 接口,加上驱动注册表
和把具名后端接成 bean 的配置驱动 module。

## 1. 职责与边界

- **做:** 定义 `Cache` 接口;提供 `Codec`/`JSONCodec` 序列化 seam 与
  `ResolveCodec` 默认;运行 `spring.cache` module 读配置并派发给已注册驱动;暴露
  `RegisterDriver`/`GetDriver` 注册表。
- **不做:**
  - 不提供具体后端。Redis / redigo / bigcache / memcached 的适配各自在 starter
    里,每个注册一个驱动、包裹自己的 client bean。
  - 本包不做进程内 `Memory`、不做 `MultiLevel`、不做 aspect 桥接。(更早的
    experimental 版本有,接口简化为"字节原生后端"后已移除——见 §4。)
  - 不做击穿保护、异步刷新、负缓存——这些是调用方的策略,不属于通用接口。

## 2. 关键抽象与缝隙

- **`Cache` 是"字节原生 + 上层 codec"。** `GetBytes`/`SetBytes` 是远端后端
  (Redis、memcached、bigcache)与各自 client 一一映射的裸原语;`Get`/`Set` 在其
  上套 `Codec`(默认 `JSONCodec`)给类型化调用方用。`ResolveCodec` 把默认收口到
  一处,所有后端一致,不在各处重写。
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
- `Cache` 实现必须并发安全。
- 无法遵守 per-entry TTL 的后端必须**静默忽略** `ttl` 参数并写明(bigcache 用全局
  `LifeWindow`)——绝不 panic。
- `driver` 解析要求驱动名和 beanID 都非空;否则作为配置错误从 module 返回,而非
  panic。

## 4. 权衡与放弃的方案

- **字节 + codec 收在同一接口,而非独立的 `ByteStore`。** 旧设计有窄接口
  `ByteStore`,经 `FromByteStore` 提升为 `Cache`。现在出厂的后端都是字节原生的,
  把两者合并到 `Cache` 上,少一个类型、少一层适配,且不失通用性。代价——将来若有
  "存活值"的进程内后端就得序列化——可接受(进程内用 bigcache)。
- **codec 放在调用级,而非实例级。** `Get`/`Set` 接受可选 codec,让一个 cache 能
  服务混合类型;默认 JSON,codec 不匹配会在 decode 时显式报错而非静默损坏。实例级
  codec 需要自己的注册表,对 1% 场景属于过度设计,放弃。
- **cache bean 按后端 beanID 命名。** 配置槽(`spring.cache.X`)只是遍历键,bean
  名是 beanID 后缀,把 cache 身份耦到后端 client。有意为之:一个 client → 一个
  cache 名,重复接线在 init 期 panic,而非静默覆盖。
- **本包不做 aspect 桥接。** 旧的 `AsStore` 把本包耦到 `aspect` 及其 `Store` 契约。
  去掉以保持本包聚焦;aspect 适配器若需要,应归 aspect 或调用方。
