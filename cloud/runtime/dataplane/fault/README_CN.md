# fault

`fault` 是 [cloud/runtime/dataplane/resilience](../resilience) 的进程内**故障注入**伴生包。它包装一个
`resilience.ClientExecutor`，让可配置比例的调用按需失败或变慢——给运行中的客户端"放火"——以验证
重试、熔断、per-attempt 超时、Fallback 真的会触发，并且 observe kit 能记录到相应结果。

## 能力

- 两个接缝：`WrapClientExecutor`（client 侧 —— 包装 executor，让注入的故障落在重试/熔断循环**内部**）
  与 `Server.Fire`（server 侧 —— 在 handler 之前判断这次入站调用要不要点火）。两者都验证
  resilience，而非绕过它。
- 配置集中、可热刷新。fault 随 resilience 共用同一个 `governance.Config`（也就是同一份
  source 文档，见 [README 设计说明 §8](../../../README.md)）；`cloud/runtime/dataplane/fault` 持有**两个互相
  独立的职权对象**——`*ClientManager` 与 `*ServerManager`，各自以 bean 形式注册——每个用自己的
  `Refresh` 原地热更，运行时切换放火无需重启。
- **一个方向一份配置**，一侧的火碰不到另一侧：`spring.governance.client.fault.*` 烧出站调用，
  `spring.governance.server.fault.*` 烧入站请求；每个**实验**各算自己的 MaxDuration/MaxAffected
  护栏——一场火自愈不会顺手关掉另一场。
- 与 resilience 共用唯一治理文档：fault 配置嵌在方向块内
  （`spring.governance.client.fault` / `spring.governance.server.fault`）。每个方向是一列
  **实验**，没有顶层效果；所有护栏计数都按实验（规则身份）各算各的——一场火自愈不会停掉另一场。
- 四种注入类型：`generic`（可重试的注入错误）、`timeout`（`context.DeadlineExceeded`）、
  `reset`（`syscall.ECONNRESET`）、`refused`（`syscall.ECONNREFUSED`）；另有纯延迟模式。
  per-service 定向走 `Config.Rules`。
- 注入错误实现 `resilience.Retryable`，无论宿主的谓词如何，都稳定触发重试。
- 除 cloud 自身的词汇外不引入任何依赖——`cloud/runtime/dataplane/chain`（client 适配器的 `Executor`，以及火写回的那条记录）、
  `cloud/runtime/dataplane/traffic`（`scope` 读取的压测标记传播器），外加 `starter.go` 里的 gs 接线。不引入第三方依赖。

## 安装

```sh
go get go-spring.org/cloud
```

## 用法

starter 里 executor 与 injector 都是注入进来的 bean（可空的 *resilience.ClientManager，加上本方向
自己的职权：出站用 `*fault.ClientManager`，入站用 `*fault.ServerManager`）：

```go
import (
    "go-spring.org/cloud/runtime/dataplane/fault"
    "go-spring.org/cloud/runtime/dataplane/resilience"
)

// 两个职权都是 bean:cloud/runtime/dataplane/fault 分别注册 *fault.ClientManager 与
// *fault.ServerManager，starter 只把本方向需要的那一个作为**可空**构造参数
// 收进来（gs.IndexArg(n, gs.TagArg("?"))）。
// nil 表示容器里没有治理，而下面两个入口都 nil 安全，所以包装是透明直通
// ——client 不会因为治理没装而无法启动。
//
// client 侧：mgr.ClientExecutorFor 返回的已是"治理 + observe"组装好的 executor，
// 出站职权从外层包上。持有指针就足以感知配置变更——中心用
// ClientManager.Refresh 原地换配置，不换对象。
exec := injClient.WrapClientExecutor(mgr.ClientExecutorFor("redis", service))

// server 侧：壳在**构建中间件时创建一次**，之后只读推送进来的内核——
// 每个请求都不再问 manager。Target 命名这次入站调用：入口 + 被调用的路由，
// 所以规则可以只给单个 endpoint 放火。
srv := injServer.NewServer()
d, err := srv.Fire(ctx, fault.Target{
    Service: "gin",
    Action:  "GET /users/{id}",
})
if err != nil {
    // 注入了错误，或延迟被取消：不要执行 handler
    return next(ctx) // （适配器负责把它映射成自己协议的失败形态）
}
next(ctx) // `d`（Decision）告诉你管辖规则、以及是否有东西真的生效
```

