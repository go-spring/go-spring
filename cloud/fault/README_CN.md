# fault

`fault` 是 [cloud/resilience](../resilience) 的进程内**故障注入**伴生包。它包装一个
`resilience.ClientExecutor`，让可配置比例的调用按需失败或变慢——给运行中的客户端"放火"——以验证
重试、熔断、per-attempt 超时、Fallback 真的会触发，并且 observe kit 能记录到相应结果。

## 能力

- 两个接缝：`WrapClientExecutor`（client 侧 —— 包装 executor，让注入的故障落在重试/熔断循环**内部**）
  与 `ApplyServer`（server 侧 —— 拦截入站 handler 调用）。两者都验证 resilience，而非绕过它。
- 配置集中、可热刷新。fault 随 resilience 共用同一个 `governance.Config`（也就是同一份
  source 文档，见 [README 设计说明 §8](../governance/README.md)）；`cloud/fault` 持有唯一的
  `*Injector`（以 bean 形式注册），通过 `SetConfig` 原地热更——运行时切换放火，无需重启。
- **一个方向一份配置**，一侧的火碰不到另一侧：`spring.governance.client.fault.*` 烧出站调用，
  `spring.governance.server.fault.*` 烧入站请求，两侧各算自己的 MaxDuration/MaxAffected 护栏——
  出站烧到自愈不会顺手关掉入站那场演练。
- 与 resilience 共用唯一治理文档：fault 配置嵌在方向块内
  （`spring.governance.client.fault` / `spring.governance.server.fault`），旧顶层
  `fault.*` 已废。护栏计数器是**按方向各算的进程级计数**——已接受的取舍：`Config.Rules`
  仍做 per-service 定向，但 `MaxAffected` / `MaxDuration` 全局计数。
- 四种注入类型：`generic`（可重试的注入错误）、`timeout`（`context.DeadlineExceeded`）、
  `reset`（`syscall.ECONNRESET`）、`refused`（`syscall.ECONNREFUSED`）；另有纯延迟模式。
  per-service 定向走 `Config.Rules`。
- 注入错误实现 `resilience.Retryable`，无论宿主的谓词如何，都稳定触发重试。
- 仅依赖 stdlib + resilience —— 无第三方依赖。注册唯一 `*Injector` bean 的 gs 接线就在本包（`starter.go`）。

## 安装

```sh
go get go-spring.org/cloud
```

## 用法

starter 里 executor 与 injector 都是注入进来的 bean（可空的 *resilience.Manager 与
*fault.Injector）：

```go
import (
    "go-spring.org/cloud/fault"
    "go-spring.org/cloud/resilience"
)

// injector 是一个 bean:cloud/fault 注册唯一的 *fault.Injector,
// starter 把它作为**可空**构造参数收进来（gs.IndexArg(n, gs.TagArg("?"))）。
// nil 表示容器里没有治理，而下面两个入口都 nil 安全，所以包装是透明直通
// ——client 不会因为治理没装而无法启动。
//
// client 侧：mgr.ClientExecutorFor 返回的已是"治理 + observe"组装好的 executor，
// fault 从外层包上。持有 injector 指针就足以感知配置变更——中心用
// Injector.SetConfig 原地换配置，不换对象。
exec := fault.WrapClientExecutor(mgr.ClientExecutorFor("redis", service), service, inj)

// server 侧：injector 在**构建中间件时捕获一次**，不 per-call 重新解析
err := fault.ApplyServer(ctx, inj, "gin", func() error { return next(ctx) })
```

自包含 injector（测试、cloud/experimental/loadtest）可用
`fault.NewInjector(fault.Configs{Client: ..., Server: ...})` 自行构造，同样方式传入。
两侧共用同一个 `Config` 类型——模型对称，差异只在取值——方向由**哪个入口读它**决定，
不经过任何调用方可能填错的参数。

配置（集中在治理规则文档里，一个方向一个块 —— 见 [README 配置指南 §6](../governance/README.md)）：

```properties
# 出站：WrapExecutor 的 per-attempt 闸门（mgr.ClientExecutorFor → fault.WrapClientExecutor）
spring.governance.client.fault.enabled=true
spring.governance.client.fault.rate=0.5
spring.governance.client.fault.error=generic        # "" | "generic" | "timeout" | "reset" | "refused"
spring.governance.client.fault.latency=50ms         # 可选，对每次调用生效
spring.governance.client.fault.latency-jitter=20ms   # 可选，每次睡眠 latency ± U[0,20ms]

# 入站：ApplyServer 的入站 handler 闸门（gin / echo / grpc / hertz / trpc / dubbo）
spring.governance.server.fault.enabled=true
spring.governance.server.fault.rate=0.2
spring.governance.server.fault.error=timeout
```

读当前生效值用 `Injector.ClientConfig()` / `Injector.ServerConfig()`。

注入点的取舍与边界见 [README](../governance/README.md)；可运行的负载压测（含放火切换）见
`starter-redigo/example-load`。

---

# 设计说明

`fault` 是 [cloud/resilience](../resilience) 的进程内故障注入伴生包。resilience
负责*保护*客户端免受下游故障，而 fault 负责*按需制造*故障，从而在压测下证明这套保护机制
真的生效。它有两个接缝——出站用 `WrapClientExecutor`、入站用 `ApplyServer`——配套
`starter-redigo/example-load` 负载二进制端到端驱动出站那个。

## 1. 职责与边界

