# resilience

[English](README.md) | [中文](README_CN.md)

`resilience` 是框架无关的客户端容错抽象：限流、熔断、bulkhead 隔离、
重试、每次尝试超时、降级。客户端 starter 把单一 `ClientExecutor` seam 插入自家请求
钩子（HTTP RoundTripper / Redis Hook / GORM plugin ...）。

## 特性

- `ClientPolicy` 字段（出站）：`RateLimit` / `Burst` / `Algorithm` / `Window` /
  `RateLimitMaxWait`、`ErrorThreshold` / `OpenDuration`、`MaxConcurrent`、
  `MaxRetries`、`AttemptTimeout`。
- `ServerPolicy` 字段（入站）：同样的限流 / 熔断 / 舱壁 / 预算旋钮，**减去 retry 族**——
  handler 已产生副作用，不能重放，所以 `ServerPolicy` 里根本写不出重试（也没有端点
  选择：入站没有端点可选）。两个模型而不是"一个模型加死字段"，约束因此是结构性的。
- 中立拒绝错误：`ErrRateLimited`、`ErrCircuitOpen`、`ErrBulkheadFull`。
- 内置 `"default"` 驱动 —— 进程内、零依赖。推荐的生产驱动 `sentinel` 在
  `starter/starter-governance-sentinel`。
- 两个客户端适配 seam:
  - `NewRoundTripper` —— HTTP client `http.RoundTripper`（覆盖面最广）。
  - `NewDialer` —— 连接级 `DialFunc`，匹配
    a dial closure over a round-robin pick pool。
- 入站 admission 跑在**同一个引擎**上，但走自己的 seam:`Manager.ServerExecutorFor`
  产出由 `Driver.NewServerExecutor` 构造的 `ServerExecutor`，各协议 starter 在它之上
  自建中间件（拒绝时 429 / 503；见 starter-gin / starter-grpc 的 admission）。
  两个方向读各自的 resolver，所以出站策略变更永远不会顺手调了入站准入。
- `Fallback(ctx, exec, fn, degrade)` —— 组合任意 executor 的降级
  helper。
- 限流是受保护调用里的一个 stage，不是独立 API：计数器就是 executor 为自身
  服务持有的预算，交给它一份 `Counters` 存储（`starter-go-redis` 的 Redis）
  则跨副本共用一份预算。

## 可插拔后端

本包**不带任何注册表**。引擎后端是以自身名字贡献、导出为 `Driver` 的 bean;
消费方把它们收成按名字索引的目录，治理中心在这份目录里解析配置里的名字。
内置引擎无需 bean 即应答 `"default"`，经 `NewDefaultDriver(nil)` 取得。

```go
// 贡献一个后端（通常在 starter 的 init 里）
gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
    Name("sentinel").
    Export(gs.As[resilience.Driver]())
```

`Export` 是承重的：gs 按精确类型索引 bean，缺了它具体的 driver 对目录不可见。

计数器存储也是后端，但是**可选**的 —— 它只决定限流能扩到多宽，不决定别的。
容器里没有 `Counters` bean 时，每个 executor 用自己的一份预算计数，和它的
熔断、舱壁状态完全一样：executor 按服务 label 建一份，所以"一份预算覆盖该
label 的所有调用方"依然成立。贡献一份跨共享后端的存储（Redis），driver 会
注入它，限额对全体副本生效，每个服务一份预算。它作用
到内置引擎的 executor 上，这正是存储的用途：自带流控的引擎（sentinel）按自己的
计数，不用这份存储。

## 安装

```
go get go-spring.org/cloud
```

## 用法

包一个 HTTP client:

```go
import (
    "net/http"

    "go-spring.org/cloud/resilience"
)

exec, _ := resilience.NewDefaultDriver(nil).NewClientExecutor("http:orders", resilience.ClientPolicy{
    RateLimit:      100,
    ErrorThreshold: 5,
    MaxRetries:     2,
    AttemptTimeout: 2 * time.Second,
})

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec),
}
```

与 `cloud/discovery` 在拨号层组合：

```go
// dialOf 取一个存活端点并拨号：discovery Resolver + loadbalance Pool
// 组合出的那个闭包。
dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
    ep, err := pool.Pick(loadbalance.PickInfo{})
    if err != nil { return nil, err }
    return (&net.Dialer{}).DialContext(ctx, network, ep.Addr)
}
dial = resilience.NewDialer(dial, exec)
```

只问一次预算是否够，就是同一次调用只配限流这一个 stage —— 空函数体就是被
准入的全部工作：

