# starter-lock-etcd 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-etcd` 属于 Contributor 形态（`starter/DESIGN.md` §2.3）的集成层
starter：贡献 etcd concurrency session 后端的 `lock.Locker` 命名 bean。

## 1. 职责与边界

- 把 `spring.lock.<name>` 条目绑定到 etcd 版 `lock.Locker` bean，每条一个，
  按 config 名注册并导出为 `lock.Locker`。
- locker 使用 `concurrency.NewSession(WithTTL)` + `concurrency.NewMutex`；
  keepalive 由 session 自身完成，因此不需要手工续期 goroutine。
- 可选 TLS（`TLSConfig{Enabled, CertFile, KeyFile, CACertFile}`），默认关闭。

## 2. 关键抽象与缝隙

- **缝隙是 bean 类型。** 与 `starter-lock-redis` 一致，无包级 driver 字符串；
  切换后端是空导入换包。
- **每次 Acquire 一个 session。** 每次 `Acquire` 都打开新的
  `concurrency.Session`，其 `Done()` 与所获锁的 `Lost()` 一一对应——不同的
  持有相互独立。
- **`ttlSeconds` 归一化。** etcd 拒绝亚秒级 session TTL；config 辅助函数向上
  取整为整秒、最小为 1，以保持抽象层 TTL 契约。

## 3. 约束

- **`Endpoints` 必填。** 空列表在启动被拒绝；没有 localhost 兜底，配置错的
  集群地址不会静默启动。
- **续期由 session keepalive 处理。** 锁不跑手工续期 goroutine——etcd session
  自身有 keepalive；此处解读 `RenewInterval` 会与 session 冲突，该值只服务
  于 consul / redis。
- **TLS 结构与家族一致。** `Enabled` 是开关，`CertFile`/`KeyFile`（mTLS）
  与 `CACertFile`（服务端校验）与其他 starter 保持一致模式。
- **不能对 proxy 跑 `go mod tidy`。** `stdlib/lock` 是 workspace 本地包；
  tidy 会 404。

## 4. 权衡 / 已否决方案

- **复用应用注入的 `clientv3.Client`——本 starter 拒绝。** 与 redis 后端不同
  （后者常复用应用自身的 Redis client），仅用于协调的 etcd 集群十分常见，
  独立配置更友好；锁自己持有并在 destroy 时关闭 client。
- **原生 etcd 选主 API——否决。** 所有后端的 `lock.Election` 都构建在同一
  `Locker` 之上，用户获得统一语义；在这里用 etcd 的 `concurrency.Election`
  会分裂抽象（见 memory `project_task65_lock.md`）。