自包含 injector（测试、cloud/experimental/loadtest）按需用
`fault.NewClientManager(prop)` / `fault.NewServerManager(prop)` 构造，再用各自的 `Refresh`
武装。两侧共用同一个 `Config` 类型——内核对称，差异只在取值——方向由**你持有的职权对象**
决定，不经过任何调用方可能填错的参数。

配置（集中在治理规则文档里，一个方向一个块 —— 见 [README 配置指南 §6](../../../README.md)）：

```properties
# 出站：WrapExecutor 的 per-attempt 闸门（mgr.ClientExecutorFor → fault.ClientManager.WrapClientExecutor）。
# 一条规则 = 一个实验；没有全局默认，没有规则匹配就什么都不放火。
# service 是客户端自己声明的标签（见 Target）。
spring.governance.client.fault.rules[0].service=redis:cache
spring.governance.client.fault.rules[0].rate=0.5
spring.governance.client.fault.rules[0].error=generic        # "" | "generic" | "timeout" | "reset" | "refused"
spring.governance.client.fault.rules[0].latency=50ms         # 可选，每次命中都施加
spring.governance.client.fault.rules[0].latency-jitter=20ms  # 可选，睡眠 latency ± U[0,20ms]
spring.governance.client.fault.rules[0].max-affected=100     # 可选，爆炸半径上限
spring.governance.client.fault.rules[0].max-duration=5m      # 可选，自愈窗口
spring.governance.client.fault.rules[0].scope=loadtest       # 可选："" | "real" | "loadtest"

# 入站：Server.Fire 的判断（gin / echo / grpc / hertz / trpc / dubbo）。
# service 为空即 catch-all——"全烧"必须显式写出来。
spring.governance.server.fault.rules[0].service=grpc
spring.governance.server.fault.rules[0].action.exact=/demo.Service/Echo
spring.governance.server.fault.rules[0].rate=0.2
spring.governance.server.fault.rules[0].error=timeout
```

一个方向只有在至少一条规则带 `rate` 或 `latency` 时才武装；没有这样的规则就不注入。
没有单独的 `enabled` 键，也**刻意没有任何顶层效果**：瞄准单个服务的操作员不可能误烧进程里
的其他依赖，每一次放火都作为一条规则出现在配置 diff 里。

读当前生效值用 `ClientManager.Config()` / `ServerManager.Config()`。

注入点的取舍与边界见 [README](../../../README.md)；可运行的负载压测（含放火切换）见
`starter-redigo/example-load`。

---

# 设计说明

`fault` 是 [cloud/runtime/dataplane/resilience](../resilience) 的进程内故障注入伴生包。resilience
负责*保护*客户端免受下游故障，而 fault 负责*按需制造*故障，从而在压测下证明这套保护机制
真的生效。它有两个接缝——出站用 `WrapClientExecutor`、入站用 `Server.Fire`——配套
`starter-redigo/example-load` 负载二进制端到端驱动出站那个。

## 1. 职责与边界

- **做**：包装一个 `resilience.ClientExecutor`，让可配置比例的调用在真正进入 executor 之前就
  失败（或变慢）；用 atomic pointer 热持有配置；导出中立的 `InjectedError`（实现了
  `resilience.Retryable`，并能表现为熟悉的 Go 错误 `context.DeadlineExceeded`、
  `syscall.ECONNRESET`）。
- **不做**:
  - 领域代码不依赖 gs / spring：注入器是个普通对象，`WrapClientExecutor` /
    `Server.Fire` 都是无容器函数。那个唯一的 bean 注册在本包 `starter.go`，热刷新由治理中心
    `cloud/runtime/controlplane/governance` 推入（随 resilience 一起，共享同一份 Config 与 source），不在每个
    client starter 各背一份。
  - 不自己做 metric/trace/log。注入的故障会流经宿主的 observe 层（executor 在其内部），
    所以会被**像真实故障一样记录**——这正是放火的目的。
  - 不做基础设施级混沌。杀容器、丢包等由 docker-compose 上的 infra chaos 工具负责，
    不归本包。

## 2. 核心抽象 / 接缝

**两个接缝，一套序列。** `faultExecutor`（出站）在自己的 `Execute` 内部把 operation `fn`
包一层，再委托给 inner executor:

```
faultExecutor.Execute(ctx, call, fn) =>
    inner.Execute(ctx, call, func(attempt) {
        d, err := fire(attempt, ...)      // 决策 + 施加延迟；err != nil 表示别继续
        if d.Fired() { record(d) }        // 适配器把它写到「这一轮」上
        if err != nil { return err }
        return fn(attempt)               // 调用由适配器负责
    })
```

