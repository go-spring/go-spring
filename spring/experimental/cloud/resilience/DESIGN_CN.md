# resilience 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`resilience` 位于 stdlib(零依赖基础层)的客户端容错抽象。定义所有 adapter
与 driver 都遵守的中立契约,内置一个可用的驱动让框架开箱可跑;生产环境通常
从 `starter/starter-resilience` 换成 sentinel 驱动。

## 1. 职责与边界

- **做:** 定义 `Policy`、`Executor`、`Driver`、`RateLimiter`、`LimiterDriver`;
  内置 `default` 驱动;提供三个可 opt-in adapter(`NewRoundTripper`、
  `NewDialer`、`NewHandler`);提供 `Fallback` 组合助手;内置 token bucket /
  sliding window 两种进程内限流器。
- **不做:**
  - **不做统一的 per-request seam**。每个 client 库暴露的调用钩子形状不一;
    本包只提供最可复用的三个(`http.RoundTripper` / `DialFunc` /
    `http.Handler`),各 client adapter 把 executor 塞进自家钩子(redis.Hook /
    gorm plugin 等)。理由见 §4。
  - 不做指标、追踪、日志。adapter 与 driver 自己决定如何暴露状态。
  - 无三方依赖。推荐的 sentinel 驱动放独立 module,框架本体保持 stdlib 无外
    依赖。

## 2. 关键抽象与缝隙

- **两级抽象:`Policy` + `Driver` + `Executor`。** `Policy` 是后端中立的声明
  式配置;`Driver.NewExecutor(Policy)` 构造具体运行时。内置驱动直接读
  policy,sentinel 驱动把 policy 翻成 sentinel-golang 的 flow / circuit-breaker
  规则。adapter 只依赖 `Executor`。
- **驱动注册表**(`RegisterDriver` / `MustGetDriver`),空 / nil / 重复注册在
  init 期 panic —— 与 discovery / cache / loadbalance 同构。应用按名(在配置里)
  选择驱动。
- **中立拒绝错误**(`ErrRateLimited` / `ErrCircuitOpen` / `ErrBulkheadFull`)
  让 adapter 做协议特定映射(`NewHandler` 里 429 vs 503),不用 import 驱动包。
- **三个 adapter 覆盖实际场景:**
  - `NewRoundTripper` —— 覆盖面最广;任何 `*http.Client` 换 Transport 即接入
    保护。重试用 `Request.GetBody` clone 请求体;5xx 计入熔断失败。
  - `NewDialer` —— 连接层通用;与 `discovery.LiveDialer.DialContext` 天然
    组合。resource 固定,因为 dialer 本就绑定一个 service。
  - `NewHandler` —— 入站 admission;每个请求恰好服务一次(入站非幂等),
    中立拒绝映射为 429 / 503,5xx 计失败。
- **`Fallback` 是助手,不进接口。** 给 `Executor.Execute` 加 `degrade` 参数会
  波及所有驱动和 adapter;独立助手可组合任何 executor(包括 nil),核心表面
  更小。
- **`RateLimiter` 是独立 seam。** 只回答"该不该允许一次动作",不绑
  熔断/重试/超时。Redis 驱动做全局共享配额,内置驱动做每副本本地限流。

## 3. 不变量

- 所有 adapter / helper 在 executor 为 nil / transport 为 nil / policy 为空时
  都是透明透传。没配 policy 时接线零成本。
- adapter 的 transport 必须暴露 `io.Closer`,让 starter 的 destroy 钩子释放
  executor。`roundTripper.Close` 已实现。
- `runOnce` 的每次尝试超时必须从调用方 ctx 派生,不能用 background —— 取消
  语义必须传播。
- `NewHandler` 必须防止重试重入一次已服务的请求;首次 `Write` 后响应已提交。
- 内置驱动下,bulkhead 槽跨越整个 Execute(含重试)持有一个,不是每次 attempt
  一个 —— 慢下游不能被放大。
- client adapter 里 `redis.Nil` / `gorm.ErrRecordNotFound` 这类"无数据"错误
  绝不能喂给熔断器;adapter 在返回 `Execute` 前把它映射为 success。

## 4. 权衡与放弃的方案

- **不做统一 per-request seam。** HTTP client 有 `RoundTripper`,但 redis-go
  用 `redis.Hook`、GORM 用 plugin callback、MQ 生产者各库形态不同。要用一个
  `Interceptor` 统一,遇到 call-site-only 型钩子(NATS / pulsar)就走不通。
  故选:小而共享的 `Executor` 内核 + 一族手写 adapter —— 与 `discovery` +
  `LiveDialer` 相同的分层。
- **Executor 而非按阶段装饰器。** 单个 `Execute` 内把 rate / breaker /
  bulkhead / retry / timeout 一起做,per-resource 状态(token bucket / breaker
  / 信号量)才协调一致。按阶段独立装饰会让"重试算不算限流"这类语义摇摆。
- **与 `loadbalance.Tracker` 熔断语义重复,不复用。** 职责不同(LB 摘除是
  可查询的候选集过滤,resilience 是运行时 reject)。共用一份 `DoneInfo.Err`
  信号即可保持一致,不需要代码耦合。
- **Redis 限流不做 sliding-window**(在 `starter-go-redis` 那侧)。只有 token
  bucket 能干净映射到原子 Lua;sliding-window 要么竞态要么每 key 数据量爆炸。
  内置驱动两者都有;Redis 分布式驱动有意只做 token bucket。
- **`NewHandler` 不做重试。** 已经写出的响应无法重放;重试只在客户端 seam 有
  意义。