```go
exec := mgr.ClientExecutorFor("gateway", "route:orders")
err := exec.Execute(ctx, func(context.Context) error { return nil })
if errors.Is(err, resilience.ErrRateLimited) { /* 429 */ }
```

---

# 设计说明

`resilience` 是 go-spring 的客户端容错抽象（`go-spring.org/cloud/resilience`），
策略由控制面 [governance](../governance/) 下发。定义所有 adapter 与 driver 都遵守的中立契约，内置一个可用的驱动让
框架开箱可跑；生产环境通常从 `starter/starter-governance-sentinel` 换成
sentinel 驱动。

## 1. 职责与边界

- **做：** 定义 `ClientPolicy`、`ClientExecutor`、`Driver`、`Counters`、`Algorithm`；
  内置 `default` 驱动；提供客户端 adapter（`NewRoundTripper`、`NewDialer`）；
  提供 `Fallback` 组合助手。
- **不做：**
  - **不做统一的 per-request seam**。每个 client 库暴露的调用钩子形状不一；
    本包只提供最可复用的两个（`http.RoundTripper` / `DialFunc`），各 client
    adapter 把 executor 塞进自家钩子（redis.Hook /
    gorm plugin 等）。理由见 §4。
  - 不做指标、追踪、日志。adapter 与 driver 自己决定如何暴露状态。
  - 无三方依赖。推荐的 sentinel 驱动放独立 module，框架本体保持 stdlib 无外
    依赖。

## 2. 关键抽象与缝隙

- **两级抽象：`ClientPolicy` / `ServerPolicy` + `Driver` + `ClientExecutor`。** 两个模型都是后端
  中立的声明式配置；`Driver.NewClientExecutor(service, Policy)` 构造出站的运行时，
  `Driver.NewServerExecutor(service, ServerPolicy)` 构造入站的。内置驱动给出**两个引擎**：
  出站 `defaultExecutor`、入站 `admissionExecutor`，互不以对方表达，共享的只是两者
  底下的器官（熔断器、限流预算、隔离闸）；sentinel 那类后端则各自映射到本土的
  flow / circuit-breaker 规则。adapter 只依赖 `ClientExecutor` / `ServerExecutor`。
  **接口之所以两个方法而不是一个**，是因为两个方向不共享模型，而**同一个对象同时实现
  两个方法**，所以 `spring.governance.driver=sentinel` 仍然是一句话切全进程。
- **容器即驱动目录。** 本包不带任何注册表：后端以自身名字贡献为 bean、导出为
  `Driver`,manager 的构造函数把所有这类 bean 收成按名字索引的 map。
  `spring.governance.driver` 在这个目录里解析出选中项 —— 与 `discovery`
  和 client starter 的 `Driver` 同一形态。内置驱动无需 bean 即应答 `"default"`。
- **中立拒绝错误**(`ErrRateLimited` / `ErrCircuitOpen` / `ErrBulkheadFull`)
  让 adapter 做协议特定映射（协议 starter 的 admission 中间件里 429 vs 503），
  不用 import 驱动包。
- **两个客户端 adapter 覆盖实际场景：**
  - `NewRoundTripper` —— 覆盖面最广；任何 `*http.Client` 换 Transport 即接入
    保护。重试用 `Request.GetBody` clone 请求体；5xx 计入熔断失败。
  - `NewDialer` —— 连接层通用；与 a dial closure over a round-robin pick pool 天然
    组合。service 固定，因为 dialer 本就绑定一个 service。

  入站 admission 走自己的 seam：各协议 starter 用 `resilience.Manager.ServerExecutorFor`
  自建中间件（见 starter-gin / starter-grpc 的 admission），把中立拒绝映射为
  429 / 503，并保证每个请求恰好服务一次。
- **`Fallback` 是助手，不进接口。** 给 `Executor.Execute` 加 `degrade` 参数会
  波及所有驱动和 adapter；独立助手可组合任何 executor（包括 nil），核心表面
  更小。
- **限流是一个 stage，不是自己的 seam。** 它跑在 `Execute` 里，调用方从不
  自己造限流器：通过 `ClientPolicy` 配置这个 stage，通过 `ErrRateLimited` 读结果。
  限流是本地还是全局，取决于计数器放在哪 —— executor 自身的预算，或一份
  `Counters` 存储 —— 而不是第二套 API：默认一个 executor 只持有一份属于它那个
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
- 内置驱动下，bulkhead 槽跨越整个 Execute（含重试）持有一个，不是每次 attempt
  一个 —— 慢下游不能被放大。
