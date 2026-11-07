# loadbalance

[English](README.md) | [中文](README_CN.md)

`loadbalance` 是 `go-spring.org/cloud/discovery` 之上的客户端负载均衡层。
discovery 回答"当下有哪些实例"；本包回答"给这份实时集合，该发到哪一个",
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
    "go-spring.org/cloud/governance/capability/loadbalance"
)

backend := discovery.NewStaticDiscovery(
    discovery.Endpoint{Addr: "10.0.0.1:8080", Scheme: "tcp"},
    discovery.Endpoint{Addr: "10.0.0.2:8080", Scheme: "tcp"},
)
resolver, err := discovery.NewResolver(ctx, backend, "orders", "")
if err != nil { return err }

bal := loadbalance.NewRoundRobin(loadbalance.Selection{})
pool := loadbalance.NewPool(resolver, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{
    Threshold:  3,               // 连续失败 3 次摘除
    SuspendFor: 5 * time.Second, // 摘除 5s 后半开
}), loadbalance.Selection{})

for {
    pick, err := pool.Pick(ctx, loadbalance.PickInfo{})
    if err != nil { return err }
    err = call(pick.Endpoint.Addr) // 你的 RPC / HTTP 调用
    pick.Complete(ctx, err)        // 必须配对：结算到选出它的那一代 + 喂摘除器
}
```

## Pool：组装与过滤

`Pool` 把三样东西粘成运行时：一个端点来源、一个策略、一个 `Tracker`——三样都由调用方
构造，池自己不创建任何东西。零值 `TrackerConfig` 建出的 tracker 即"不摘除"、完全透明；
写了阈值就无需任何额外接线即可被治理。

```go
pool := loadbalance.NewPool(resolver, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{}), loadbalance.Selection{})
```

- **端点来源**就是一个 `discovery.Resolver`——`func(ctx context.Context) ([]discovery.Endpoint, error)`。
  新鲜度全在 discovery 后端内部，每次 `Pick` 都重读最新快照。
- 每次 `Pick` 依次过滤：**selector**（治理动态收窄）→ **discovery 资格**
  （禁用/不健康的实例）→ **摘除**（`Tracker` 冷却中的实例）→ **零权重摘流**，
  幸存者交给策略挑选。selector 与 disabled 是硬过滤：匹配不到或全部禁用时返回
  `ErrNoEndpoints`，不兜底（灰度规则不能被悄悄放宽）；摘除和摘流在会清空集合时跳过，
  回退到过滤前集合——绝不黑洞流量。
- **网格模式**：根本不建池——client entry 问一次 `Runtime.UsesDiscovery`，得到
  否，就直连配置的稳定地址，LB 交给 sidecar。`loadbalance` 本身零 mesh 引用。

## Balancer：策略

策略决定"从幸存者里选谁"。六种内置；治理规则按名引用其中之一，
任何链接进来的包注册的策略也可以按同样的方式被引用。另有 `zone-aware` 开关，
它不占策略名，叠加在所选策略之上。

| 场景 | 策略 | 理由 |
|---|---|---|
| 实例同质、无特殊需求 | `round_robin` | 最简单，零状态 |
| 实例性能不均（磁盘慢、GC 频繁） | `least_conn` 或 `p2c` | 前者按在途数自适应，后者按实测延迟自适应 |
| 请求时延差异大（如导出类接口） | `p2c` | 在途数是滞后指标，p2c 的 EWMA 区分"忙"和"慢" |
| 需要会话/缓存亲和 | `consistent_hash` | 同 key 稳定落同实例，扩缩容只迁移少数 key |
| 实例配置不同（4C8G vs 8C16G） | `weighted` | 按能力配比，SWRR 平滑打散 |
| 跨机房/可用区部署 | `zone-aware` 开关 | 叠加在所选策略之上：本地优先，逐级回退，省跨区延迟与流量费 |
| 超高并发、无状态可以接受 | `random` | 无共享游标，无原子竞争热点 |

策略分两类：**无每请求状态**（`round_robin` 只有一个原子游标、`random` 完全无状态，
两者 `Complete` 都是 no-op）和**有状态**（`weighted` 维护平滑轮询的 current weight；
`least_conn` 维护在途表；`p2c` 维护延迟模型；`consistent_hash` 缓存哈希环）。所有状态
按 endpoint 地址键，实例增删、快照重排都不影响幸存实例的存量状态。

自定义策略实现 `Balancer`（Pick/Complete，须并发安全；`Complete` 收本次所结
请求的 ctx），再提供一个 `Provider`——注册名即规则引用的策略名。工厂收到整份
`Selection`，贡献策略从 `Selection.Params`（规则里的 `balancer-params` 扩展
子映射）读自己拥有的键、拒绝其余：

```go
func newMyStrategy(sel loadbalance.Selection) (loadbalance.Balancer, error) {
    window := time.Second // 策略自己的参数，缺省值在此声明
    if v, ok := sel.Params["window"]; ok {
        w, err := time.ParseDuration(v)
        if err != nil {
            return nil, err
        }
        window = w
    }
    for k := range sel.Params { // 拒绝本策略不认识的键
        if k != "window" {
            return nil, fmt.Errorf("my_strategy: unknown parameter %q", k)
        }
    }
    return &myBalancer{window: window}, nil
}

