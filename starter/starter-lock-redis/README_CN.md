# starter-lock-redis

[English](README.md) | [中文](README_CN.md)

`starter-lock-redis` 为 Go-Spring 应用贡献一个基于 Redis 的
[`lock.Locker`](../../spring/lock) Bean，在已有的 Redis（single / sentinel /
cluster）上提供分布式锁与 Leader 选举，且不额外维护连接。

它属于 *Contributor* 形态（见 [starter/DESIGN.md](../DESIGN.md)）：
Starter 本身不占端口，也不持有自己的客户端；它复用 `starter-go-redis` 已注册的
`*redis.Client`，仅在 `lock.Locker` 这个与框架无关的接缝上贡献一个 Bean。
从 Redis 切换到 etcd/consul 只需要换一个 blank import，业务代码不动。

## 安装

```bash
go get go-spring.org/starter-lock-redis
```

## 快速开始

### 1. 同时引入两个 Starter

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-lock-redis"
)
```

### 2. 先配置一个 Redis 客户端，再配置一个引用它的 Locker

```properties
# 由 starter-go-redis 管理的 Redis 客户端。
spring.go-redis.cache.addr=127.0.0.1:6379

# 绑定到该客户端的 Locker，`client` 是 Redis 实例名。
spring.lock.jobs.client=cache
spring.lock.jobs.ttl=30s
spring.lock.jobs.key-prefix=myapp:
```

`client` 属性是**必填**的。启动时若缺失，Starter 会 fail-fast 直接拒绝启动，
不会静默 fallback 到某个默认实例。

### 3. 注入 `lock.Locker`

```go
import "go-spring.org/spring/lock"

type Service struct {
    Lock lock.Locker `autowire:"jobs"`
}

func (s *Service) RunOnce(ctx context.Context) error {
    held, ok, err := s.Lock.TryAcquire(ctx, "nightly-report")
    if err != nil || !ok {
        return err
    }
    defer held.Unlock(ctx)
    // ...临界区...
    return nil
}
```

## 配置项

所有键都位于 `spring.lock.<name>` 下：

| 键               | 默认值   | 说明                                                                                     |
|------------------|----------|------------------------------------------------------------------------------------------|
| `client`         | —        | **必填。** 位于 `spring.go-redis.<client>` 下的 `*redis.Client` Bean 名。                |
| `ttl`            | `30s`    | 默认租约 TTL。调用方可通过 `lock.WithTTL` 逐次覆盖。                                     |
| `renew-interval` | `0`      | 续租间隔。`0` 表示按 lock 包默认取 `ttl/3`；负值关闭自动续租。                            |
| `retry-interval` | `100ms`  | `Acquire` 在争抢中的轮询间隔。                                                           |
| `key-prefix`     | *空*     | 键名前缀，用于多个应用共享同一 Redis 时隔离命名空间。                                    |

## Leader 选举

在任意 `lock.Locker` 之上通过
[`lock.NewElection`](../../spring/lock/election.go) 即可获得 Leader 选举能力：

```go
el := lock.NewElection(lock.ElectionConfig{
    Locker: s.Lock,
    Key:    "scheduler-leader",
    OnElected: func(ctx context.Context) {
        // 只有 Leader 才会执行的工作；ctx 取消时尽快返回。
    },
})
go el.Run(ctx)
```

由于 Election 是构建在 `Locker` 接口之上的，同一段代码在 Redis、etcd、consul、
或内置的 in-memory 实现上表现一致。

## 保证

* **争抢下的正确性** —— 使用 Redis `SET NX PX` 抢锁；释放走 Lua 的
  compare-and-DEL，租约过期的旧持有者不会误删他人的锁。
* **幂等 `Unlock`** —— 第二次及以后调用是空操作；只有当 Redis 明确证明键
  已被其他 token 持有时，才返回 `lock.ErrNotHeld`。
* **丢失信号** —— 续租循环一旦发现键消失或被他人接管，就关闭 handle 的
  `Lost()` channel，临界区可以据此尽快中止。
* **fail-fast 配置** —— 缺少 `client` 直接启动失败，而不是等到第一次
  `Acquire` 才暴露问题。

## 单 Redis Redlock

本 Starter 实现的是单节点 Redlock。多节点 Redlock 未内置：Sentinel 故障切换
或 Cluster HA 已能覆盖常见场景，如需更强一致性，请换到 etcd 或 consul 版本
（同样是 blank import 切换，代码不动）。