`fire` 不收回调、自己也不记录任何东西：它返回一个 `Decision`（管辖的规则、是否注入了错误、
施加的延迟）外加一个"可不可以继续"的 error，由客户端接缝把这份决策写到这次 attempt
所在的那条调用记录上。写回留在适配器，引擎才能不碰
任何调用状态（也完全不 import `chain`），而 observe 依然能从调用记录里读回来、给 span 和
访问日志打标。写回必须留在 **per-attempt 闭包内部**：在 `Execute` 返回之后才捕获决策，
拿到的只会是最后一次 attempt 的，一个「第一次放火、第二次成功」的调用就会丢掉打标。

`Decision.Rule` 命名的是**管辖**这次调用的规则，而不只是真的放火的那条：匹配上了但 rate
没抽中的规则同样会被命名，由 `Fired()` 区分两者——所以「我的规则为什么没生效？」光看决策
就能回答。

注入点是刻意选的：因为 `fn` 在 inner executor 的重试循环**内部**被放火，注入的失败会被
重试、被熔断器计数、被 per-attempt 超时约束、被 Fallback 捕获——与真实下游故障走的完全是
同一条路径。若改成在 `Execute` 边界短路，这一切都被绕过，放火就失去意义。

**宿主 starter 里的包装顺序**（以 redigo 为例）：`fault( observe( rawExec ) )`。
`ClientManager.ClientExecutorFor` 自己在**尚未发布**的治理 executor 上应用 observe 层（这正是
observe 能在构造期挂上熔断 listener 的原因），fault 再从外层包住它。注入的故障依然抵达
真实 executor 的重试循环，既被 resilience *处理*，又被 observe *记录*。

**可重试性**。`InjectedError` 实现 `Retryable() bool` 返回 true,`resilience.ClientPolicy.ShouldRetry`
会优先采信——所以注入故障稳定触发重试，不受宿主配置的谓词影响。带类型的 kind 会包装一个
真实错误，因此 `errors.Is(err, context.DeadlineExceeded)` 成立，下游分类器（observe 的
outcome 映射）会把这次调用标记得和真实超时/复位一致。

**命名这次调用：`Target`。** 规则只按一个 `Target` 匹配——它能命名三个坐标：`Service`
（调的是谁）、`Action`（调的是它的哪个接口）、`Metadata`（调用方自带的定位维度，由规则的
`Match` 匹配）。它是 fault 自己的类型，刻意不用 `chain.Operation`：那是一个**客户端形状**的
调用*声明*，带着规则从不读的 trace/metric/log 块和客户端边语义，而入站请求根本没有一个有意义
的它可给。Target 由各接缝的适配器构造——只有适配器知道自己协议的词汇：

- client（`WrapClientExecutor`）：`Service` 取调用的目标，为空则回退到这个 executor 的标签；
  `Action` 取 `chain.Request.Action`。
- server（`Server.Fire`）：`Service` 是入口（"gin"、"grpc"），`Action` 是被调用的路由或方法，
  于是**入站规则可以只给一个接口放火**——`GET /users/{id}`、`/pkg.Svc/Method`——而不是整个入口。
  适配器也可以暴露请求 metadata 供规则匹配。

规则用 `MetadataMatch`（普通 `map[string]string`，值相等的子集匹配）和
`ActionMatch`（类型 `ActionMatcher`：exact 或 prefix）声明它要匹配什么。字段名直接
说明匹配的是哪个轴，因为它们是不同维度上的条件。

匹配规则刻意保持得浅。路由需要丰富的匹配——转发错后端是事故；放火需要操作员能**预判**
的匹配，因为演练时第一个问题就是"我们到底打中了什么"。主流混沌工具也是这么做的：
Chaos Mesh 匹配 method + path（支持通配）+ header map；ChaosBlade 把 regex 放在独立的
可选参数里。那些丰富的匹配文法（Envoy `StringMatcher`、Istio `StringMatch`、Gateway API、
Traefik、APISIX `vars`）属于流量路由。放火还需要的另一个轴是**剂量**——`Rate` /
`MaxAffected` / `MaxDuration` / `Scope`——而不是更复杂的谓词。`ActionMatcher` 是 fault
自己的类型，理由与 `Target` 相同：与 `chain.ActionMatch` 结构一样，但互不派生。

扩展方式：新增操作符 = 在 `ActionMatcher` 里加新字段（同一 one-of 约定），绝不复用/重新
解释已有字段；metadata 的富匹配将来以一个兄弟字段 `MetadataMatchers []MetadataMatcher`
加入，与 map 做 AND。无论哪种，`match.tenant=acme` 的含义永久不变。

