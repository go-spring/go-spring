# admission

[English](README.md) | [中文](README_CN.md)

`admission` 是 `chain` 的服务端对偶：描述一次入站受治理请求的形状。它不是 server 版的 Executor。

## 为什么不能复用 chain

| | client（`chain`） | server（`admission`） |
|---|---|---|
| 执行 | 单一环绕函数、N 次 attempt | 一次、两个钩子点 |
| 重试 | 一等公民 | 类型上无法表达 |
| 身份 | 每个出站服务/客户端 | 每个 server 实例 |
| 失败 | 一个 `error` | 协议自己的状态/码/结果 |
| 可观测 | 统一，挂在 `Operation` 上 | 可选且随协议而异 |

入站请求无法表达成 `Executor`：admission 在 handler 之前，而 bulkhead 的状态只能在 handler 之后完成。

## 模型

```
一个 server 实例 = 一个 Guard = 一个状态管理器
```

- 接线期创建，绑定 `Identity{System, Service}`，被协议中间件捕获。
- 拥有该实例的限流预算、bulkhead 计数和当前配置代。同 label 的两个实例也不共享状态。
- 治理权威推送新配置代；Guard 整体替换它物化的状态。调用路径只读当前状态。

## 熔断：入站没有

熔断**只属于出站**，规范见
[`resilience`](../../outbound/resilience/README_CN.md#熔断规则)。
按 server 实例做入站熔断，能做的只有“全拒”，没有安全的爆炸半径——一个坏接口或一个共享依赖会把同实例上所有健康接口一起打掉。因此原来的实例级熔断阶段已在 2026-10-24 连同策略字段、verdict、`Outcome` 分类和指标一起删除。

将来若做入站自保，它会是入站侧自己的一份组合——按资源（action/route）分粒度、动作是降级——而不是把客户端按 service 的熔断器搬过方向。kernel 状态机可以作为积木复用；scope、决策点和效果则不能。

## 公开面

```go
type Guard interface {
    Admit(ctx context.Context, req Request) (Result, Done)
    Close() error
}

type Done func()

// 独立固定策略。错误表示策略无法构建（max-concurrent 为负）。
func New(id Identity, p Policy, opts ...Option) (Guard, error)

// 托管：跟随推送的 Settings。
m := admission.NewManager()
g, err := m.GuardFor(id, admission.WithFault(f))
m.Apply(admission.Settings{Default: ..., Rules: ...})
```

`Result` 只读：`Admitted`、`Verdict`、`Err`、`RetryAfter`、`Latency`、`Rule`、`Timeout`、`Fault`。`Timeout` 是 adapter 施加到 handler context 上的处理预算（Guard 无法取消它没有交出去的 context）。

`WithFault` 注入入站放火阶段，`Fault` 实现位于 `fault` 包。Guard 通过 `observe.go` 注册的可观测仪器发布**自己的**阶段状态（限流用量、bulkhead 占用）；请求面 observer 从不读取这些状态。

`WithClusterLimiter` 让限流阶段扣共享 store（通常是 Redis），而不是实例本地预算。store 失败策略是“放行”：计数后端不可达不能变成一次故障。`governance` 把进程已有的 `ratelimit.ClusterLimiter` bean 适配过来，一个 limiter 同时覆盖两个方向。

各层也可以作为独立对象使用，供调用方在自己的中间件里组合同一套状态模型。本包只导出限流器；bulkhead 直接来自 capability 包，本包不做二次导出：

```go
lim := admission.NewRateLimiter(p, id.Service, nil)   // Allow(ctx) bool
bulk := bulkhead.New(p.MaxConcurrent, bulkhead.WithQueueTimeout(p.BulkheadQueueDuration)) // Acquire -> release
```

`Admit` 就是这些对象的组合，不存在第二套实现。

`Admit` 按固定顺序执行 handler 前的阶段：bulkhead → 限流 → 放火，返回本次请求的判定。它从不执行 handler。

`Done` 是完成半边：归还 bulkhead 槽位。它总是非 nil（未启用 bulkhead 时是 no-op）、幂等——完成对象把到点兜底和显式调用收敛成一次归还，bulkhead 的 release 因此严格恰好一次——通常由 adapter defer：

```go
r, done := g.Admit(ctx, req)
defer done()
if !r.Admitted() {
    return mapVerdict(r.Verdict()), r.Err()
}
return handler(ctx)
```

返回函数的约定是刻意的：这是 Go 里“必须配对”的惯用法，闭包已经绑定了本次请求的 ticket，不可能结算错对象。

## 阶段分类

**一次完成**（`Done` 是 no-op）：

- 限流
- 放火（延迟 / 注入错误）

**两阶段**（`Admit` 获取，`Done` 完成）：

- bulkhead（占用 / 归还）

区别的本质是生命周期，而不是 API 便利：handler 跑完之前，槽位不能归还。

## 归属

包内提供：

- 形状：`Identity`、`Request`、`Verdict`、`Result`、`Done`
- 策略：`Policy`、`Settings`、`Rule`
- 每实例状态管理器与默认引擎
- 可独立使用的层对象：本包导出 `RateLimiter`；`Bulkhead` 直接用 capability 包

包外：

- handler 执行/回调
- 重试
- 可观测（span / metric / access log）
- 协议状态码映射与降级响应体
- client 的 `Driver`

放火是 `Fault` 接口，由 `fault` 包实现并在接线期注入；`admission` 不 import `fault`。

## 集群限流

`WithClusterLimiter` 让限流阶段扣共享 store（通常是 Redis），而不是实例本地预算。store 返回 `admission.ErrRateLimited` 才拒绝，其他错误按放行处理。`governance` 会把进程里已有的 `ratelimit.ClusterLimiter` bean 适配过来，一个 store 同时覆盖出站和入站。

## 与 resilience 的关系

`admission` 是 server 侧，`resilience` 是 client 侧。今天只共享限流词汇：`Spec`、本地预算、`ClusterLimiter` seam 和计数算法都在 `cloud/governance/capability/ratelimit`，两个方向各自把自己的策略映射进去。熔断器是出站专属的器官（`cloud/governance/capability/circuitbreaker`），入站没有消费者。

`resilience` 的旧入站半边（`Manager` / `ServerEngine` / `ServerPolicy` / `Driver.NewServerExecutor`）已删除；`driver=sentinel` 现在只影响出站，入站统一走 admission 的默认引擎。