// 定义该策略的包的 starter 里：
gs.Provide(func() loadbalance.Provider {
    return myStrategyProvider{} // Strategies() = []string{"my_strategy"}
}).Name("my_strategy")
```

策略贡献一个 `loadbalance.Provider` bean。manager 把所有 Provider bean（内置集合 +
各贡献）收集成一个 `Factory`，没有全局注册表，也没有 init 副作用。

内置策略的参数平铺在 `Selection` 顶层，用策略前缀标明归属（如
`consistent-hash-replicas`）；别的策略不读它，配置类型或取值越界会被文档校验
整体拒绝——参数指向错了从不静默丢弃。zone-aware 不是
策略而是叠加过滤器：`zone-aware: true` 打开、`zone-key` 指定匹配键，作用于同一
份规则选出的任意策略，因此策略名只需写一次。贡献策略的参数走 `Selection.Params`
扩展子映射，键由策略自己解释、自己拒绝。新增一个贡献策略及其专属参数，本包
一行都不用改。

重试场景下，每次重试重新 `Pick` 即可拿到新鲜实例——候选集每次都重新过滤，
不需要（也没有）失败端点黑名单接口；持续失败由 `Tracker` 自动摘除。

## PickInfo：路由提示

`Pick` 的第二个参数携带这次请求的路由依据，全部可选；不理解它的策略直接忽略：

```go
// 按 hash key 亲和（consistent_hash 消费）
pick, _ := pool.Pick(ctx, loadbalance.PickInfo{HashKey: userID})
pick.Complete(ctx, err)

// 按 zone 就近（zone-aware 过滤器消费）；接受有序回退列表，逐级下探，全空才溢出
pick, _ = pool.Pick(ctx, loadbalance.PickInfo{Zone: "us-east-1a,us-east-1"})
```

## Tracker：离群摘除

`Tracker` 解决 discovery 看不到的故障形态：实例还注册着、健康检查也过，但
实际请求持续失败（僵尸实例）。它只从传给 `Complete` 的 `err` 学习，无需额外调用：

```
正常 --连续失败达 Threshold--> 摘除（SuspendFor 冷却，Pick 不再选中）
                                   |
                              冷却到点 ↓ 半开（重新放行流量）
                          首个成功 → 状态清零，恢复服务
                          任一失败 → 重新摘除，冷却重启
