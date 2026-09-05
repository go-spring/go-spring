# lock

[English](README.md) | [中文](README_CN.md)

`lock` 回答多副本部署下的一个问题：*"此刻这个副本能不能执行这段互斥工作？"*——
定时任务、单例后台 worker、一次性迁移，以及任何在同一时刻至多运行一次的逻辑。
它以一个 `Locker` 接口提供分布式锁与选主，后端可以是 Redis / etcd / Consul /
Kubernetes Lease starter，也可以是内置的单进程 `MemoryLocker`。

## 安装

```
go get go-spring.org/cloud
```

## 快速开始：保护临界区

```go
import (
    "context"
    "time"

    "go-spring.org/cloud/lock"
)

locker := lock.NewMemoryLocker()          // 换成任意 starter 提供的 Locker
defer locker.Close()

l, err := locker.Acquire(context.Background(), "jobs/rollup",
    lock.WithTTL(30*time.Second))
if err != nil {
    return err
}
defer l.Unlock(context.Background())

select {
case <-l.Lost():
    // 租约过期或续租失败——中止工作
default:
    // 执行互斥工作
}
```

长时间运行的工作应在循环内持续 select `Lost()`，而不是只检查一次：`Lost()`
是所有基于租约的后端都必须提供的中止信号。

## TryAcquire：竞争时跳过

`Acquire` 在锁被他人持有时阻塞重试。当正确行为是"跳过这次执行"时，用
`TryAcquire`：

```go
l, ok, err := locker.TryAcquire(ctx, "jobs/rollup")
if err != nil {
    return err // 后端故障，区别于普通竞争
}
if !ok {
    return nil // 其他副本持有——跳过
}
defer l.Unlock(ctx)
```

## Lock 句柄

| 成员 | 语义 |
|---|---|
| `Key()` | 该锁守护的资源 key |
| `Token()` | 本次获取的唯一 fencing token；下游存储可用它拒绝过期持有者的写入 |
| `Unlock(ctx)` | 幂等——释放已释放或已过期的锁返回 nil；仅在后端能证明锁被他人接管时返回 `ErrNotHeld` |
| `Lost()` | 租约过期或续租失败时被 close |

每次 `Acquire` 返回新的 fencing token。没有可重入锁——分布式重入是坑，
确有需要请在上层自建。

## Options 与时序优先级

```go
locker.Acquire(ctx, key,
    lock.WithTTL(10*time.Second),          // 租约时长；默认 30s
    lock.WithRenewInterval(3*time.Second), // 默认 TTL/3；负值关闭续租
    lock.WithRetryInterval(200*time.Millisecond), // Acquire 重试节奏；默认 100ms
    lock.WithToken("worker-42"),           // 显式 fencing token；默认随机 16 字节 hex
)
```

时序来自三层，统一由一处解析，所有后端以相同方式组合：

```
单次获取的 Option  >  starter DefaultOptions  >  包默认值
```

starter 的 `spring.lock.<name>.ttl` 是*可覆盖的默认值*：只填调用方未设置的部分，
单次调用的 `WithTTL` 永远优先。负的 `WithRenewInterval`（关闭续租）在分层中
被保留。后端在 `Acquire`/`TryAcquire` 开头调用 `Resolve` 以获得
该优先级；无 starter 级配置的后端传零值 `DefaultOptions` 即可。

慎重选择 TTL：它是持有者崩溃后的最大影响半径。自动续租为存活持有者延续租约；
续租失败时 `Lost()` 触发。

## 选主

`Election` 在任意 `Locker` 之上构建选主——同一份代码无论后端是 Redis、etcd、
Consul 还是内存实现，行为一致。刻意不复用 etcd/consul 原生选主，保证跨后端
行为可推理。

```go
elect := lock.NewElection(lock.ElectionConfig{
    Locker: locker,
    Key:    "leaders/reporter",
    TTL:    15 * time.Second,
    OnElected: func(ctx context.Context) {
        // leader 工作；丢失领导权时 ctx 被取消——请响应它并及时返回
        <-ctx.Done()
    },
    OnResigned: func() { /* 清理 */ },
})

// 阻塞直到 ctx 结束；放到后台 goroutine / 注册为 Runner
err := elect.Run(ctx)
```

- `IsLeader()` 报告当前是否为 leader。
- 丢失租约时：先取消任期 ctx，再等 `OnElected` 返回，然后解锁——顺序固定——
  之后重新竞选。`RetryInterval` 同时决定 follower 重新竞选的节奏。
- `NewElection` 在缺少 `Locker`/`Key` 时 panic：配置错误的选举永远选不出
  leader，直接快速失败。

## 后端

`Locker` 就是接缝。与 `discovery` 不同，这里没有全局字符串注册表，因为锁后端
需要活连接（Redis 连接、etcd client……）而非声明式策略。每个 starter 自建
client 并导出一个 `Locker` bean；业务代码注入 `lock.Locker`，永不改动。

| 后端 | 模块 |
|---|---|
| Redis | `starter-lock-redis` |
| etcd | `starter-lock-etcd` |
| Consul | `starter-lock-consul` |
| Kubernetes Lease (`coordination.k8s.io`) | `starter-lock-k8s` |
| 单进程（测试 / 单节点） | `lock.NewMemoryLocker()` |

切换后端只需换 blank import。K8s 后端除集群控制面外不需要外部中间件，且不
提供 starter 级时序配置——它的旋钮就是单次调用的 `Option`，与其他后端一致。

## 可观测性

`WrapLocker` 用 observe kit 的三信号（trace span + 时长/在途指标 + 访问日志，
`lock.*` 约定）包装任意 `Locker`。starter 统一安装，后端不必各自携带拷贝：

```go
locker = lock.WrapLocker("redis", cfg, inner)
```

未引入 starter-otel 时全局 provider 均为 no-op，包装几乎零开销且不改变行为。

## 编写后端

```go
type myLocker struct{ /* 活连接 */ }

func (b *myLocker) Acquire(ctx context.Context, key string, opts ...lock.Option) (lock.Lock, error) {
    o := lock.Resolve(b.defaults, opts...) // 保住 starter/单次调用优先级
    // 取租约、启动自动续租、返回 Lost() 会在续租失败或过期时 close 的句柄
}
```

必须遵守的契约：

- `Unlock` 幂等；仅在能证明被接管时返回 `ErrNotHeld`。
- fencing token 必填且非空。
- 租约消失时必须 close `Lost()`——临界区的中止信号。
- `Locker` 与 `Lock` 并发安全；`Close` 释放后端资源，不释放已发出的锁。

## 示例

可运行、自校验的演示见 [`example/`](example/)：

```
cd cloud/lock/example && go run .
```

它演示 `Acquire`/`TryAcquire` 竞争、fencing token、租约被接管时的 `Lost()`，以及一次
leader 任期交接——全部跑在 `MemoryLocker` 上，无需外部服务。
