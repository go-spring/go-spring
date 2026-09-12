# fault

`fault` 是 [cloud/governance/resilience](../resilience) 的进程内**故障注入**伴生包。它包装一个
`resilience.Executor`,让可配置比例的调用按需失败或变慢——给运行中的客户端"放火"——以验证
重试、熔断、per-attempt 超时、Fallback 真的会触发,并且 observe kit 能记录到相应结果。

## 能力

- 两个接缝:`WrapExecutor`(client 侧 —— 包装 executor,让注入的故障落在重试/熔断循环**内部**)
  与 `Apply`(server 侧 —— 拦截入站 handler 调用)。两者都验证 resilience,而非绕过它。
- 配置集中、可热刷新。fault 随 resilience 共用同一个 `governance.Config`(也就是同一份
  source 文档,见 [../DESIGN_CN.md §8](../DESIGN_CN.md));starter-governance 持有唯一的全进程
  `*Injector`,经中立 `InjectorFor()` seam 暴露,通过 `SetConfig` 原地热更——运行时切换放火,无需重启。
- 三种注入类型:`generic`(可重试的注入错误)、`timeout`(`context.DeadlineExceeded`)、
  `reset`(`syscall.ECONNRESET`);另有纯延迟模式。per-resource 定向走 `Config.Rules`。
- 注入错误实现 `resilience.Retryable`,无论宿主的谓词如何,都稳定触发重试。
- 仅依赖 stdlib + resilience —— 无第三方依赖,不依赖 gs/spring。gs 接线在 starter-govern,不在此包。

## 安装

```sh
go get go-spring.org/cloud
```

## 用法

starter 里通过中立 seam 解析 executor 和 injector(不 import cloud/governance,也无 per-starter
fault 配置):

```go
import (
    "go-spring.org/cloud/governance/fault"
    "go-spring.org/cloud/governance/resilience"
)

// client 侧:ExecutorFor 返回的已是"治理 + observe"组装好的 executor,
// fault 从外层包上。fault.WrapExecutor nil 安全;InjectorFor() 无注册
// (governance 关闭/尚未注册)时 Execute 惰性解析、透明直通——因此 starter
// 可在 Init 阶段(早于 starter-govern 注册注入器)就装好 fault 层。
exec := fault.WrapExecutor(resilience.ExecutorFor("redis", resource))

// server 侧:per-call 解析(nil injector 即透明直通)
err := fault.Apply(ctx, fault.InjectorFor(), "gin", func() error { return next(ctx) })
```

自包含 injector(测试、cloud/loadtest)可自行构造,传给 `WrapExecutorWith(exec, inj)` 或
`Apply(ctx, inj, ...)`。

配置(集中在治理规则文档里,key 为 `govern.fault.*` —— 见 [../CONFIG_CN.md §6](../CONFIG_CN.md)):

```properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic        # "" | "generic" | "timeout" | "reset"
govern.fault.latency=50ms         # 可选,对每次调用生效
```

注入点的取舍与边界见 [DESIGN_CN.md](DESIGN_CN.md);可运行的负载压测(含放火切换)见
`starter-redigo/example-load`。

## 状态

`WrapExecutor`(client)+ `Apply`(server)接缝、per-resource `Rules`、以及收进治理中心
(`InjectorFor()` 背后的全进程 injector)均已落地。