```

一笔成功即清零失败计数，偶发失败不触发摘除；所有实例都被摘除时回退全量。
冷却期内的结果（挂起前放行的请求）会被忽略——只有半开阶段的结果决定恢复。
`Threshold <= 0`（由零值 `TrackerConfig` 建出的 tracker）时完全透明，零开销。

## 受管选择（治理）

链的策略与摘除阈值可以从进程外驱动：按**服务**而不是构造期参数传进来。调用方注入
`*loadbalance.Manager` bean（见 [manager.go](manager.go)），在它下面建链：

```go
// mgr 是注入进来的 *loadbalance.Manager;nil 表示进程里没有治理
var pick loadbalance.PickResult
if mgr != nil {
	chain, err := mgr.Chain(ctx, resolver, "http:user-svc") // 跟随该 service 的 Selection
	if err != nil {
		return err
	}
	pick, err = chain.Pick(ctx, loadbalance.PickInfo{})
	if err != nil {
		return err
	}
} else {
	bal := loadbalance.NewRoundRobin(loadbalance.Selection{})
	pool := loadbalance.NewPool(resolver, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{}), loadbalance.Selection{}) // 独立使用
	pick, err = pool.Pick(ctx, loadbalance.PickInfo{})
	if err != nil {
		return err
	}
}
err := call(pick.Endpoint.Addr)
pick.Complete(ctx, err) // 结算到选出它的那一代
```

- `mgr.Chain(resolver, service)` 在该 service 下把链建出来，于是它当前的 `Selection` **立刻**
  作用在它建出的这一代上，之后的变更**整条重建**——全新的 balancer、tracker、池，
  与它替换掉的那一代毫无共享。重建发生在**推送上**、不在请求路径上：
  `Manager.Apply` 在治理源的单一 loop 里遍历注册的链，service 解析出的
  `Selection` 与上一次**按值比较**，变了就当场重建换入。请求路径对刷新的
  全部感知是一次 atomic load。`Chain` 是唯一对外出口：请求路径用 `chain.Pick`，
  再用返回的 `pick.Complete` 结算，内层 `Pool` 不对外暴露。一个 service 在进程里
  **只允许一条链**：同名第二次 `Chain` 注册直接返回错误（两个客户端共用一个 label
  会分叉选择状态，且只有第一个 source 会被用到）。注册发生在接线期（先于源 loop
  启动），没有解绑，全部引用由 `Manager.Close` 在 Manager 生命结束时统一释放。
- 在**尚未武装**的 Manager 下建链是安全的，而且是容器接线期建链的常态：先建出基础代，
  文档到达后的推送会当场把它换过去。完全没有 Manager 时（容器里没有
  策略根的 bean,或独立调用方）链只建一次、
  永不重建——透明旁路。
- 解析出来的 `Selection` 必须点名策略：未点名时它会被**整体**替换为基础 selection
  （`round_robin`），同一份 selection 上的阈值、selector 等字段也随之失效。基础策略由
  生产 settings 的一方明确写上（Manager 的基础文档、治理侧翻译）。策略名写错 = 整份
  文档被拒绝，沿用上一代配置——重建没有"当前策略"可以继承。摘除阈值总是应用。
- 摘除那一半只在 `Pick` 配对了 `Complete` 的池上才有效果——`Tracker` 一直在，
  但没有配对就没有成败可计，阈值就是设在了空气上。

`Manager.Apply(Settings{...})` 是治理中心唯一的入口——启动时用来源快照调一次，之后每次
推送再调。中心只**推**规则、匹配由本包做——默认 `Selection`（嵌在文档根上）加一份 `Rules` 列表，首条
`Service` 等于该 service 的 Rule 胜出，和 `fault` 早就是同一副形态。Apply 到此为止：它把文档作为新的一代
发布出去，再逐条链比对重建。变更检测是**按值**的：内容未变的重复推送对链是零成本，
重复 Apply 同一份文档不算变更；每次真正动了 service 的推送当场重建该 service 的链。

规则给出策略名，内置策略的参数是平铺的、带策略前缀的 key（`<strategy>-<param>`）；
`balancer-params` 是留给贡献策略的扩展缝：

```yaml
balancer: consistent_hash
consistent-hash-replicas: 200
outlier-threshold: 5
selector:                 # 候选集收窄，发生在策略挑点之前
  version:
    prefix: v2.
