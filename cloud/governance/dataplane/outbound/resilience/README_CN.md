# resilience

[English](README.md) | [中文](README_CN.md)

`resilience` 是框架无关的**客户端**容错抽象：限流、熔断、
重试、超时、降级。客户端 starter 把单一 `chain.Executor` seam 插入自家请求
钩子（HTTP RoundTripper / Redis Hook / GORM plugin ...）。

它是**纯出站**的。入站请求走 [`cloud/governance/dataplane/inbound/admission`](../../inbound/admission/)：每实例一个 `Guard`、
两阶段 `Admit` + `Done`、自己的策略词汇和默认引擎。旧的入站半边
（`Manager` / `ServerEngine` / `ServerPolicy` / `Driver.NewServerExecutor`）
已经删除；出站的后端缝（`Driver`）也一并删除，manager 自己持有唯一的引擎。

## 特性

- `Policy` 字段（出站）：`RateLimit` / `Burst` / `Limiter` / `Window` /
  `RateLimitMaxWait` / `RateLimitWarmUpDuration` / `WarmUpColdFactor`、
  `ErrorThreshold` / `OpenDuration` / `BreakerStrategy`
  （`consecutive` | `error-rate` | `slow-call` | `error-count`）/
  `ErrorRateThreshold` / 慢调用阈值对 / `HalfOpenRequests`、
  `Retry`（`resilience.Retry` 节点）、`Timeout`。
- 中立拒绝错误：`ErrRateLimited`、`ErrCircuitOpen`、`ErrBulkheadFull`。
- 内置 `"default"` 驱动 —— 进程内、零依赖。推荐的生产驱动 `sentinel` 在
  `starter/starter-governance-sentinel`。
- 一个客户端适配 seam：`NewRoundTripper` —— HTTP client `http.RoundTripper`。
  （曾经还有一个连接级 `DialFunc` seam，已退回唯一需要它的 starter —— 见 §2；
  请求层才是那四个策略名下名副其实的地方。）
- 入站 admission 不在本包：各协议 starter 走 `admission.Guard.Admit` 自建中间件（拒绝时
  429 / 503；见 starter-gin / starter-grpc 的 admission）。治理中心把两个方向各自独立的
  文档分别推给 outbound 的 `Manager` 与 inbound 的 `admission.Manager`，所以出站策略变更
  永远不会顺手调了入站准入。
- 限流是受保护调用里的一个 stage，不是独立 API：计数器就是 executor 为自身
  服务持有的预算，交给它一份 `Limiter` 存储（`starter-go-redis` 的 Redis）
  则跨副本共用一份预算。

## 熔断规则

熔断**只属于出站**，作用域是单个下游 **service** —— 与限流同一个 scope。
一个 service label 只有一个 executor，因此该 label 的所有调用方共用一台熔断器：
下游坏了，所有人一起被拒；下游健康时，也不会因为某个调用方自己乱来而被拒。
不提供按 endpoint 的粒度：它只会成倍增加熔断器，却换不来调用方需要的隔离。

熔断器只回答一个问题：**这个单元还值得再试吗？** 它说的是单元，不是这次请求，
也不替代其他 stage：

| 机制 | 限制什么 | 依据 | 拒绝时 |
| --- | --- | --- | --- |
| 限流 | 请求速率 | 配置的配额 | `ErrRateLimited` |
| bulkhead | 并发度 | 配置的容量 | `ErrBulkheadFull` |
| **熔断** | **是否还值得尝试** | **近期结果** | `ErrCircuitOpen` |
| 重试 | 单次调用的尝试次数 | 上一次尝试 | 再试一次 |
| 降级 | — | 拒绝结果 | 调用方自己的兜底 |

### 分类

每个**逻辑调用**（含全部重试）恰好贡献一个样本。分类由 adapter 完成，
熔断器只负责计数：