- client adapter 里 `redis.Nil` / `gorm.ErrRecordNotFound` 这类"无数据"错误
  绝不能喂给熔断器；adapter 在返回 `Execute` 前把它映射为 success。

## 4. 权衡与放弃的方案

- **不做统一 per-request seam。** HTTP client 有 `RoundTripper`，但 redis-go
  用 `redis.Hook`、GORM 用 plugin callback、MQ 生产者各库形态不同。要用一个
  `Interceptor` 统一，遇到 call-site-only 型钩子（NATS / pulsar）就走不通。
  故选：小而共享的 `ClientExecutor` 内核 + 一族手写 adapter —— 与 `discovery` +
  `Resolver` 相同的分层。
- **Executor 而非按阶段装饰器。** 单个 `Execute` 内把 rate / breaker /
  bulkhead / retry / timeout 一起做，per-service 状态（token bucket / breaker
  / 信号量）才协调一致。按阶段独立装饰会让"重试算不算限流"这类语义摇摆。
- **与 `loadbalance.Tracker` 熔断语义重复，不复用。** 职责不同（LB 摘除是
  可查询的候选集过滤，resilience 是运行时 reject）。两者消费同一份
  per-call 错误即可保持一致，不需要代码耦合。
- **Redis 计数器存储不做 sliding-window**（在 `starter-go-redis` 那侧）。只有
  token bucket 能干净映射到原子 Lua;sliding-window 要么竞态要么每 key 数据量
  爆炸。executor 自身的预算两种都支持；Redis 存储有意只做 token bucket，配了
  sliding-window 的 scope 按 token bucket 计数。
- **入站 admission 不做重试。** 已经写出的响应无法重放；重试只在客户端 seam 有
  意义。这条约束由类型保证：`ServerPolicy` 里没有 retry 字段，后端想配也配不出来。

## 5. 引擎结构、接缝与治理接线

- **两个引擎、一份契约。** 出站与入站是完全隔离的两套引擎——`defaultExecutor`
  （出站，带重试）与 `serverExecutor`（入站，无重试，`AttemptTimeout` 是整通预算）；
  两者不 embed、不投影，`ServerPolicy.AsPolicy() ClientPolicy` 已删除。两个接口
  （`ClientExecutor` / `ServerExecutor`）已合并为单一 `Executor`，**定义在叶子包
  `cloud/chain`**（`chain.Executor`），resilience 侧不另设别名；方向隔离由
  `ClientPolicy`/`ServerPolicy` 两个类型 + manager 两侧 registry 承担。
- **器官共享、参数中立。** 熔断状态机 / 限流桶 / `serviceState` / `Counters`
  各一份，各自收自己的词汇：`RateSpec` 导出（store 实现与 gateway 过滤器都要读），
  `breakerSpec` 包内，`resolveBreakerStrategy` 决定策略。
- **依赖方向严格单向：** `chain ← observability ← resilience`。拒绝哨兵在 `chain`；
  `BreakerState` / `BreakerEventListener(Setter)` 定义在 `observability/breaker.go`，
  resilience 侧全用别名保兼容。
- **命名：** 两个半边叫 `side`（`clientSide` / `serverSide`，字段 `m.client` /
  `m.server`）；`lane` 一词已否决。
- **manager 去泛型：** `clientSide` / `serverSide` 两个具体类型，`resolve` 是唯一
  函数字段；注册表存 `*entry{exec, policy}` 指针；`Apply` 逐 label 比 policy，
  变了淘汰、没变保留（保住 breaker/限流状态）。
- **executor 构造期绑定一个 service。** `Execute(ctx, fn)` 不再收 service 参数：
  拥有保护状态的 executor 在构造期就知道自己保护谁。别再往 `Execute` 上加 service
  或任何 per-call 维度；要按 key 分区就递一个 `Counters` 并在 scope 里放 key。
- **breaker 必须首次调用时惰性构建**（`snapshot()` / `buildState()`），不能在构造
  executor 时建——否则 `SetBreakerEventListener` 接不上（`Manager.ClientExecutorFor`
  里的 `WrapClientExecutor` 是手递手装上的）。
- **托管路径一个 label 一个 executor。** client 一律走
  `Manager.ClientExecutorFor`，不自己 `Subscribe` + `NewExecutor`。别给某个 client
  加「需要看见 resolved policy」的能力（那会逼它绕开 manager），要改就在 govern rule
  层改。httpx 曾是唯一绕开 manager 的 client（自带 `min-requests=5` 下限），该下限
  已删、改走 `ClientExecutorFor("http", service)`；`Policy.MinRequests` 的零值下限
  `<=0 → 1` 保持原样。
