# ctxcache 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`ctxcache` 借助 `context.Context`，为短生命周期
的请求作用域数据提供一个"带类型、写一次、边界内自动清理"的容器，避免把
这类数据加到函数签名里。

## 1. 职责与边界

- 承担"一次请求、一袋带类型的值、在边界清理"的模式。所有数据挂在具体的
  context 上，没有进程级全局 map。
- 不是"TTL / 淘汰 / 命中率"意义上的缓存。它是一个作用域受控、类型明确的
  小型 key/value 存储，其生命周期正好是外层请求。

## 2. 关键抽象

- **`Cache`**：通过私有 `cacheKey` 挂在 context 上的受互斥锁保护的
  `map[any]any`。每个 context 只允许一个 Cache；对同一 context 再调用
  `Init` 是幂等操作。
- **`TypedKey[T]`**：key 是 `(string, 类型)` 对，由泛型生成。名字相同但 `T`
  不同的键彼此不相干 —— 这也是 `Get`/`Set` 必须是泛型而不是 `any` 的原因。
- **显式生命周期**：`Init` 返回一个 `cancel` 函数。调用它会清空 map 并把
  Cache 永久标记为已清理，之后的 `Get`/`Set` 都返回 `ErrCacheAlreadyCleared`。
  有意不做"第二生命" —— 复用会静默地在请求间共享状态。

## 3. 约束与取舍

- Key 写一次。使得"存在的值不会被别人改掉"成为总契约。需要可变性的调用方
  应存指针，或用稳定 key 存 `sync.Map`。
- cancel 必须调用 —— 通常在中间件边界 `defer`。漏调只会泄漏一个小 map，
  不会泄漏 goroutine。
- 并发安全。请求作用域数据大多顺序流转，锁竞争极少，单互斥锁是最简单
  正确的选择。
- 错误类型是通过 `errutil.Explain(nil, ...)` 构造的 sentinel 变量，
  请使用 `errors.Is` 判定。
