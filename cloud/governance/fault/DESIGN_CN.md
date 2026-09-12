# fault 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`fault` 是 [cloud/governance/resilience](../resilience) 的进程内故障注入伴生包。resilience
负责*保护*客户端免受下游故障,而 fault 负责*按需制造*故障,从而在压测下证明这套保护机制
真的生效。首批只做一个接缝 —— [WrapExecutor] —— 配套 `starter-redigo/example-load`
负载二进制端到端驱动它。

## 1. 职责与边界

- **做**:包装一个 [resilience.Executor],让可配置比例的调用在真正进入 executor 之前就
  失败(或变慢);用 atomic pointer 热持有配置;导出中立的 [InjectedError](实现了
  [resilience.Retryable],并能表现为熟悉的 Go 错误 `context.DeadlineExceeded`、
  `syscall.ECONNRESET`)。
- **不做**:
  - 不依赖 gs / spring。fault 仅依赖 stdlib + resilience;热刷新接线留在治理中心
    `cloud/governance` 里(随 resilience 一起,共享同一份 Config 与 source),不在每个
    client starter 各背一份。
  - 不自己做 metric/trace/log。注入的故障会流经宿主的 observe 层(executor 在其内部),
    所以会被**像真实故障一样记录**——这正是放火的目的。
  - 不做基础设施级混沌。杀容器、丢包等由 docker-compose 上的 infra chaos 工具负责,
    不归本包。

## 2. 核心抽象 / 接缝

**唯一接缝:[WrapExecutor]。** [faultExecutor] 在自己的 `Execute` 内部把 operation `fn`
包一层,再委托给 inner executor:

```
faultExecutor.Execute(ctx, res, fn) =>
    inner.Execute(ctx, res, func(attempt) {
        sleep? -> 被取消? return ctx.Err()
        inject? -> return InjectedError
        return fn(attempt)
    })
```

注入点是刻意选的:因为 `fn` 在 inner executor 的重试循环**内部**被放火,注入的失败会被
重试、被熔断器计数、被 per-attempt 超时约束、被 Fallback 捕获——与真实下游故障走的完全是
同一条路径。若改成在 `Execute` 边界短路,这一切都被绕过,放火就失去意义。

**宿主 starter 里的包装顺序**(以 redigo 为例):`fault( observe( rawExec ) )`。
`resilience.ExecutorFor` 自己在**尚未发布**的治理 executor 上应用 observe 层(这正是
observe 能在构造期挂上熔断 listener 的原因),fault 再从外层包住它。注入的故障依然抵达
真实 executor 的重试循环,既被 resilience *处理*,又被 observe *记录*。

**可重试性**。[InjectedError] 实现 `Retryable() bool` 返回 true,[resilience.Policy.ShouldRetry]
会优先采信——所以注入故障稳定触发重试,不受宿主配置的谓词影响。带类型的 kind 会包装一个
真实错误,因此 `errors.Is(err, context.DeadlineExceeded)` 成立,下游分类器(observe 的
outcome 映射)会把这次调用标记得和真实超时/复位一致。

**热刷新**。[Injector] 用 `atomic.Pointer` 持有 [Config]。集中化后,治理中心
持有**唯一**的全进程 injector(starter 不再各自背 fault 配置),source 每次 push 时一次
`SetConfig(new.Fault)` 即可原地热更,放火可在运行时切换、无需重启。
starter 侧通过中立 seam 取 injector:client 侧 `fault.WrapExecutor(exec)`(不传 injector 时
`Execute` 惰性解析 `InjectorFor()`,镜像 `resilience.ExecutorFor` 的 call-time
延迟解析,故可在早于 centerHolder 注册的 `Init` 阶段调用);server 侧 per-call 调
`fault.Apply(ctx, fault.InjectorFor(), label, ...)`(nil injector 透明直通,中间件无条件安装)。
集中化顺带消除了旧版"启动时必须 `enabled=true` 才能被包装、否则运行时热开关无效"的限制。

## 3. 约束

- **仅 stdlib + resilience**。无第三方依赖,与 resilience 同处零依赖层,starter 引入 fault
  不会带来任何新依赖。
- **nil 透明 / 惰性**。`WrapExecutor(nil)` 返回 nil;**injector** 为 nil 是透明的:
  `WrapExecutor(exec)`(惰性解析 `InjectorFor()`)与 `WrapExecutorWith(exec, nil)` 在没有任何
  放火配置时原样执行 `fn`。惰性这一点使 starter 能在 `Init` 阶段就装配 fault 层,而不丢失稍后
  centerHolder 注册的注入器,与 `resilience.ExecutorFor` 的延迟解析一致。
- **生命周期转发**。`faultExecutor.Close` 与 `Refresh` 转发给 inner executor;fault 自己不持有
  资源或策略。

## 4. 取舍 / 已否决方案

- **在 Execute 边界短路**(注入、直接返回、不调 inner):否决——会绕过重试/熔断/超时,只验证
  *调用方*的反应而非 resilience 栈。除非未来出现明确用例,否则不做。
- **镜像 resilience 的 `Driver` 注册表做 `FaultDriver`**:暂否决。当前只有一种进程内注入策略,
  注册表是投机性的。等出现第二个后端(如 chaos-mesh 驱动)再加。
- **per-resource `[]Rule` 配置**:已落地。`Config.Rules` 每条带 `Resources / Rate / Latency / Error`,
  `Injector.maybe(resource)` 按第一条匹配的 Rule(或 catch-all)分发,支持"只给 redis 放火、其余全量慢调用"。
- **首批做 Dialer / RoundTripper 接缝**:推迟到有 HTTP 或 gRPC starter 试点放火时。包结构已留好
  形状,`WrapDialer`/`WrapRoundTripper` 可与 `WrapExecutor` 并列落下。
