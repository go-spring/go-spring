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

backend := discovery.NewStaticDiscovery(
    discovery.Endpoint{Host: "10.0.0.1", Port: 8080},
    discovery.Endpoint{Host: "10.0.0.2", Port: 8080},
)
resolver, err := discovery.NewResolver(ctx, backend, "orders")
if err != nil { return err }

bal := loadbalance.NewRoundRobin()
pool := loadbalance.NewPool(resolver, bal, loadbalance.WithTrackerConfig(loadbalance.TrackerConfig{
    Threshold:  3,               // 连续失败 3 次摘除
    SuspendFor: 5 * time.Second, // 摘除 5s 后半开试探
}))

for {
    ep, err := pool.Pick(loadbalance.PickInfo{})
    if err != nil { return err }
    err = call(ep.Addr)     // 你的 RPC / HTTP 调用
    pool.Complete(ep, err)  // 必须配对:销账 + 喂摘除器
}
```

## Pool:组装与过滤

`Pool` 把三样东西粘成运行时:一个端点来源、一个策略、属于它自己的 `Tracker`
(构造即为禁用态,无需任何接线就可被治理;初始阈值用 `WithTrackerConfig` 设定)。

```go
pool := loadbalance.NewPool(resolver, bal)
```

- **端点来源**是任何实现 `Endpoints() ([]discovery.Endpoint, error)` 的对象,
  直接传 discovery `Resolver`——新鲜度全在
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

自定义策略实现 `Balancer`(Pick/Complete,须并发安全),再作为一个命名
`Factory` bean 贡献给容器——bean 名即规则引用的策略名,策略自己的参数从
`Params` 里读:

```go
type myFactory struct{}

func (myFactory) Build(_ loadbalance.Directory, p *loadbalance.Params) (loadbalance.Balancer, error) {
    window, err := p.Duration("window", time.Second) // 策略自己的参数
    if err != nil {
        return nil, err
    }
    if err := p.Done(); err != nil { // 拒绝本策略不认识的键
        return nil, err
    }
    return &myBalancer{window: window}, nil
}

// starter 的 init 里:
gs.Provide(func() loadbalance.Factory { return myFactory{} }).
    Name("my_strategy").
    Export(gs.As[loadbalance.Factory]()).Caller(1)
```

核心不认识任何策略参数——`Params` 就是规则里的扁平 `balancer-params` 映射,
策略读自己拥有的键、拒绝其余。新增一个策略及其专属参数,本包一行都不用改。

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
`Threshold <= 0`(不传 `WithTrackerConfig` 时的默认值)时完全透明,零开销。

## 受管选择(治理)

池的策略与摘除阈值可以从进程外驱动:按**服务标签**而不是构造期参数传进来。调用方注入
`*loadbalance.Manager` bean(见 [manager.go](manager.go)),把池交给它:

```go
// mgr 是注入进来的 *loadbalance.Manager;nil 表示进程里没有治理
stop := mgr.Bind(pool, "http:user-svc")
defer stop()
```

- `Bind(pool, label)` **立刻**应用该标签当前的 `Selection`,之后每次变更再应用一次,
  全部原地生效——不重建、不重连,下一次 `Pick` 就走新策略。它返回解绑函数:生命周期
  短于进程的池**必须**调用,否则 Manager 会留着一个指向已死池的回调。
- 给**尚未武装**的 Manager 绑定是安全的,而且是容器接线期建池的常态:订阅会被记住,
  等第一次 `Apply` 时自动武装。完全没有注入 Manager 时(容器里没有 starter-governance,
  或独立调用方)池保持构造时的策略——透明旁路。
- 策略名留空 = 保持当前策略;策略名写错 = **被忽略**,沿用上一个可用策略。摘除阈值
  总是应用。
- 摘除那一半只在 `Pick` 配对了 `Complete` 的池上才有效果——`Tracker` 一直在,
  但没有配对就没有成败可计,阈值就是设在了空气上。

`Manager.Apply(Settings{...})` 是治理中心唯一的入口——启动时用来源快照调一次,之后每次
推送再调;`Manager.SelectionFor(label)` 读回某标签当前解析出的选择。

规则给出策略名,并把策略自己的参数放进一个对本包不透明的扁平子映射:

```yaml
balancer: consistent_hash
balancer-params:
  replicas: 200
outlier-threshold: 5
```

`Bind` 是"策略名变成策略"的唯一位置:它拿名字去 Manager 的 `Directory`——内置策略
加上容器贡献的 `Factory` bean——解析出一个已构造的 `Balancer` 交给池,所以池永远不持有
工厂表。`Pool.ApplyBalancer`(策略)与 `Pool.ApplySuspension`(阈值)是同样两半的直接
入口,`Pool.Selection()` 可读回最近一次被接受的选择——自己管配置、不经 Manager 时用得上。

## Pick/Complete 契约

两段必须**恰好配对一次**。漏调 `Complete` 的后果: `least_conn` 在途计数
泄漏(该实例被饿死)、`p2c` 延迟模型失真、`Tracker` 收不到成败信号(摘除
失效)。
