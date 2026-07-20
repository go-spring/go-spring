# lock 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`lock` 是 stdlib 层零依赖的分布式锁与选主抽象。真正的后端在 starter 里
(`starter-lock-redis`、`starter-lock-etcd`、`starter-lock-consul`),内置
`MemoryLocker` 保 stdlib 自身可跑测试且无外部依赖。

## 1. 职责与边界

- 回答"这个命名锁我现在能拿吗?"并返回带 fencing token 与 `Lost()` 通道的
  handle。
- 在任意 `Locker` 之上提供统一 `Election`——刻意不走 etcd/consul 原生选主,
  让抽象成为唯一真源,跨后端一份代码。
- 不是防击穿缓存、不是队列、不是二阶段提交。`Unlock` 幂等;竞争要么由
  `TryAcquire` 以 `ok=false` 报,要么以 `Acquire` 的阻塞重试呈现。

## 2. 关键抽象与缝隙

- `Locker` = 缝隙。与 `discovery`、`resilience` 不同,这里**没有全局字符串
  driver 注册表**——锁后端需要活的 client(Redis 连接、etcd client...),而
  非声明式策略。缝隙落在 `Locker` bean 类型:每个 starter 建 client 并导出一
  个 `Locker` bean;切换后端 = blank-import 换包,业务代码零改动。
- `Options`(函数选项,`Apply` 归一)统一 TTL / RenewInterval /
  RetryInterval / Token。`RenewInterval` 特殊值:`0` → TTL/3;负值 → 禁用自
  动续期。
- `Lock.Lost()` 是租约后端必须遵守的契约;临界区必须 select 它,租约失效时立
  即中止。
- `Election` 建立在 `Locker.Acquire` + `Lock.Lost()` 之上:拿到共享 key、以
  term ctx 跑 `OnElected`、watch `Lost()`、cancel 后重新参选。

## 3. 约束(禁止破坏)

- **`Unlock` 幂等**。仅当后端能**证明**锁已被他人接管时,才返回 `ErrNotHeld`;
  释放已释放/过期的锁返回 nil。
- **Fencing token** 是非空必填;`Apply` 未设时填随机 16 字节 hex。下游存储可
  据此拒掉过期持有者的写入。
- **租约生命周期**:TTL 是崩溃持有者影响半径的上限;自动续期维持租约;
  `Lost()` 在续期失败/租约到期时触发。别"永久持锁"——TTL 需仔细选。
- **`Election.OnElected` 接收 term context**——丢主时会被 cancel,必须遵守。
  `Election.Run` 严格顺序:先 cancel,再等 leader 协程结束,再 Unlock,最后重
  新参选。
- **`NewElection` 缺 Locker/Key 直接 panic**——错配的选主永远选不出人,构造
  期 fail-fast。

## 4. 权衡 / 未做的方案

- **不做 driver 注册表**。活 client 不该跨测试/进程重启塞进包级全局 map;那
  套路适合 `resilience`(声明式),但不适合这里。
- **不用 redsync / etcd-election / consul 原生 leader**。`Election` 是
  `Locker` 之上一条路径,跨后端行为一致易推,后续 K8s Lease 也能在同一缝隙下
  加。
- **无可重入锁**。每次 `Acquire` 都发新 fencing token;需要递归请在上层自行
  搭建(通常也不该需要——分布式重入是雷)。
- **暂无 Kubernetes Lease 后端**。已延后;缝隙允许后续无侵入接入。
