# resilience
[English](README.md) | [中文](README_CN.md)

`resilience` 是框架无关的客户端容错抽象:限流、熔断、bulkhead 隔离、
重试、每次尝试超时、降级。客户端 starter 把单一 `ClientExecutor` seam 插入自家请求
钩子(HTTP RoundTripper / Redis Hook / GORM plugin ...);限流这一 stage 的
计数器就是 executor 为自身服务持有的预算,因此一份预算覆盖该服务,换成共享
`Counters` 存储后则覆盖整个集群。

## 特性

- `ClientPolicy` 字段(出站):`RateLimit` / `Burst` / `Algorithm` / `Window` /
  `RateLimitMaxWait`、`ErrorThreshold` / `OpenDuration`、`MaxConcurrent`、
  `MaxRetries`、`AttemptTimeout`。
- `ServerPolicy` 字段(入站):同样的限流 / 熔断 / 舱壁 / 预算旋钮,**减去 retry 族**——
  handler 已产生副作用,不能重放,所以 `ServerPolicy` 里根本写不出重试(也没有端点
  选择:入站没有端点可选)。两个模型而不是"一个模型加死字段",约束因此是结构性的。
- 中立拒绝错误:`ErrRateLimited`、`ErrCircuitOpen`、`ErrBulkheadFull`。
- 内置 `"default"` 驱动 —— 进程内、零依赖。推荐的生产驱动 `sentinel` 在
  `starter/starter-governance-sentinel`。
- 两个客户端适配 seam:
  - `NewRoundTripper` —— HTTP client `http.RoundTripper`(覆盖面最广)。
  - `NewDialer` —— 连接级 `DialFunc`,匹配
    a dial closure over a round-robin pick pool。
- 入站 admission 跑在**同一个引擎**上,但走自己的 seam:`Manager.ServerExecutorFor`
  产出由 `Driver.NewServerExecutor` 构造的 `ServerExecutor`,各协议 starter 在它之上
  自建中间件(拒绝时 429 / 503;见 starter-gin / starter-grpc 的 admission)。
  两个方向读各自的 resolver,所以出站策略变更永远不会顺手调了入站准入。
- `Fallback(ctx, exec, fn, degrade)` —— 组合任意 executor 的降级
  helper。
- 限流是受保护调用里的一个 stage,不是独立 API:计数器就是 executor 为自身
  服务持有的预算,交给它一份 `Counters` 存储(`starter-go-redis` 的 Redis)
  则跨副本共用一份预算。

## 可插拔后端

本包**不带任何注册表**。引擎后端是以自身名字贡献、导出为 `Driver` 的 bean;
消费方把它们收成按名字索引的目录,治理中心在这份目录里解析配置里的名字。
内置引擎无需 bean 即应答 `"default"`,经 `NewDefaultDriver(nil)` 取得。

```go
// 贡献一个后端(通常在 starter 的 init 里)
gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
    Name("sentinel").
    Export(gs.As[resilience.Driver]())
```

`Export` 是承重的:gs 按精确类型索引 bean,缺了它具体的 driver 对目录不可见。

计数器存储也是后端,但是**可选**的 —— 它只决定限流能扩到多宽,不决定别的。
容器里没有 `Counters` bean 时,每个 executor 用自己的一份预算计数,和它的
熔断、舱壁状态完全一样:executor 按服务 label 建一份,所以"一份预算覆盖该
label 的所有调用方"依然成立。贡献一份跨共享后端的存储(Redis),driver 会
注入它,限额对全体副本生效,每个服务一份预算。它作用
到内置引擎的 executor 上,这正是存储的用途:自带流控的引擎(sentinel)按自己的
计数,不用这份存储。

## 安装

```
go get go-spring.org/cloud
```

## 用法

包一个 HTTP client:

```go
import (
    "net/http"

    "go-spring.org/cloud/governance/resilience"
)

exec, _ := resilience.NewDefaultDriver(nil).NewExecutor("http:orders", resilience.ClientPolicy{
    RateLimit:      100,
    ErrorThreshold: 5,
    MaxRetries:     2,
    AttemptTimeout: 2 * time.Second,
})

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec),
}
```

与 `cloud/discovery` 在拨号层组合:

```go
ld, _ := discovery.NewClientDialer(ctx, "default", "orders")
dial  := resilience.NewDialer(ld.DialContext, exec)
```

只问一次预算是否够,就是同一次调用只配限流这一个 stage —— 空函数体就是被
准入的全部工作:

```go
exec := mgr.ClientExecutorFor("gateway", "route:orders")
if err := exec.Execute(ctx, func(context.Context) error { return nil });
    errors.Is(err, resilience.ErrRateLimited) { /* 429 */ }
```

---

# 设计说明

`resilience` 是治理家族(`go-spring.org/cloud/governance/resilience`)的客户端
容错抽象。定义所有 adapter 与 driver 都遵守的中立契约,内置一个可用的驱动让
框架开箱可跑;生产环境通常从 `starter/starter-governance-sentinel` 换成
sentinel 驱动。

## 1. 职责与边界

- **做:** 定义 `ClientPolicy`、`ClientExecutor`、`Driver`、`Counters`、`Algorithm`;
  内置 `default` 驱动;提供客户端 adapter(`NewRoundTripper`、`NewDialer`);
  提供 `Fallback` 组合助手。
- **不做:**
  - **不做统一的 per-request seam**。每个 client 库暴露的调用钩子形状不一;
    本包只提供最可复用的两个(`http.RoundTripper` / `DialFunc`),各 client
    adapter 把 executor 塞进自家钩子(redis.Hook /
    gorm plugin 等)。理由见 §4。
  - 不做指标、追踪、日志。adapter 与 driver 自己决定如何暴露状态。
  - 无三方依赖。推荐的 sentinel 驱动放独立 module,框架本体保持 stdlib 无外
    依赖。

## 2. 关键抽象与缝隙

- **两级抽象:`ClientPolicy` / `ServerPolicy` + `Driver` + `ClientExecutor`。** 两个模型都是后端
  中立的声明式配置;`Driver.NewClientExecutor(service, Policy)` 构造出站的运行时,
  `Driver.NewServerExecutor(service, ServerPolicy)` 构造入站的。内置驱动把 `ServerPolicy`
  投影回它的 `ClientPolicy` 引擎(`ServerPolicy.AsPolicy`)——对它而言这个投影是精确的,因为
  它没有入站专有原语;sentinel 那类后端则各自映射到本土的 flow / circuit-breaker
  规则。adapter 只依赖 `ClientExecutor` / `ServerExecutor`。**接口之所以两个方法而不是
  一个**,是因为两个方向不共享模型,而**同一个对象同时实现两个方法**,所以
  `spring.governance.driver=sentinel` 仍然是一句话切全进程。
- **容器即驱动目录。** 本包不带任何注册表:后端以自身名字贡献为 bean、导出为
  `Driver`,governance 的 wiring bean 把所有这类 bean 收成按名字索引的 map。
  `spring.governance.driver` 在这个目录里解析出选中项 —— 与 `discovery`
  和 client starter 的 `Driver` 同一形态。内置驱动无需 bean 即应答 `"default"`。
- **中立拒绝错误**(`ErrRateLimited` / `ErrCircuitOpen` / `ErrBulkheadFull`)
  让 adapter 做协议特定映射(协议 starter 的 admission 中间件里 429 vs 503),
  不用 import 驱动包。