- **做**：包装一个 `resilience.ClientExecutor`，让可配置比例的调用在真正进入 executor 之前就
  失败（或变慢）；用 atomic pointer 热持有配置；导出中立的 `InjectedError`（实现了
  `resilience.Retryable`，并能表现为熟悉的 Go 错误 `context.DeadlineExceeded`、
  `syscall.ECONNRESET`）。
- **不做**:
  - 领域代码不依赖 gs / spring：注入器是个普通对象，`WrapClientExecutor` /
    `ApplyServer` 都是无容器函数。那个唯一的 bean 注册在本包 `starter.go`，热刷新由治理中心
    `cloud/governance` 推入（随 resilience 一起，共享同一份 Config 与 source），不在每个
    client starter 各背一份。
  - 不自己做 metric/trace/log。注入的故障会流经宿主的 observe 层（executor 在其内部），
    所以会被**像真实故障一样记录**——这正是放火的目的。
  - 不做基础设施级混沌。杀容器、丢包等由 docker-compose 上的 infra chaos 工具负责，
    不归本包。

## 2. 核心抽象 / 接缝

**两个接缝，一套序列。** `faultExecutor`（出站）在自己的 `Execute` 内部把 operation `fn`
包一层，再委托给 inner executor:

```
faultExecutor.Execute(ctx, fn) =>
    inner.Execute(ctx, func(attempt) {
        sleep? -> 被取消? return ctx.Err()
        inject? -> return InjectedError
        return fn(attempt)
    })
```

注入点是刻意选的：因为 `fn` 在 inner executor 的重试循环**内部**被放火，注入的失败会被
重试、被熔断器计数、被 per-attempt 超时约束、被 Fallback 捕获——与真实下游故障走的完全是
同一条路径。若改成在 `Execute` 边界短路，这一切都被绕过，放火就失去意义。

**宿主 starter 里的包装顺序**（以 redigo 为例）：`fault( observe( rawExec ) )`。
`Manager.ClientExecutorFor` 自己在**尚未发布**的治理 executor 上应用 observe 层（这正是
observe 能在构造期挂上熔断 listener 的原因），fault 再从外层包住它。注入的故障依然抵达
真实 executor 的重试循环，既被 resilience *处理*，又被 observe *记录*。

**可重试性**。`InjectedError` 实现 `Retryable() bool` 返回 true,`resilience.ClientPolicy.ShouldRetry`
会优先采信——所以注入故障稳定触发重试，不受宿主配置的谓词影响。带类型的 kind 会包装一个
真实错误，因此 `errors.Is(err, context.DeadlineExceeded)` 成立，下游分类器（observe 的
outcome 映射）会把这次调用标记得和真实超时/复位一致。

**两个方向，一个 Injector。** `Injector` 内部每个方向持有一个 `side`（配置 + 护栏计数），
所以出站的火与入站的火互不可见、互不触发：`WrapClientExecutor` 的 per-attempt 闸门读出站侧，
`ApplyServer` 读入站侧。这也解释了为什么两份配置必须拆成两组 key
（`spring.governance.client.fault.*` / `spring.governance.server.fault.*`），而不是一个块加一个 per-rule 方向标记——
最需要方向的恰恰是 `rate`/`latency`/`error`/`scope`/护栏这些**全局**旋钮，它们没有 service
label 可供标记挂靠。

**热刷新**。每个方向用 `atomic.Pointer` 持有自己的 `Config`；治理中心在 source 每次 push 时
调 `SetConfig`，放火可在运行时切换、无需重启。**没有**"启动时必须 enabled 才能包装"的限制：
包装与中间件都是结构性的、无条件安装（nil injector 即透明直通），所以
`enabled=false → true` 的 push 在下一次调用就生效。

## 3. 约束

- **核心仅 stdlib + resilience**。无第三方依赖（gs 接线只在 `starter.go`，而 `cloud` 早已为其它
  权威依赖 spring），引入 fault 不会带来任何新依赖。
- **nil 透明**。`WrapClientExecutor(nil, service, nil)` 返回 nil;**injector** 为 nil 是透明的：
  `WrapClientExecutor(exec, service, nil)` 在没有任何放火配置时原样执行 `fn`。
- **生命周期转发**。`faultExecutor.Close` 转发给 inner executor;fault 自己不持有
  资源或策略。

## 4. 取舍 / 已否决方案

- **在 Execute 边界短路**（注入、直接返回、不调 inner）：否决——会绕过重试/熔断/超时，只验证
  *调用方*的反应而非 resilience 栈。除非未来出现明确用例，否则不做。
- **镜像 resilience 的 `Driver` 注册表做 `FaultDriver`**：暂否决。当前只有一种进程内注入策略，
  注册表是投机性的。等出现第二个后端（如 chaos-mesh 驱动）再加。
- **per-service `[]ClientRule` 配置**：已落地。`Config.Rules` 每条带 `Service / Rate / Latency / Error`,
  `Injector.maybe(service)` 按第一条匹配的 ClientRule（或 catch-all）分发，支持"只给 redis 放火、其余全量慢调用"。
- **首批做 Dialer / RoundTripper 接缝**：推迟到有 HTTP 或 gRPC starter 试点放火时。包结构已留好
  形状，`WrapDialer`/`WrapRoundTripper` 可与 `WrapClientExecutor` 并列落下。
- **注入全局 `InjectorFor()` 解析器**：否决——injector 在构造期一次建好（`gs.Provide` 直接造
  唯一的 `*fault.Injector`），没有全局解析器。client 侧 `fault.WrapClientExecutor` 在 injector
  为 nil 时**构造期短路**（直接返回 inner executor）；server 侧 per-call 调
  `fault.ApplyServer(ctx, inj, service, fn)`，中间件无条件安装（nil 透通）。
