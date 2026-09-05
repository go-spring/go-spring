# loadbalance

[English](README.md) | [中文](README_CN.md)

`loadbalance` 是 `go-spring.org/cloud/discovery` 之上的客户端负载均衡层。
discovery 回答"当下有哪些实例";本包回答"给这份实时集合,该发到哪一个",
并把持续失败的实例摘除。

## 安装

```
go get go-spring.org/cloud
```

## 快速开始

```go
import (
    "context"
    "time"

    "go-spring.org/cloud/discovery"
    "go-spring.org/cloud/loadbalance"
)

resolver, err := discovery.NewResolver(ctx, "default", "orders")
if err != nil { return err }

bal, _ := loadbalance.New(loadbalance.RoundRobin)
tracker := loadbalance.NewTracker(loadbalance.TrackerConfig{
    Threshold:  3,               // 连续失败 3 次摘除
    SuspendFor: 5 * time.Second, // 摘除 5s 后半开试探
})
pool := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal, loadbalance.WithTracker(tracker))

for {
    ep, err := pool.Pick(loadbalance.PickInfo{})
    if err != nil { return err }
    err = call(ep.Addr)     // 你的 RPC / HTTP 调用
    pool.Complete(ep, err)  // 必须配对:销账 + 喂摘除器
}
```

## Pool:组装与过滤

`Pool` 把三样东西粘成运行时:一个端点来源、一个策略、一个可选的 `Tracker`。

```go
pool := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal, loadbalance.WithTracker(tracker))
```

- **端点来源**是任何实现 `Endpoints() ([]discovery.Endpoint, error)` 的对象,
  通过 `loadbalance.SourceFunc(resolver)` 接入 discovery `Resolver`——新鲜度全在
  discovery 后端内部,每次 `Pick` 都重读最新快照。
- 每次 `Pick` 依次过滤:**discovery 资格**(禁用/不健康的实例)→ **摘除**
  (`Tracker` 冷却中的实例)→ **零权重摘流**(权重为 0 的实例),幸存者交给
  策略挑选。每级过滤都保证不把非空集合滤成空集——绝不黑洞流量。
- **网格模式**(`discovery.MeshMode()` 开启)下自动降级为单一稳定 endpoint,
  LB 交给 sidecar,无需改代码。

## Balancer:策略

策略决定"从幸存者里选谁"。七种内置,按稳定名注册,`New` 按名取用;
自定义策略用 `Register` 注册。

| 场景 | 策略 | 理由 |
|---|---|---|
| 实例同质、无特殊需求 | `round_robin` | 最简单,零状态 |
| 实例性能不均(磁盘慢、GC 频繁) | `least_conn` 或 `p2c` | 前者按在途数自适应,后者按实测延迟自适应 |
| 请求时延差异大(如导出类接口) | `p2c` | 在途数是滞后指标,p2c 的 EWMA 区分"忙"和"慢" |
| 需要会话/缓存亲和 | `consistent_hash` | 同 key 稳定落同实例,扩缩容只迁移少数 key |
| 实例配置不同(4C8G vs 8C16G) | `weighted` | 按能力配比,SWRR 平滑打散 |
| 跨机房/可用区部署 | `zone_aware` | 本地优先,逐级回退,省跨区延迟与流量费 |
| 超高并发、无状态可以接受 | `random` | 无共享游标,无原子竞争热点 |

策略分两类:**无状态**(round_robin/weighted/consistent_hash/random,`Complete`
是 no-op)和**有状态**(least_conn 维护在途表;p2c 维护延迟模型)。所有策略
状态按 endpoint 地址键,实例增删、快照重排都不影响幸存实例的存量状态。

自定义策略实现 `Balancer` 接口后注册即可,与内置策略同等可用:

```go
loadbalance.Register("my_strategy", func() loadbalance.Balancer {
    return &myBalancer{} // 实现 Pick 和 Complete;须并发安全
})
```

重试场景下,每次重试重新 `Pick` 即可拿到新鲜实例——候选集每次都重新过滤,
不需要(也没有)失败端点黑名单接口;持续失败由 `Tracker` 自动摘除。

## PickInfo:路由提示

`Pick` 的第二个参数携带这次请求的路由依据,全部可选,无状态策略直接忽略:

```go
// 按 hash key 亲和(consistent_hash 消费)
ep, _ := pool.Pick(loadbalance.PickInfo{HashKey: userID})

// 按 zone 就近(zone_aware 消费);接受有序回退列表,逐级下探,全空才溢出
ep, _ := pool.Pick(loadbalance.PickInfo{Zone: "us-east-1a,us-east-1"})
```

## Tracker:离群摘除

`Tracker` 解决 discovery 看不到的故障形态:实例还注册着、健康检查也过,但
实际请求持续失败(僵尸实例)。它只从 `Complete(err)` 学习,无需额外调用:

```
正常 --连续失败达 Threshold--> 摘除(SuspendFor 冷却,Pick 不再选中)
                                   |
                              冷却到点 ↓ 半开试探(放一笔进来)
                          成功 → 状态清零,恢复服务
                          失败 → 重新摘除,循环
```

一笔成功即清零失败计数,偶发失败不触发摘除;所有实例都被摘除时回退全量。
`Threshold <= 0`(或不挂 `WithTracker`)时完全透明,零开销。

## Pick/Complete 契约

两段必须**恰好配对一次**。漏调 `Complete` 的后果: `least_conn` 在途计数
泄漏(该实例被饿死)、`p2c` 延迟模型失真、`Tracker` 收不到成败信号(摘除
失效)。
