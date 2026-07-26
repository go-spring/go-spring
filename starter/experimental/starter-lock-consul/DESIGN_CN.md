# starter-lock-consul 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-lock-consul` 属于 Contributor 形态（`starter/DESIGN.md` §2.3）的集成层
starter：贡献 Consul session + `api.Lock` 后端的 `lock.Locker` 命名 bean。

## 1. 职责与边界

- 把 `spring.lock.<name>` 条目绑定到 Consul 版 `lock.Locker` bean，每条一个，
  按 config 名注册并导出为 `lock.Locker`。
- 每个实例自持 `api.Client`（Consul 未提供 `starter-go-redis` 那种可共享
  client），启动时按 Address/Scheme/Token/TLS 构建。
- `TryAcquire` 走 `LockTryOnce`；`Acquire` 走阻塞的 `Lock()`；`Lost()` 即
  `Lock()` 返回的 channel。

## 2. 关键抽象与缝隙

- **缝隙是 bean 类型。** 无包级 driver 字符串；切换后端是空导入换包。
- **TTL 被 Consul 钳制。** Consul session 要求 TTL 在 `[10s, 86400s]` 范围；
  超范围的配置在启动时钳制，让小于 10s 的配置仍能启动（运行时使用有效 TTL），
  而不是在创建 session 时崩溃。
- **Unlock 吞 `api.ErrLockNotHeld`。** 抽象保证 `Unlock` 幂等；已释放的锁
  再释放不作为调用侧错误抛出。

## 3. 约束

- **`Address` 必填。** 无 localhost 兜底；缺失即在启动被拒绝。
- **`Scheme=https` 不等于 TLS。** `Scheme` 只选 URL 协议；真正启用 TLS 需要
  `TLS.Enabled=true` 及对应 cert/CA 字段。只设 `Scheme=https` 会拨号失败。
- **KeyPrefix 默认 `lock/`。** 多个应用共用一个 Consul 集群时，通过前缀区分
  key 空间，而不是在扁平 key 上撞车。
- **不能对 proxy 跑 `go mod tidy`。** `spring/lock` 是 workspace 本地包。

## 4. 权衡 / 已否决方案

- **自造阻塞循环替代 `api.Lock`——否决。** `api.Lock` 已处理 session 创建、
  keepalive、waiter 释放 channel，且与惯用 Consul 实现方式一致；手写循环
  只是重复。
- **Consul 原生选主 API——否决。** 所有后端的 `lock.Election` 都构建在同一
  `Locker` 抽象之上，让调用侧 API 不因 Consul/etcd/Redis/K8s 选择而变化。