| 样本 | 具体情形 |
| --- | --- |
| failure | 连接拒绝/重置、DNS/TLS 失败、响应前 EOF、I/O 超时、`Policy.Timeout` 到期、HTTP 5xx、gRPC `Unavailable` / `Internal` / `DeadlineExceeded` / `DataLoss`，以及调用返回前的 panic |
| success | HTTP 2xx/3xx、HTTP 4xx（单元正常作答并拒绝了这次请求）、HTTP 429 / gRPC `ResourceExhausted`（流控也是一种作答，不是故障）、未列入 failure 的 gRPC code、adapter 声明的成功哨兵（cache miss / no rows） |
| ignored | 调用方自身的取消或超时，且此时还没有拿到关于该单元的裁决 |

慢调用**不是独立的样本类别**：每个样本都带耗时，由选中的策略在耗时**严格大于** `SlowCallDurationThreshold` 时把它计为慢调用——成功和失败都一样。`error-rate` 下，慢失败既是 failure（失败率分子），也是慢调用（慢率分子）；`slow-call` 下只看慢。resilience4j 和 Sentinel 都是这样统计的。

4xx **不是** failure：下游正常工作了，是请求有问题。5xx 才是：下游收到请求但没做成。
调用方中途离开不构成任何证据；`Policy.Timeout` 是我们自己的处理预算，到期仍算 failure。
重试预算拒绝不会抹掉第一次尝试的失败：样本取自逻辑调用，而不是那次拒绝。
ignored 样本既不打开也不关闭电路；在半开状态下它归还探测许可，
所以一次无裁决的探测不会把健康的电路重新打开。

### adapter 的责任

上表是每个 adapter 要实现的契约；引擎只看得到 `error`。HTTP adapter 把
`>= 500` 映射为 failure，其余响应原样返回，所以 2xx/3xx/4xx 和 429 都是
success。gRPC adapter 必须把 `InvalidArgument` / `NotFound` / `AlreadyExists` /
`PermissionDenied` / `Unauthenticated` / `FailedPrecondition` / `OutOfRange`
映射为 success，把 `Unavailable` / `Internal` / `DataLoss` / `Canceled` 映射为
failure。**调用方自身的取消**是唯一的 ignored 情形，由引擎从调用方 context
自行识别；adapter 没有单独的 ignored 通道，凡是以 error 上报的都算 failure。
cache miss / no rows 这类业务哨兵算 success。adapter 无法归类时，如实透传
真实 error，不要凭空造一个。

### 触发、冷却与恢复

在最近一个 `BreakerWindow` 内，满足任一条件即熔断：

- 样本数 ≥ `MinRequests`，且失败率 ≥ `ErrorRateThreshold`；或
- 配置了慢调用阈值对时，慢调用率 ≥ `SlowCallRateThreshold`；或
- `consecutive` 策略下连续失败 ≥ `ErrorThreshold` 次。

熔断进入 Open，持续 `OpenDuration`；随后半开轮次放行 `HalfOpenRequests`
个探测，全部成功才关闭，首个失败立即重新打开。`HalfOpenTimeout`
约束始终不结算的轮次；为 0 时无限等待，低流量下就是持续探测。

拒绝以 `ErrCircuitOpen` 暴露。调用方拿到拒绝后返回什么——缓存值、默认值、
错误——是它自己的降级，不属于熔断器。

### 开关、默认值与严格性

一个熔断旋钮都没配时，熔断是关闭的；配置任意一个旋钮即开启（`BreakerDisabled`
保留旋钮但显式关闭保护，即 kill switch）。

选中策略但没写跳闸条件时，会套用该策略的主流默认值：`consecutive` /
`error-count` 默认 5 次（gobreaker），`error-rate` 默认 50%（Hystrix/resilience4j），
所有窗口策略的最小样本默认 5（Sentinel）。slow-call 没有有意义的默认 pair，仍然必填。
辅助旋钮默认 `BreakerWindow` 1s、`OpenDuration` 5s、`HalfOpenTrials` 1、
`HalfOpenTimeout` 无限等待。

所有值都在边界校验：非法或含糊的配置在构造时被拒绝，绝不会静默退化成惰性熔断器。

### 方向

本契约仅限客户端。入站请求由 `admission` 治理，**入站没有熔断器**（决定日期 2026-10-24）：
实例级“全拒”没有安全的爆炸半径。将来若做入站自保，它会是入站侧自己的一份组合——
按资源（action/route）分粒度、动作是降级——而不是把这台按 service 的熔断器
搬过方向。kernel 的状态机可以作为积木复用；scope、决策点和效果则不能。

