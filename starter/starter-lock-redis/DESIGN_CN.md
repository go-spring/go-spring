# starter-lock-redis 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-redis` 属于 Contributor 形态（`starter/DESIGN.md` §2.3）的集成层
starter：贡献 Redis 后端的 `lock.Locker` 命名 bean；不开监听、不持有连接，复用
`starter-go-redis` 注册的 `*redis.Client`。

## 1. 职责与边界

- 把 `spring.lock.<name>` 条目绑定到 Redis 版 `lock.Locker` bean，每条一个，
  按 config 名注册并导出为 `lock.Locker`。
- `Locker` 使用 Redis `SET NX PX` + Lua 释放 / 续期实现，因此**不需要**引入
  额外库（如 `redsync`）。
- **不拥有** Redis 客户端生命周期：destroy 只停自己起的续期 goroutine；
  `*redis.Client` 由 `starter-go-redis` 关闭。

## 2. 关键抽象与缝隙

- **缝隙是 bean 类型，不是 driver 字符串。** `stdlib/lock` 有意**不**设置
  包级字符串 driver 注册表，与 `stdlib/discovery`、`stdlib/resilience` 不同。
  锁需要**活的**后端句柄（`*redis.Client`），而不是声明式策略；换 Redis 为
  etcd/consul/k8s 是空导入换包，改变的是哪个 starter 注册 `lock.Locker` bean。
- **实例与客户端的绑定走 `TagArg`。** `Config.Client` 字段是
  `spring.go-redis.<Client>` 下的 `*redis.Client` bean 名。starter 通过
  `gs.TagArg(c.Client)` 调用 provide 构造器——这就是把某个 `Locker` 绑到
  特定 Redis 实例的缝隙。
- **锁后端共用配置前缀。** 所有 lock starter 都落在 `spring.lock.<name>`
  （`starter/DESIGN.md` §3），业务代码按名注入 `lock.Locker`，切换后端时
  无需改动。

## 3. 约束

- **`Client` 必填。** `spring.lock.<name>.client` 为空时 `errutil.Explain`
  在启动直接拒绝。静默默认到某个 Redis 实例只会把配置错误藏到第一次
  `Acquire`，风险太大。
- **destroy 停续期不停 client。** `destroyLocker` 调 `Locker.Close()`，停每个
  持有锁的续期 goroutine；从不 `Close` 注入的 `*redis.Client`——该连接可能
  还在被其他 bean 使用。
- **`Locker` API 属于抽象层。** 可调项（TTL、RenewInterval、RetryInterval、
  Token）都通过 `lock.Option` 传入；config 里只提供默认值。`KeyPrefix` 用于
  隔离 key 空间，让多个应用共用一个 Redis 集群。

## 4. 权衡 / 已否决方案

- **`redsync`——否决。** 手写 `SET NX PX` + compare-and-DEL Lua 大约 50 行，
  依赖面与 `starter-go-redis` 一致。
- **自动探测 `*redis.Client` bean——否决。** 显式 `client=` 让绑定关系一目
  了然；一旦应用跑多个 Redis 实例自动探测立刻失效。
- **在 `spring.lock.<name>` 内塞 Redis 配置——否决。** 复用现有
  `*redis.Client` bean 意味着共享集群 / 独立集群切换在 Redis 侧的配置变更
  即可完成。