- **两个客户端 adapter 覆盖实际场景:**
  - `NewRoundTripper` —— 覆盖面最广;任何 `*http.Client` 换 Transport 即接入
    保护。重试用 `Request.GetBody` clone 请求体;5xx 计入熔断失败。
  - `NewDialer` —— 连接层通用;与 a dial closure over a round-robin pick pool 天然
    组合。service 固定,因为 dialer 本就绑定一个 service。

  入站 admission 走自己的 seam:各协议 starter 用 `resilience.Manager.ServerExecutorFor`
  自建中间件(见 starter-gin / starter-grpc 的 admission),把中立拒绝映射为
  429 / 503,并保证每个请求恰好服务一次。
- **`Fallback` 是助手,不进接口。** 给 `Executor.Execute` 加 `degrade` 参数会
  波及所有驱动和 adapter;独立助手可组合任何 executor(包括 nil),核心表面
  更小。
- **限流是一个 stage,不是自己的 seam。** 它跑在 `Execute` 里,调用方从不
  自己造限流器:通过 `ClientPolicy` 配置这个 stage,通过 `ErrRateLimited` 读结果。
  限流是本地还是全局,取决于计数器放在哪 —— executor 自身的预算,或一份
  `Counters` 存储 —— 而不是第二套 API:默认一个 executor 只持有一份属于它那个
  服务的预算,共享后端(Redis)按集群计数。
- **计数器按 executor 绑定的服务分域。** 一份预算覆盖该服务的所有调用方,这才
  让限流保护的是下游,而不是下游的某一个 client。

## 3. 不变量

- 所有 adapter / helper 在 executor 为 nil / transport 为 nil / policy 为空时
  都是透明透传。没配 policy 时接线零成本。
- adapter 的 transport 必须暴露 `io.Closer`,让 starter 的 destroy 钩子释放
  executor。`roundTripper.Close` 已实现。
- `runOnce` 的每次尝试超时必须从调用方 ctx 派生,不能用 background —— 取消
  语义必须传播。
- 入站 admission 中间件(协议 starter 自建)必须防止重试重入一次已服务的请求;
  首次 `Write` 后响应已提交。
- 内置驱动下,bulkhead 槽跨越整个 Execute(含重试)持有一个,不是每次 attempt
  一个 —— 慢下游不能被放大。
- client adapter 里 `redis.Nil` / `gorm.ErrRecordNotFound` 这类"无数据"错误
  绝不能喂给熔断器;adapter 在返回 `Execute` 前把它映射为 success。

## 4. 权衡与放弃的方案

- **不做统一 per-request seam。** HTTP client 有 `RoundTripper`,但 redis-go
  用 `redis.Hook`、GORM 用 plugin callback、MQ 生产者各库形态不同。要用一个
  `Interceptor` 统一,遇到 call-site-only 型钩子(NATS / pulsar)就走不通。
  故选:小而共享的 `ClientExecutor` 内核 + 一族手写 adapter —— 与 `discovery` +
  `Resolver` 相同的分层。
- **Executor 而非按阶段装饰器。** 单个 `Execute` 内把 rate / breaker /
  bulkhead / retry / timeout 一起做,per-service 状态(token bucket / breaker
  / 信号量)才协调一致。按阶段独立装饰会让"重试算不算限流"这类语义摇摆。
- **与 `loadbalance.Tracker` 熔断语义重复,不复用。** 职责不同(LB 摘除是
  可查询的候选集过滤,resilience 是运行时 reject)。共用一份 `DoneInfo.Err`
  信号即可保持一致,不需要代码耦合。
- **Redis 计数器存储不做 sliding-window**(在 `starter-go-redis` 那侧)。只有
  token bucket 能干净映射到原子 Lua;sliding-window 要么竞态要么每 key 数据量
  爆炸。executor 自身的预算两种都支持;Redis 存储有意只做 token bucket,配了
  sliding-window 的 scope 按 token bucket 计数。
- **入站 admission 不做重试。** 已经写出的响应无法重放;重试只在客户端 seam 有
  意义。这条约束由类型保证:`ServerPolicy` 里没有 retry 字段,后端想配也配不出来。