**状态：稳定外壳 + 可换内核——推送，而不是查询。** manager 与
`resilience.ClientManager` / `loadbalance.Manager` 用同一个模型，并落实了它要求的归属：每个壳
（`Client` / `Server`）持有自己的内核原子指针并直接读取，调用路径**从不问 manager**。
manager 登记它创建的每个壳；refresh 时构建一次新内核，在锁内**推送给所有已登记的壳**。
refresh 是**对账**而不是重写——身份（`service[index]`）没变的规则保留计数、新规则从零开始、
消失的规则随旧内核丢弃。因此"在推送的文档里出现"就是实验的完整生命周期：出现即开始，
消失即结束并回收，`max-duration` / `max-affected` 触发自动结束，壳 Close 时自动注销。
刻意**没有**单独的启动/结束调用——文档本身就是控制面。

**两个方向，两个 Manager。** `ClientManager` 与 `ServerManager` 是互相独立的职权对象，
各自持有一个 `direction`（推送的文档 + 已登记的壳），所以出站的火与入站的火互不可见、
互不触发：`Client` 读出站职权对象的内核，`Server` 读入站职权对象的内核。两者共享
`core.go` 里的中立内核——规则语义、注入序列、护栏模型——但不共享职权对象与创建 API，
这是让两侧各自演化自己接缝的前提。两份配置也因此必须拆成两组 key
（`spring.governance.client.fault.*` / `spring.governance.server.fault.*`），而不是一个块加一个
per-rule 方向标记——方向是"这场火在哪儿被评估"的属性，而一条 catch-all 规则在两侧都成立，
规则内部没有任何东西能声明它。

**热刷新**。每个方向用 `atomic.Pointer` 持有自己的 `Config`；治理中心在 source 每次 push 时
调两个职权对象各自的 `Refresh`，放火可在运行时切换、无需重启。**没有**"启动时必须挂火才能包装"的限制：
包装与中间件都是结构性的、无条件安装（nil injector 即透明直通），所以
惰性 → 挂火的 push 在下一次调用就生效。

## 3. 约束

- **只依赖 cloud 自身的词汇**。唯二的 import 是 `cloud/runtime/dataplane/chain`（client 适配器的 `Executor`，以及
  火写回的那条记录）与 `cloud/runtime/dataplane/traffic`（`scope` 读取的压测标记传播器），外加 `starter.go` 里的 gs
  接线；不引入任何第三方依赖。
- **零配置透明——但 nil *injector* 不透明**。injector 在、规则未挂时，闸门在 `Config.Inert()`
  处短路，原样执行 `fn`，与 resilience 的 `NewRoundTripper` 同一条零配置不变式。injector 本身
  必须非 nil:它是一个构造函数为 REQUIRED 的 bean，缺失会**启动失败**而不是静默降级。
- **生命周期转发**。`faultExecutor.Close` 转发给 inner executor;fault 自己不持有
  资源或策略。

## 4. 取舍 / 已否决方案

- **在 Execute 边界短路**（注入、直接返回、不调 inner）：否决——会绕过重试/熔断/超时，只验证
  *调用方*的反应而非 resilience 栈。除非未来出现明确用例，否则不做。
- **镜像 resilience 的 `Driver` 注册表做 `FaultDriver`**：暂否决。当前只有一种进程内注入策略，
  注册表是投机性的。等出现第二个后端（如 chaos-mesh 驱动）再加。
- **per-service `[]ClientRule` 配置**：已落地。`Config.Rules` 每条带 `Service / Rate / Latency / Error`,
  `Manager.maybe(service)` 按第一条匹配的 ClientRule（或 catch-all）分发，支持"只给 redis 放火、其余全量慢调用"。
- **首批做 Dialer / RoundTripper 接缝**：推迟到有 HTTP 或 gRPC starter 试点放火时。包结构已留好
  形状，`WrapDialer`/`WrapRoundTripper` 可与 `WrapClientExecutor` 并列落下。
- **注入全局 `InjectorFor()` 解析器**：否决——每个职权对象在构造期一次建好（`gs.Provide`
  分别造唯一的 `*fault.ClientManager` 与 `*fault.ServerManager`），没有全局解析器。client 侧
  `fault.ClientManager.WrapClientExecutor` 无条件包装（bean 必然存在）；server 侧 per-call 调
  `srv.Fire(ctx, target)`，中间件无条件安装（inert 配置透通）。
