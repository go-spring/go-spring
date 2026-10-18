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

入站请求无法表达成 `Executor`：admission 在 handler 之前，而熔断/bulkhead 的状态只能在 handler 之后完成。

## 模型

```
一个 server 实例 = 一个 Guard = 一个状态管理器
```

- 接线期创建，绑定 `Identity{System, Service}`，被协议中间件捕获。
- 拥有该实例的限流预算、bulkhead 计数、熔断器和当前配置代。同 label 的两个实例也不共享状态。
- 治理权威推送新配置代；Guard 整体替换它物化的状态。调用路径只读当前状态。

## 公开面

```go
type Guard interface {
    Admit(ctx context.Context, req Request) (Result, Done)
    Close() error
}

type Done func(Outcome, error)
```

`Admit` 按固定顺序执行 handler 前的阶段：限流 → bulkhead → 熔断 gate → 放火，返回本次请求的判定。它从不执行 handler。

`Done` 是完成半边：归还 bulkhead 槽位、把结果写回熔断器。它总是非 nil（未启用这两层时是 no-op）、幂等，通常由 adapter defer：

```go
r, done := g.Admit(ctx, req)
defer func() { done(classify(), err) }()
if !r.Admitted() {
    return mapVerdict(r.Verdict()), r.Err()
}
err = handler(ctx)
return err
```

返回函数的约定是刻意的：这是 Go 里“必须配对”的惯用法，闭包已经绑定了本次请求的 ticket，不可能结算错对象。

## 阶段分类

**一次完成**（`Done` 是 no-op）：

- 限流
- 放火（延迟 / 注入错误）
- 静态降级

**两阶段**（`Admit` 获取，`Done` 完成）：

- bulkhead（占用 / 归还）
- 熔断（gate / 结果记录、半开 trial）
- 动态降级（依赖熔断状态）

区别的本质是“是否依赖本次结果”：handler 跑完之前，熔断器不知道这次失败，bulkhead 槽位也不能归还。

## 归属

包内提供：

- 形状：`Identity`、`Request`、`Verdict`、`Result`、`Outcome`、`Done`
- 策略：`Policy`、`Settings`、`Rule`
- 每实例状态管理器与默认引擎
- 可独立使用的层对象：`RateLimiter` / `Bulkhead` / `Breaker`

包外：

- handler 执行/回调
- 重试
- 可观测（span / metric / access log）
- 协议状态码映射与降级响应体
- client 的 `Driver`

放火是 `Fault` 接口，由 `fault` 包实现并在接线期注入；`admission` 不 import `fault`。

## 降级

`Policy.DegradeOnOpen` 让熔断打开时返回 `VerdictDegraded`，同时在 `Result.Err()` 保留 `ErrCircuitOpen` 作为原因。handler 仍然不执行，降级响应由 adapter 自己产出；没有降级响应的 adapter 可以继续把它映射成 503。

## 集群限流

`WithClusterLimiter` 让限流阶段扣共享 store（通常是 Redis），而不是实例本地预算。store 返回 `admission.ErrRateLimited` 才拒绝，其他错误按放行处理。`governance` 会把进程里已有的 `ratelimit.ClusterLimiter` bean 适配过来，一个 store 同时覆盖出站和入站。

## 各层独立 API

```go
lim := admission.NewRateLimiter(p, id.Service, nil)                  // Allow(ctx) bool
bulk := admission.NewBulkhead(p.MaxConcurrent, p.BulkheadQueueDuration) // Acquire -> release
br := admission.NewBreaker(p, id.Service, listener)                 // Allow / Record(Outcome, dur)
```

`Admit` 就是这三个对象的组合，不存在第二套实现。

## 与 resilience 的关系

`admission` 是 server 侧，`resilience` 是 client 侧。`resilience` 的旧入站半边（`ServerManager` / `ServerEngine` / `ServerPolicy` / `Driver.NewServerExecutor`）已删除；`driver=sentinel` 现在只影响出站，入站统一走 admission 的默认引擎。