```

selector 是端点收窄**动态的一半**：由 `Pool.Pick` 用 `discovery.FilterByMetadata`
作为第一个过滤阶段施加——先读源、再过滤、然后才做 eligibility；它**不**包装 resolver，
源因此是链在代际之间稳定的目标。它能和 resolver 构造期传入的 scheme 已经施加的静态收窄叠加
（两者都必须成立）；一次推送和任何选择变更一样，推动该 service 的代际。这里**不兜底**——selector
匹配不到任何端点时直接 `ErrNoEndpoints`，因为隐式兜底会把一条灰度规则悄悄放宽到全量实例。

`Factory.Create` 是"策略名变成策略"的唯一位置：manager 在每个新代际构建时调用它，
所以池永远不持有工厂表。`Chain.Selection()` 可读回当前生效那一代**构建时**用的选择——
仅供检查，不在请求路径上。

## 覆盖范围与「会重新挑端点吗」自查

选路**只**走治理动态通道——`balancer` 名 + 其带前缀的参数（`consistent-hash-replicas`）、
`zone-aware` 开关与 `zone-key`，或贡献策略的 `balancer-params`，经 `Manager.Apply` →
重建出的 `Chain` 新一代（热更新，但整条重建）。zone-aware 不是策略而是叠加过滤器：
它作用于同一份规则选出的任意策略，策略名只需写一次。starter 不新增自己的本地静态
`balancer` 键；各链的基础代直接是 round_robin（不走注册表查名，无 error）。
`Provider` 参数化保留，服务于治理通道递参。

覆盖到的客户端：所有走发现模式的 `loadbalance.Pool` 消费者——`httpx`/`gateway`、gorm
四方言、`redigo`、`go-redis`、`mongodb`、`grpc`。

> 自查：**这条 client 每次请求/每次建连会重新挑端点吗？** 会 → 它的服务就能配
> `balancer`；不会 → 别指望 `spring.governance.client.loadbalance.services[N].balancer` 对它生效。

「不会」的每种，都是有意为之：

- **直连**（固定 addr/host）：没有候选集可选。
- **只挑一次**（`neo4j`）：host 固化进启动 URI，没有「每次挑选」可管（治理只到保护策略）。
- **成熟库自持选择**（`elasticsearch` / `memcached` / MQ 族 / `s3`）：选择权在库内部
  （ES 的节点选择器、memcached 的按 key 一致性哈希）。判据：**库里已经有成熟选择器时
  不自己造一个**，只把实时地址喂给它。

**DB/缓存类客户端剔除粒度是「连接」而非「查询」。** gorm / redigo / go-redis / mongodb
的挑选发生在建连时，能喂给 `Tracker` 的成败信号只有 dial 结果本身；单条查询失败归各自
的 resilience executor 管，不参与点数。

**`grpc` 在进程级服务下挑点。** 它的选点身份是进程级 `grpc:client`，配 `balancer=` 会
同时改掉所有内置 `gs_*`；要按服务隔离，就用 `RegisterBalancer` 注册自定义名字（自定义
名字保留自己的策略，按设计豁免进程级覆盖）。

**建池三件套** = `Tracker` + 建连处 `Pick`/`Complete` 配对 + 订阅治理下发的
`Selection`；新增 client 照做（已写进 `starter/DESIGN.md` §4 检查项）。一个 service
在一个进程里只允许一条链：同名第二次 `Manager.Chain` 直接返回错误，客户端必须保证
同一个治理 label 只建一次链，而不是让多个池各自注册同名订阅。

## 权重语义

- `Endpoint.Weight == 0` = 摘流（drain）。（2026-08-27）
- `weighted` 策略把单个端点的权重钳制在 `1<<20`：权重来自发现后端且没有上界，
  两个极端值会让平滑轮询的累加溢出、饿死其他实例。
- 消费端所有 LB 策略统一剔除 Weight==0 端点（健康/tracker 过滤后），全零时回退过滤前
  集合（防黑洞）；**负权重**保留在轮换。
- 写侧 registrar 的 Register 把 weight ≤0 归一为 1（"未配置"永不落 0）；`UpdateWeight(0)`
  是显式摘流、允许落 0。别把 0 归一成 1——那是写侧 Register 的职责。
- **摘流与注销（Disabled）是两回事**：前者零损失轮转，后者立即移除。

## 地址新鲜度：独立的一条轴

谁挑节点，与地址集跟不跟命名服务是两件独立的事。覆盖到的客户端每次建连/每请求重读；
库自持选择的那些装自己的活集：ES 装 `ConnectionPoolFunc`、memcached 装 `ServerSelector`
（保持 CRC32-of-key 哈希 + 地址排序）、neo4j 装 driver 的 `AddressResolver`（仅 `neo4j://`
路由模式）。读失败时各自保留上一份可用集，**绝不返回空集**。

## Pick/Complete 契约

`Pick` 返回 `PickResult`（端点 + 作出选择的那一代），必须用 `pick.Complete(ctx, err)`
结算：结果记到**选出该端点的那一代**，所以 Pick 与 Complete 之间发生治理重建也不会
串到新代际；被替换掉的那一代会丢弃它的在途结果。

两段必须**恰好配对一次**。漏调 `Complete` 的后果：`least_conn` 在途计数泄漏（该实例
被饿死）、`p2c` 的在途记录不释放（且清理会跳过有在途的地址）、`Tracker` 收不到成败
信号（摘除失效）。

`Complete` 收本次请求的 ctx：策略自己的遥测与日志借此挂到引发状态变化的那次请求上
（`Tracker` 的摘除、恢复两条日志因此能串进链路）。它做的是内存记账——不得发起 I/O，
也不得依赖这个 ctx 仍然存活。