## 对齐：sentinel 与 OpenSergo

策略模型是两个引擎共同翻译的中立超集，可直接映射到
[sentinel-golang](https://github.com/alibaba/sentinel-golang) 规则与
[OpenSergo](https://opensergo.io) 容错 CRD：

| 策略旋钮 | sentinel 规则（v1.0.4） | OpenSergo 策略 |
| --- | --- | --- |
| `RateLimit` + `Window` | flow `Threshold`（按窗口归一化）+ `StatIntervalInMs` | RateLimitStrategy |
| `RateLimit` + `RateLimitMaxWait` | flow `Throttling` + `MaxQueueingTimeMs` | ThrottlingStrategy |
| `RateLimitWarmUpDuration` / `WarmUpColdFactor` | flow `WarmUp` + `WarmUpPeriodSec` / `WarmUpColdFactor` | （预热） |
| `breaker-strategy=error-rate` | circuitbreaker `ErrorRatio` | CircuitBreakerStrategy ErrorRatio |
| `breaker-strategy=slow-call`（+ 慢调用对） | circuitbreaker `SlowRequestRatio` + `MaxAllowedRtMs` | CircuitBreakerStrategy SlowRequestRatio |
| `breaker-strategy=error-count` | circuitbreaker `ErrorCount`（精确） | CircuitBreakerStrategy ErrorCount |
| `breaker-strategy=consecutive`（默认） | `ErrorCount` —— **近似**：sentinel 没有连续失败熔断（它按窗口计数），要精确映射请写 `error-count` | — |
| `HalfOpenRequests` | `ProbeNum` | — |
| `Retry`（`MaxRetries` / 退避） | 在 sentinel 之外（executor 自己的重试循环） | RetryStrategy |

各 driver 的合法忽略（文档注明、绝不伪造）：`Burst`、limiter kind 在 sentinel
flow 中无对应；滑动窗口限流不排队（只有令牌桶吃 `RateLimitMaxWait`）；default
driver 的预热是冷启动近似（容量爬坡），不是 sentinel 的速率爬坡；sentinel 的
system / hotspot / 集群流控是 future work（入站或参数维度，暂无 policy 字段）。
OpenSergo 的 `RetryOnSame`（实例级重试提示）属 loadbalance，不在此处。

## 引擎与计数器

本包**没有注册表，也没有可选引擎**。`Executor` 自己持有它这一 label 的状态：
`Manager.Wrap(service, next)` 把节点和状态一起构造出来，
`Manager.Wrap(service, nil)` 是终点形式（`next == nil`）。一个 label 就是一个
保护域，构造一次并持有。想要不同机制的后端（sentinel）就自己构造一个
`chain.Executor`，通过客户端的 executor 缝显式安装。

计数器存储是**可选**的 —— 它只决定限流能扩到多宽，不决定别的。
容器里没有 `Limiter` bean 时，每个 label 的引擎用自己的一份预算计数，和它的
熔断状态完全一样：引擎按服务 label 建一份，所以"一份预算覆盖该
label 的所有调用方"依然成立。贡献一份跨共享后端的存储（Redis），manager 会
注入它，限额对全体副本生效，每个服务一份预算。它作用
在内置引擎上，这正是存储的用途：自带流控的引擎（sentinel）按自己的计数，
不用这份存储。

## 安装

```
go get go-spring.org/cloud
```

## 用法

包一个 HTTP client:

```go
import (
    "net/http"

    "go-spring.org/cloud/governance/capability/matcher"
    "go-spring.org/cloud/governance/capability/ratelimit"
    "go-spring.org/cloud/governance/dataplane/outbound/resilience"
)

fact, _ := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
mgr := resilience.NewManager(fact)
_ = mgr.ApplyTimeoutRetry(resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
    Service: "http:orders",
    Rules: []resilience.TimeoutRetryRule{{
        ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
        TimeoutRetry: resilience.TimeoutRetry{Timeout: 2 * time.Second, Retry: resilience.Retry{MaxRetries: 2}},
    }},
}}})
_ = mgr.ApplyBreaker(resilience.BreakerSettings{Services: []resilience.BreakerService{{
    Service: "http:orders",
    Rules: []resilience.BreakerRule{{
        ActionMatch:   matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
        BreakerPolicy: resilience.BreakerPolicy{ErrorThreshold: 5},
    }},
}}})
_ = mgr.ApplyRateLimit(resilience.RateLimitSettings{Services: []resilience.RateLimitService{{
    Service: "http:orders",
    Rules: []resilience.RateLimitRule{{
        ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
        Policy:      ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 100},
    }},
}}})

exec := mgr.ExecutorFor("http:orders")

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, "http:orders", nil),
}
```

只问一次预算是否够，就是同一次调用只配限流这一个 stage —— 空函数体就是被
准入的全部工作：

```go
exec := mgr.ExecutorFor("route:orders")
err := exec.Execute(ctx, chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil })
if errors.Is(err, resilience.ErrRateLimited) { /* 429 */ }
```

---

# 设计说明

`resilience` 是 go-spring 的客户端容错抽象（`go-spring.org/cloud/governance/dataplane/outbound/resilience`），
策略由控制面 [governance](../../../README.md) 下发。定义所有 adapter 都遵守的中立契约，自己持有唯一
引擎让框架开箱可跑；需要 sentinel 的调用方从 `starter/starter-governance-sentinel`
构造 executor 并显式安装。

## 1. 职责与边界

- **做：** 定义策略词汇与 `chain.Executor` 链节点；持有内置引擎；
  提供客户端 adapter（`NewRoundTripper`）。
- **不做：**
  - **不做统一的 per-request seam**。每个 client 库暴露的调用钩子形状不一；
    本包只提供最可复用的一个（`http.RoundTripper`），各 client
    adapter 把 executor 塞进自家钩子（redis.Hook /
    gorm plugin 等）。理由见 §4。
  - 不做指标、追踪、日志。adapter 自己决定如何暴露状态。
  - 无三方依赖。sentinel 后端放独立 module，框架本体保持 stdlib 无外依赖。

## 2. 关键抽象与缝隙

- **两块：manager 和它的链节点。** manager 持有策略快照和每个 label 一份共享引擎；
  `Manager.Wrap(service, next)` 在该引擎上构造韧性节点，
  `Manager.Wrap(service, nil)` 是终点形式（`next == nil`）。adapter 只依赖
  `chain.Executor`。入站的准入引擎不在这里，见 `guard`。
- **一个 Executor，一个保护域。** Executor 自己持有 per-label 状态；
  `Manager.Wrap(service, next)` 把节点和状态一起构造，`Manager.Wrap`
  是终点形式。没有注册表、没有后端 bean、没有 `spring.resilience.driver`：
  starter 只注册 manager。
- **中立拒绝错误**(`ErrRateLimited` / `ErrCircuitOpen` / `ErrBulkheadFull`)
  让 adapter 做协议特定映射（协议 starter 的 admission 中间件里 429 vs 503），
  不用 import 驱动包。
- **一个客户端 adapter 覆盖实际场景：**
  - `NewRoundTripper` —— 覆盖面最广；任何 `*http.Client` 换 Transport 即接入
    保护。重试用 `Request.GetBody` clone 请求体；5xx 计入熔断失败。

  这里曾还有一个连接级 seam（`NewDialer`），服务于没有请求层钩子的 client。
  它已退回唯一需要它的 starter（`starter-mongodb`，其驱动只提供 `SetDialer`）：
  超时 / 重试 / 熔断 / 降级这四个策略描述的是**一次操作**，把它们挂在拨号上会让
  熔断器错过运维真正在意的失败（连得上、但请求慢或报错的那些），重试变成重做握手
  而非重做工作，降级则无物可降。它是"这个 client 无处可放保护"时的降级，不是推荐
  seam —— 所以待在它那个 client 那里；只有第二个 client 证明这需求是普遍的，它才
  会回到本包。

  入站 admission 走自己的 seam：各协议 starter 用 `admission.Guard.Admit` 自建中间件
  （见 starter-gin / starter-grpc 的 admission），把中立拒绝映射为 429 / 503，并保证
  每个请求恰好服务一次。
- **降级在调用方。** `Execute` 只返回失败；决定改成什么（熔断时读缓存、返
  默认值、还是继续上抛）是调用方的几行代码，而不是一个会波及所有驱动和
  adapter 的 `degrade` 参数。
- **限流是一个 stage，不是自己的 seam。** 它跑在 `Execute` 里，调用方从不
  自己造限流器：通过 `Policy` 配置这个 stage，通过 `ErrRateLimited` 读结果。
  限流是本地还是全局，取决于计数器放在哪 —— executor 自身的预算，或一份
  `Limiter` 存储 —— 而不是第二套 API：默认一个 executor 只持有一份属于它那个
  服务的预算，共享后端（Redis）按集群计数。
- **计数器按 executor 绑定的服务分域。** 一份预算覆盖该服务的所有调用方，这才
  让限流保护的是下游，而不是下游的某一个 client。

## 3. 不变量

- 所有 adapter / helper 在 executor 为 nil / transport 为 nil / policy 为空时
  都是透明透传。没配 policy 时接线零成本。
- adapter 的 transport 必须暴露 `io.Closer`，让 starter 的 destroy 钩子释放
  executor。`roundTripper.Close` 已实现。
- `runOnce` 的每次尝试超时必须从调用方 ctx 派生，不能用 background —— 取消
  语义必须传播。
- 入站 admission 中间件（协议 starter 自建）必须防止重试重入一次已服务的请求；
  首次 `Write` 后响应已提交。
- 出站并发隔离（客户端 bulkhead）目前**不在**内置引擎里；设计取舍与暂缓原因见
  `KNOWN_ISSUES.md` §2（F-6）。自带 isolation 规则的后端仍可能返回
  `chain.ErrBulkheadFull`；adapter 已经会映射它。
- client adapter 里 `redis.Nil` / `gorm.ErrRecordNotFound` 这类"无数据"错误
  绝不能喂给熔断器；adapter 在返回 `Execute` 前把它映射为 success。
- 半开态下内置驱动严格只放行一个试探（单许可 channel，不是 bool 标志，
  所以冷却刚过时的两个并发调用不会都被放行）。sentinel 驱动用
  `ProbeNum=1` 近似；其自身的 probe 计数在强并发下不保证严格单次放行。
- 开启重试时，两个驱动的**采样数**不同：内置引擎每个**逻辑调用**只记一个
  熔断样本（跨全部重试），sentinel 驱动每次 **attempt** 建一个
  `sentinel.Entry`，所以每次重试也会成为一个样本。sentinel 把流控与熔断计数
  耦合在同一个 entry 上，要对齐就得放弃按 attempt 的流控；"每个逻辑调用一个
  样本"是内置引擎的契约，也是本包写在规范里的规则。
- 内置的 `error-rate` 熔断是“失败率 OR 慢率”。sentinel 驱动每个资源规则只能
  映射一个策略，它的 `error-rate` 规则只覆盖失败率；两个信号都需要时请用内置驱动。

## 4. 权衡与放弃的方案

- **不做统一 per-request seam。** HTTP client 有 `RoundTripper`，但 redis-go
  用 `redis.Hook`、GORM 用 plugin callback、MQ 生产者各库形态不同。要用一个
  `Interceptor` 统一，遇到 call-site-only 型钩子（NATS / pulsar）就走不通。
  故选：小而共享的 `Executor` 内核 + 一族手写 adapter —— 与 `discovery` +
  `Resolver` 相同的分层。
- **Executor 而非按阶段装饰器。** 单个 `Execute` 内把 rate / breaker /
  retry / timeout 一起做，per-service 状态（token bucket / breaker）才协调一致。按阶段独立装饰会让"重试算不算限流"这类语义摇摆。
- **与 `loadbalance.Tracker` 熔断语义重复，不复用。** 职责不同（LB 摘除是
  可查询的候选集过滤，resilience 是运行时 reject）。两者消费同一份
  per-call 错误即可保持一致，不需要代码耦合。
- **Redis 计数器存储不做 sliding-window**（在 `starter-go-redis` 那侧）。只有
  token bucket 能干净映射到原子 Lua;sliding-window 要么竞态要么每 key 数据量
  爆炸。executor 自身的预算两种都支持；Redis 存储有意只做 token bucket，配了
  sliding-window 的 scope 按 token bucket 计数。
- **入站 admission 不做重试。** 已经写出的响应无法重放；重试只在客户端 seam 有
  意义。这条约束由类型保证：`admission.Policy` 里没有 retry 字段，后端想配也配不出来。

## 5. 引擎结构、接缝与治理接线

- **一个出站引擎、一份契约。** 本包只有 `defaultExecutor`（出站，带重试）；入站引擎
  在 `guard`。两个方向只有一个 `Executor` 接口，**定义在叶子包
  `cloud/governance/dataplane/outbound/chain`**（`chain.Executor`），resilience 侧不另设别名；
  方向隔离由 `resilience.Policy` / `admission.Policy` 两个类型和各自的 manager 承担。
- **器官共享、参数中立。** 熔断状态机 / 限流桶 / `serviceState` / `Limiter`
  各一份，各自收自己的词汇：`ratelimit.Policy` 导出（store 实现与 gateway 过滤器都要读），
  `BreakerPolicy.Spec()` 包内，`resolveBreakerStrategy` 决定策略。
- **出站只有一个观测面。** 请求面（`outbound/observe`）只记录单次调用的结果，从不读取治理状态；
  本包保护措施对一次调用的全部影响都已在调用自己的错误上——拒绝是 `error_type=circuit_open`
  及其同类，下游没服务成功是 `downstream`——因此本包不发布任何器官状态序列。
  没有状态 observer 可注册，没有按规则的 key 变成 label，也不在 `chain.Executor` 上放状态快照接口。
- **命名：** 出站不再带方向限定词——本包就是出站。入站在 `guard`
  （`admission.Manager` / `admission.Policy`），两个 authority 读起来是一对对称的类型，
  谁都不需要 `Client`/`Server` 前缀。
- **manager 是具体类型：** 一个 `Manager` 持有三份已发布的 capability 快照，不持有
  任何注册表。决定一个 label 要不要重建的，是每个 executor 在调用点上自己比对的、
  按**内容**派生的代际。
- **executor 构造期绑定一个 service；`call` 不改变这一点。** `Execute(ctx, call, fn)`
  显式接收调用的身份（`chain.Call`：target / scenario / 幂等），于是每一层从签名读到、
  无须走 ctx——但保护**状态**仍绑定在构造期：一个 executor 一个 breaker、
  一份预算。`call.Target` / `call.Scenario` 目前只是**携带给读者**（以及将来按它分域的那一步）；
  它们现在不划分任何东西。绝不要让 `Execute` 按任意 per-call key **选择**保护状态；要按 key
  分区计数器，就递一个 `Limiter` 并在 scope 里放 key。
- **器官在首次调用时惰性构建**（`snapshot()` / `buildState()`）。executor 的状态观测在采集时
  读取当前代际，因此惰性构建和策略换代都不需要任何装配握手。
- **托管路径一个 label 一个 executor。** client 一律走
  `Manager.Wrap`，不自己 `Subscribe` + `NewExecutor`。别给某个 client
  加「需要看见 resolved policy」的能力（那会逼它绕开 manager），要改就在 govern rule
  层改。httpx 曾是唯一绕开 manager 的 client（自带 `min-requests=5` 下限），该下限
  已删、改走 `ExecutorFor("http", service)`；`Policy.MinRequests` 的默认值现在是 5（Sentinel 的 minRequestAmount）。
- **executor 是稳定的壳，它物化的状态是一代可替换的内核。** 刷新协议已不存在。
  `Manager.Apply` 发布一份快照就结束；每个 executor 持有一个 `PolicySource`（就是
  manager），在调用点上问「这次调用跑在什么策略下、属于哪一代」
  （`PolicySource.Resolve`）。遇到自己还没物化过的那一代，它会把这一代拥有的状态整份
  重建——breaker 与 retry 预算。**限流计数器是例外**：它活在 executor 上，规则随每次
  调用传入，配置变化是就地更新计数，而不是重建。**配置变更让 breaker/retry 重来，
  限流计数器继续计数。**
  `loadbalance.Chain` 是同一形状（`SelectionSource` + 更换 `*Pool`），也是这次对齐的模板。
  两个要讲清的后果。**出站仍然不能走「淘汰 + 重建」——淘汰的是 EXECUTOR**：client 把构造
  时拿到的 executor 缓存终身（`go-redis`、`gorm`、`memcached`、`redigo`、`httpx` 都是）。
  移动的是 executor 后面的状态，永远不是那个对象本身。**换代瞬间在途的调用**会用自己
  加载的那一代结算，代价只是丢一个样本。
  代际按该 label 所依赖规则集的**内容**派生，不是推送计数，所以内容未变的重复推送不会
  重置一个活着的熔断器。
  状态在框架够不到的地方的后端保留自己的边界：sentinel 的规则在 SDK 里、按 resource
  name 存，对它来说换代是一次**换规则**，SDK 的统计得以保留。
- **限流是 executor 的一个 stage，只有一份契约。** `Limiter.Allow(ctx, policy, n)` 有两个实现：executor 自带的进程内计数器（每个 rate key 一个，
  默认路径）与共享 store（Redis，跨副本一份预算；scope = executor 绑定的 service
  名）。进程级、被无关 client 共享的内存 store 仍然否决——不要加回
  `NewMemoryCounters`；按 key 计数是消费方（gateway）自己的事；共享 store 实现的
  文档只讲跨副本。
- **breaker 记录是 per-逻辑调用一次，rate-limit 保留 per-attempt。**
- **`Policy` 单定义**、自带全部 value tag；governance 的 `ResilienceService` /
  `ResilienceConfig` 直接嵌它（`AdmissionService` 嵌 `admission.Policy`），不造绑定孪生或
  翻译层。新增治理可绑旋钮直接加在 Policy 上（带 tag）；不把函数字段塞进 Policy。
- **`Timeout` 已更名 `Timeout`**（与键名一致）；`RetryPredicate` 函数字段已删
  （全仓零赋值），重试分类只走 `Retryable` 接口的**白名单**：未声明可重试的错误不重试。
  重试已收进 `Retry`
  节点（`resilience.Retry`），不再是 Policy 上的平铺字段：**超时和重试都只由治理文档
  决定，调用方代码不参与**。非幂等调用由文档侧保证：不给它的 service/interface 声明
  调度即可；调用方只保留自己的 context deadline（可以收紧治理预算，不能放宽）。节点字段：退避
  `InitialInterval` / `Multiplier` / `MaxInterval` / `RandomizationFactor`、整通
  `Timeout`、breaker 策略 `BreakerStrategy` / `ErrorRateThreshold` /
  `MinRequests` / `BreakerWindow`；全部零值 = 旧行为。Policy 不能 `== Policy{}`
  比较，用 `IsZero()`。拆两份仅在有硬结构理由时允许（当前仅剩 loadbalance 工厂入参
  `Config` 一处内部翻译，消费 `Selection.Params`）。
- **一个 Executor，一个保护域。** 没有注册表、没有后端 bean、没有
  `spring.resilience.driver`；`Manager.Wrap(service, next)` 把节点和状态一起构造，
  `Manager.Wrap(service, nil)` 是终点形式。不同机制（sentinel）在 manager 之外
  作为 `chain.Executor` 由调用方显式安装。
- **后端专属能力。** 集群限流、Warm-up 预热、按调用方限流、系统自适应整体保护
  （Sentinel SystemRule 类）属 sentinel executor 的能力，不进内置引擎；需要的
  调用方沿其原生配置面提供。
- **可观测桥接。** 出站/入站执行器包装落在 **`cloud/governance/dataplane/outbound/observe`**
  （`WrapClientExecutor(inner, system, service)` / `WrapServerExecutor(inner,
  system, service)`，返回 `chain.Executor`）；出站放火在
  `cloud/governance/dataplane/clientfault.Manager.WrapClientExecutor(inner, in)`；拒绝哨兵在 `cloud/governance/dataplane/outbound/chain`。
  独立的 `observe-resilience` 模块已删。gRPC 上的入站包装必须用
  `grpc.ChainUnaryInterceptor` / `ChainStreamInterceptor`——`grpc.UnaryInterceptor`
  是 setter 非追加，后者会覆盖前者。