- **热更换成「淘汰 + 重建」。** `Refresh` 已从两个 executor 接口删除；policy 变了
  淘汰、下次调用重建，没变则保留状态。
- **限流是 executor 的一个 stage**，只提供 1+3：第 1 层 executor 自带 per-service
  预算（`rateState`，默认路径），第 3 层 `Counters` 接口只服务跨副本共享存储
  （scope = executor 绑定的 service 名）。第 2 层（进程内共享）已否决——不要往 cloud
  加回内存 `Counters` 实现（`NewMemoryCounters` 已删）；按 key 计数是消费方（gateway）
  自己的事；给 `Counters` 写文档只讲跨副本。
- **breaker 记录是 per-逻辑调用一次，rate-limit 保留 per-attempt。**
- **`ClientPolicy` / `ServerPolicy` 各自单定义**、自带全部 value tag；governance 的
  `ClientRule` / `ClientDefaultPolicy` / `ServerRule` 直接嵌它们，不造绑定孪生或
  翻译层。新增治理可绑旋钮直接加在 Policy 上（带 tag）；不把函数字段塞进 Policy。
- **`Timeout` 已更名 `AttemptTimeout`**（与键名一致）；`RetryPredicate` 函数字段已删
  （全仓零赋值），重试分类只走 `Retryable` 接口 + 默认全重试。Policy 字段：退避
  `InitialInterval` / `Multiplier` / `MaxInterval` / `RandomizationFactor`、总预算
  `MaxDuration`、breaker 策略 `BreakerStrategy` / `ErrorRateThreshold` /
  `MinRequests` / `BreakerWindow`；全部零值 = 旧行为。Policy 不能 `== Policy{}`
  比较，用 `IsZero()`。拆两份仅在有硬结构理由时允许（当前仅剩 loadbalance 工厂入参
  `Config` 一处内部翻译，消费 `Selection.Params`）。
- **驱动目录在容器里。** 注册表已删（`RegisterDriver` / `GetDriver` /
  `RegisterLimiter` / `GetLimiter` 全删，两个 init 注册的 `"default"` 也删）；
  resilience 是零全局、零 gs 依赖的纯契约包。后端 = 命名 bean +
  `Export(gs.As[resilience.Driver]())`；容器把这些 bean 收集成
  `map[string]resilience.Driver` 直接交给构造器 `resilience.NewManager(drivers)`
  ——没有 wiring bean，也没有 `BindDrivers` / `SetDrivers`。按名解析收在 `Manager`
  内部（字段 `driverName` + `Manager.driverFor(name)`）：空名回落内置驱动；启动时
  校验，选不到就 panic 并列出可用名（`(*Center).GoLive` 时校验，仅 Enabled），不再
  是静默回落 noop。内置驱动 = `resilience.NewDefaultDriver(counters Counters)`。
  `spring.governance.driver` 留在治理文档（`autowire:"${...}"` 读 gs properties，
  而治理键来自 `Source.Snapshot()`，搬走会劈裂配置系统）。`gs.Provide` 的 ctor 返回
  具体类型时必须 `Export`（gs 按精确类型索引）。Limiter 的选择键来自 gateway route
  filter 参数，与 driver 的文档键不同源——两者形态相同但不要合并成一步改。
  `httpx.Config.ResilienceDriver` 是 `resilience.Driver` 对象（httpx
  container-free）。
- **后端专属能力。** 集群限流、Warm-up 预热、按调用方限流、系统自适应整体保护
  （Sentinel SystemRule 类）属 Driver 后端能力，不进核心 `Policy`；sentinel 接入时
  沿其原生配置面提供。
- **可观测桥接。** 出站/入站执行器包装落在 **`cloud/observability`**
  （`WrapClientExecutor(inner, system, service)` / `WrapServerExecutor(inner,
  system, service)`，返回 `chain.Executor`）；出站放火在
  `cloud/fault.WrapClientExecutor(inner, service, in)`；拒绝哨兵在 `cloud/chain`。
  独立的 `observe-resilience` 模块已删。gRPC 上的入站包装必须用
  `grpc.ChainUnaryInterceptor` / `ChainStreamInterceptor`——`grpc.UnaryInterceptor`
  是 setter 非追加，后者会覆盖前者。
