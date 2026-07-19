# starter-bigcache 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-bigcache` 属于 Client 形态（`starter/DESIGN.md` §2.2），提供
`github.com/allegro/bigcache` 支撑的进程内缓存。因为 bigcache 是纯 Go、
GC 友好的堆内缓存，所以没有网络生命周期——但依然有一个后台驱逐
goroutine，必须在关停时释放。

## 1. 职责与边界

- 用 `gs.Group` 把 `spring.bigcache.instances.<name>` 每条绑到
  `*bigcache.BigCache` bean。不做默认单实例
  （见 `project_client_starter_multiinstance`）。
- 向 `stdlib/cache` 注册名为 `bigcache` 的 driver，让使用
  `cache.Cache` 抽象的调用方（包括 `stdlib/cache` 的 MultiLevel）能按名
  选此后端，无需直接 import bigcache。
- 无跨进程一致性：缓存在本进程堆内。两副本各存一份；这是零跳读的代价。

## 2. 关键抽象与缝隙

- **每实例 `gs.Group`。** 不同配置（LRU/过期、小容量快路径+大容量慢路径）
  可各自作为独立 bigcache 实例互不影响调参。
- **`destroy = Close`。** `LifeWindow > 0` 或 `CleanWindow > 0` 时 bigcache
  会起后台驱逐 goroutine；不 `Close` 就每实例泄一个 goroutine。starter
  接 `destroy` 就为这个（`project_starter_bigcache`）。
- **`AsCache` 适配器接进 driver 注册表。** starter 对 `stdlib/cache` 的
  贡献走 driver 注册表（见 `project_stdlib_cache`），配置里写
  `cache: bigcache` 即可，不用 import bigcache。
- **`check.sh` 不需要 docker。** 进程内缓存无服务容器——冒烟就是
  普通 `go test`。

## 3. 约束

- **容量前置静态。** `Shards` 必须是 2 的幂；`MaxEntriesInWindow` ×
  `MaxEntrySize` 大致框定预分配后备内存。不支持运行时 resize。
- **值类型是 `[]byte`。** 业务类型由调用方编解码；`stdlib/cache`
  MultiLevel 路径通过共享 `ByteStore` 缝隙走 JSON。
- **`LifeWindow` 是全局 TTL 非 per-entry。** 一个实例内所有条目共用 TTL。
  不同 TTL 类=不同命名实例。

## 4. 权衡 / 已否决方案

- **freecache / ristretto——不内置。** 同一定位（进程内缓存）三选一
  本就是进程级决策；只挑一家能让代码基更小。bigcache 因稳定性与 GC
  行为已知良好被选中（`project_starter_bigcache`）。
- **自动 loader / refresh 缓存——否决。** loader 语义属于缓存抽象层
  （`stdlib/cache`），不属于后端 starter。starter 贡献 store，抽象层贡献
  loader/refresh 策略。
