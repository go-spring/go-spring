# loadbalance
[English](README.md) | [中文](README_CN.md)

`loadbalance` 是 `go-spring.org/spring/discovery` 之上的客户端负载均衡层。
discovery 回答"当下有哪些实例";本包回答"给这份实时集合,该发到哪一个",
并把持续失败的实例摘除。

## 特性

- 五种内置策略(各按稳定名注册):
  - `round_robin` —— 无状态轮询。
  - `least_conn` —— 选择当前在途最少的实例。
  - `consistent_hash` —— FNV-32 环 + 虚拟节点;按 hash key 亲和。
  - `weighted` —— nginx 平滑加权轮询(SWRR)。
  - `zone_aware` —— 本地优先 + 内层 balancer 承载最终选择。
- `Factory` 注册表(`Register` / `New`)—— 策略在 `init` 中自注册,按名切换。
- `Tracker` —— 连续失败阈值 + 半开探测的离群点摘除,按 endpoint 地址键,
  可查询,故 `Pool` 能在路由前主动剔除。
- `Pool` —— 绑定 `EndpointSource`(`discovery.LiveDialer` 直接满足)、
  `Balancer`、可选 `Tracker`;两级过滤(先 `Healthy`、后 `Tracker.Eligible`),
  每级空了都回退输入,绝不黑洞流量。
- 网格模式:`discovery.MeshMode()` 打开时,`Pool.Pick` 降级为单一稳定
  endpoint 且跳过摘除,LB 交给 sidecar。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

```go
import (
    "context"

    "go-spring.org/spring/discovery"
    "go-spring.org/spring/loadbalance"
)

ld, err := discovery.NewClientDialer(ctx, "default", "orders")
if err != nil { return err }
defer ld.Stop()

bal, _ := loadbalance.New(loadbalance.RoundRobin)
tracker := loadbalance.NewTracker(loadbalance.TrackerConfig{
    Threshold: 3,
    EjectFor:  5 * time.Second,
})
pool := loadbalance.NewPool(ld, bal, loadbalance.WithTracker(tracker))

for {
    res, err := pool.Pick(loadbalance.PickInfo{Ctx: ctx})
    if err != nil { return err }
    err = call(res.Endpoint.Addr) // 你的 RPC / HTTP 调用
    if res.Done != nil {
        res.Done(loadbalance.DoneInfo{Err: err})
    }
}
```

按 hash key 或 zone 路由,只需填 `PickInfo`:

```go
res, _ := pool.Pick(loadbalance.PickInfo{Ctx: ctx, HashKey: userID, Zone: "us-east-1a"})
```
