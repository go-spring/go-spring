# fault

`fault` 是进程内**故障注入**家族共享的、方向中立的内核：它持有规则文档、匹配词汇与注入序列。
两个方向包包在它外面：[outbound/fault](../../dataplane/outbound/fault/) 在 resilience executor 的重试循环
内部给一次出站尝试放火，[inbound/fault](../../dataplane/inbound/fault/) 在 handler 之前决定一次入站请求。

两个方向各持一份文档与内核、互不影响——给运行中的客户端或服务端"放火"，都能验证重试、
熔断、per-attempt 超时、Fallback 真的会触发，而一侧的火碰不到另一侧。

## 能力

- 两个接缝：`WrapClientExecutor`（client 侧 —— 包装 executor，让注入的故障落在重试/熔断循环**内部**）
  与 `Server.Fire`（server 侧 —— 在 handler 之前判断这次入站调用要不要点火）。两者都验证
  resilience，而非绕过它。
- 配置集中、可热刷新。fault 随 resilience 共用同一个 `governance.Config`（也就是同一份
  source 文档，见 [README 设计说明 §8](../../../../README.md)）；两个方向包各自持有**独立的职权对象**——
  `*clientfault.Manager` 与 `*serverfault.Manager`，各自以 bean 形式注册——每个用自己的
  `Refresh` 原地热更，运行时切换放火无需重启。
- **一个方向一份配置**，一侧的火碰不到另一侧：`spring.fault.client.*` 烧出站调用，
  `spring.fault.server.*` 烧入站请求；每条**规则**各算自己的 MaxDuration/MaxAffected
  护栏——一场火自愈不会顺手关掉另一场。
- 只有规则、没有顶层效果：每个方向就是一列规则，且每条规则必须**点名 service**，
  没有任何 catch-all。所有护栏计数都按规则身份各算各的——一场火自愈不会停掉另一场。
- 四种注入类型：`generic`（可重试的注入错误）、`timeout`（`context.DeadlineExceeded`）、
  `reset`（`syscall.ECONNRESET`）、`refused`（`syscall.ECONNREFUSED`）；另有纯延迟模式。
  per-service 定向走 `Config.Rules`。
- 注入错误同时实现 `resilience.Retryable` 与 `resilience.FailureClass`，重试与熔断的判定都严格按规则声明的 `class` 来，不受宿主自身谓词影响。
- 内核本身不 import cloud 的任何调用词汇——只依赖共享的 `capability/matcher`；`chain`
  只出现在方向适配器里（client 接缝的 `Executor` 与火写回的那条记录），不在本包。流量分类也不是依赖：
  构造 operation 的 seam 把它记成一条普通的 metadata（`traffic`），规则按通用维度匹配。

## 安装

```sh
go get go-spring.org/cloud
```

## 用法

starter 里 executor 与 injector 都是注入进来的 bean（可空的 *resilience.Manager，加上本方向
自己的职权：出站用 `*clientfault.Manager`，入站用 `*serverfault.Manager`）：

```go
import (
    clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
    serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
    "go-spring.org/cloud/governance/dataplane/outbound/resilience"
)

// 两个职权都是 bean:outbound/fault 注册 *clientfault.Manager，inbound/fault 注册
// *serverfault.Manager，starter 只把本方向需要的那一个作为**可空**构造参数
// 收进来（gs.IndexArg(n, gs.TagArg("?"))）。
// nil 表示容器里没有治理，而下面两个入口都 nil 安全，所以包装是透明直通
// ——client 不会因为治理没装而无法启动。
//
// client 侧：mgr.ExecutorFor 返回的已是"治理 + observe"组装好的 executor，
// 出站职权从外层包上。持有指针就足以感知配置变更——中心用
// Manager.Refresh 原地换配置，不换对象。
exec := injClient.WrapClientExecutor(mgr.ExecutorFor("redis", service))

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
next(ctx) // `d`（Result）告诉你管辖规则、以及是否有东西真的生效
```

自包含 injector（测试、cloud/experimental/loadtest）按需用
`clientfault.NewManager()` / `serverfault.NewManager()` 构造，再用各自的 `Refresh`
武装。两侧共用同一个 `Config` 类型——内核对称，差异只在取值——方向由**你持有的职权对象**
决定，不经过任何调用方可能填错的参数。

配置（集中在治理规则文档里，一个方向一个块 —— 见 [README 配置指南 §6](../../../../README.md)）：

```properties
# 文档级元数据：标记"这一次演练"。会复制进出站/入站两个方向，使所有副本的 trace
# 与计数能按同一个 run 聚合。每一份发布的文档都必填——解除武装也要带同一个 run id。
spring.fault.run-id=drill-2026-10-27

# 出站：WrapExecutor 的 per-attempt 闸门（mgr.ExecutorFor → clientfault.Manager.WrapClientExecutor）。
# 规则是实验的最小执行单元；没有全局默认，没有规则匹配就什么都不放火；所有匹配的规则都生效。
# service 是客户端自己声明的标签（见 Target）。
spring.fault.client.rules[0].service=redis:cache
spring.fault.client.rules[0].rate=0.5
spring.fault.client.rules[0].error=generic        # 可选："generic"（默认）| "timeout" | "reset" | "refused"
spring.fault.client.rules[0].class=transient      # 可选：transient | permanent | business
spring.fault.client.rules[0].latency=50ms         # 可选，每次 attempt 都施加
spring.fault.client.rules[0].latency-jitter=20ms  # 可选，睡眠 latency ± U[0,20ms]
spring.fault.client.rules[0].max-affected=100     # 可选，爆炸半径上限（按进程）
spring.fault.client.rules[0].max-duration=5m      # 可选，自愈窗口（按进程）
spring.fault.client.rules[0].match.traffic=loadtest       # 可选："real" | "loadtest" | 任意自定义类别

# 入站：Server.Fire 的判断（gin / echo / grpc / hertz / trpc / dubbo）。
# 每条规则必须写目标 service——没有 catch-all。
spring.fault.server.rules[0].service=grpc
spring.fault.server.rules[0].action.exact=/demo.Service/Echo
spring.fault.server.rules[0].rate=0.2
spring.fault.server.rules[0].error=timeout
```

一个方向只有在至少一条规则带 `rate` 或 `latency` 时才武装；没有这样的规则就不注入。
没有单独的 `enabled` 键，也**刻意没有任何顶层效果**：瞄准单个服务的操作员不可能误烧进程里
的其他依赖，每一次放火都作为一条规则出现在配置 diff 里。

读当前生效值用 `clientfault.Manager.Config()` / `serverfault.Manager.Config()`。

注入点的取舍与边界见 [README](../../../../README.md)；可运行的负载压测（含放火切换）见
`starter-redigo/example-load`。

---

# 设计说明

`fault` 是 [cloud/governance/dataplane/outbound/resilience](../../dataplane/outbound/resilience) 的进程内故障注入伴生包。resilience
负责*保护*客户端免受下游故障，而 fault 负责*按需制造*故障，从而在压测下证明这套保护机制
真的生效。它有两个接缝——出站用 `WrapClientExecutor`、入站用 `Server.Fire`——配套
`starter-redigo/example-load` 负载二进制端到端驱动出站那个。

## 1. 职责与边界

- **做**：包装一个 `chain.Executor`，让可配置比例的调用在真正进入 executor 之前就
  失败（或变慢）；用 atomic pointer 热持有配置；导出中立的 `InjectedError`（实现了
  `resilience.Retryable`，并能表现为熟悉的 Go 错误 `context.DeadlineExceeded`、
  `syscall.ECONNRESET`）。
- **不做**:
  - 领域代码不依赖 gs / spring：注入器是个普通对象，`WrapClientExecutor` /
    `Server.Fire` 都是无容器函数。那个唯一的 bean 注册在本包 `starter.go`，热刷新由**放火控制面**
    `cloud/governance/controlplane/fault` 推入——它有自己的文档和 source，治理中心不碰放火，
    每个 client starter 也不各背一份。
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

`fire` 不收回调、自己也不记录任何东西：它返回一个 `Result`（管辖的规则、是否注入了错误、
施加的延迟）外加一个"可不可以继续"的 error，由客户端接缝把这份决策写到这次 attempt
所在的那条调用记录上。写回留在适配器，引擎才能不碰
任何调用状态（也完全不 import `chain`），而 observe 依然能从调用记录里读回来、给 span 和
访问日志打标。写回必须留在 **per-attempt 闭包内部**：在 `Execute` 返回之后才捕获决策，
拿到的只会是最后一次 attempt 的，一个「第一次放火、第二次成功」的调用就会丢掉打标。

规则是**有序列表**：第一条条件命中的规则管辖本次 attempt，命中即停止，后续规则不再运行——
一次调用永远不会吃到多个实验。顺序即同 service 规则间的优先级：具体规则在前。
命中的规则若没有实际放火（概率未抽中、护栏触顶、爆炸半径已满），本次调用不受影响，但**选择
仍然在此终止**：匹配决定管辖，掷骰决定是否注入。单条规则本身就能同时携带延迟和错误，覆盖
Envoy/Istio 的 delay+abort 组合。

注入点是刻意选的：因为 `fn` 在 inner executor 的重试循环**内部**被放火，注入的失败会被
重试、被熔断器计数、被 per-attempt 超时约束、被 Fallback 捕获——与真实下游故障走的完全是
同一条路径。若改成在 `Execute` 边界短路，这一切都被绕过，放火就失去意义。

**宿主 starter 里的包装顺序**（以 redigo 为例）：`fault( observe( rawExec ) )`。
`Manager.ExecutorFor` 自己在**尚未发布**的治理 executor 上应用 observe 层（这正是
observe 能在构造期挂上熔断 listener 的原因），fault 再从外层包住它。注入的故障依然抵达
真实 executor 的重试循环，既被 resilience *处理*，又被 observe *记录*。

**故障分类**。`InjectedError` 同时实现 `Retryable()` 与 `CountsAsFailure()`，由规则的
`class` 驱动：`transient`（默认）可重试且计入熔断，`permanent` 不重试但仍计入，`business`
既不重试也不计入——调用方错误的形状（等价 HTTP 4xx），不该把健康下游的熔断器打开。分类会
覆盖宿主谓词，所以硬编码"可重试"注入到不了的路径（"这个错不该重试""这个错不该打开熔断"）
现在可测。入站 adapter 把分类带到线上：`business` 映射为 400 / `codes.InvalidArgument`，
其余为 503 / `codes.Unavailable`，于是客户端自己的重试和熔断会按规则声明的分类处理。带类型的
kind 会包装一个
真实错误，因此 `errors.Is(err, context.DeadlineExceeded)` 成立，下游分类器（observe 的
outcome 映射）会把这次调用标记得和真实超时/复位一致。

**命名这次调用：`Target`。** 规则只按一个 `Target` 匹配——它能命名三个坐标：`Service`
（调的是谁）、`Action`（调的是它的哪个接口）、`Metadata`（调用方自带的定位维度）。它是
fault 自己的类型，刻意不用 `chain.Operation`：那是一个**客户端形状**的调用*声明*，带着规则
从不读的 trace/metric/log 块和客户端边语义，而入站请求根本没有一个有意义的它可给。Target
由各接缝的适配器构造——只有适配器知道自己协议的词汇：

- client（`WrapClientExecutor`）：`Service` 取调用的目标，为空则回退到这个 executor 的标签；
  `Action` 取 `chain.Request.Action`。
- server（`Server.Fire`）：`Service` 是入口（"gin"、"grpc"），`Action` 是被调用的路由或
  方法——于是**入站规则可以只给一个接口放火**（`GET /users/{id}`、`/pkg.Svc/Method`），
  而不是整个入口。适配器也可以暴露请求 metadata 供规则匹配。

规则用 `MetadataMatch`（普通 `map[string]string`，值相等的子集匹配）、
`ActionMatch`（`ActionMatch`：exact 或 prefix）声明它要匹配什么。字段名直接说明匹配的是哪个轴，因为它们是不同维度上的条件。

**单副本编排是下发决策，不是规则字段**：平台只把文档发给选中的实例（控制台/配置中心通道
知道目标是谁），因此同一份文档在收到它的每个进程上都会生效。

匹配规则刻意保持得浅。路由需要丰富的匹配——转发错后端是事故；放火需要操作员能**预判**
的匹配，因为演练时第一个问题就是"我们到底打中了什么"。主流混沌工具也是这么做的：
Chaos Mesh 匹配 method + path（支持通配）+ header map；ChaosBlade 把 regex 放在独立的
可选参数里。那些丰富的匹配文法（Envoy `StringMatcher`、Istio `StringMatch`、Gateway API、
Traefik、APISIX `vars`）属于流量路由。放火还需要的另一个轴是**剂量**——`Rate` /
`MaxAffected` / `MaxDuration`——而不是更复杂的谓词。`ActionMatch` 是 fault
自己的类型，理由与 `Target` 相同：与 `chain.ActionMatch` 结构一样，但互不派生。

扩展方式：新增操作符 = 在 `ActionMatch` 里加新字段（同一 one-of 约定），绝不复用/重新
解释已有字段；metadata 的富匹配将来以一个兄弟字段 `MetadataMatchers []MetadataMatcher`
加入，与 map 做 AND。无论哪种，`match.tenant=acme` 的含义永久不变。

**状态：稳定外壳 + 可换内核——推送，而不是查询。** manager 与
`resilience.Manager` / `loadbalance.Manager` 用同一个模型：manager 是稳定壳，一次 push
换掉的是编译好的内核。每个方向的职权对象只持有一个内核原子指针；`Fire` 每次 attempt
装载一次，调用路径**不取锁、不问 manager**。refresh 无锁——串行性由唯一的 source loop
契约承担。refresh 是**对账**而不是重写——规则身份是 `(run id, name)`：同一次 run 里同名规则保留计数
（所以控制器可以每个周期重推同一份文档），换 run id 则每条规则从零开始（上一场烧完的
`MaxAffected` 不能把下一场静默关掉）；消失的规则随旧内核丢弃。因此"在推送的文档里出现"就是
一条规则的完整生命周期：出现即开始，消失即结束并回收，`max-duration` / `max-affected` 触发自动结束。
刻意**没有**单独的启动/结束调用——文档本身就是控制面。

**两个方向，两个 Manager。** `clientfault.Manager` 与 `serverfault.Manager` 是互相独立的职权对象，
各自持有一份文档与一个内核，所以出站的火与入站的火互不可见、
互不触发：`Client` 读出站职权对象的内核，`Server` 读入站职权对象的内核。两者共享
`manager.go` 里的中立内核——规则语义、注入序列、护栏模型——但不共享职权对象与创建 API，
这是让两侧各自演化自己接缝的前提。两份配置也因此必须拆成两组 key
（`spring.fault.client.*` / `spring.fault.server.*`），而不是一个块加一个
per-rule 方向标记——方向是"这场火在哪儿被评估"的属性，而一条规则可能在两侧都成立，
规则内部没有任何东西能声明它。

**热刷新**。每个方向用一个 `atomic.Pointer` 整体换内核；放火中心在 source 每次 push 时
调两个职权对象各自的 `Refresh`，放火可在运行时切换、无需重启。文档在绑定时已由各字段的
`expr` 校验过——无名规则、越界剂量根本成不了规则；被 source 拒绝的文档保留上一份继续服务。
跨副本的计数聚合是平台的活，读各进程的 `Config()` 即可。**没有**"启动时必须挂火才能包装"的限制：
包装与中间件都是结构性的、无条件安装（nil injector 即透明直通），所以
惰性 → 挂火的 push 在下一次调用就生效。

## 3. 约束

- **只依赖 cloud 自身的词汇**。内核只 import 共享的 `capability/matcher`；`chain` 在方向适配器
  （client 接缝的 `Executor` 与火写回的那条记录），外加 `starter.go` 里的 gs 接线；
  不引入任何第三方依赖。
- **零配置透明——但 nil *injector* 不透明**。injector 在、规则未挂时，未武装的内核判不出火，
  `fn` 原样执行，与 resilience 的 `NewRoundTripper` 同一条零配置不变式。injector 本身
  必须非 nil:它是一个构造函数为 REQUIRED 的 bean，缺失会**启动失败**而不是静默降级。
- **生命周期转发**。`faultExecutor.Close` 转发给 inner executor;fault 自己不持有
  资源或策略。

## 4. 取舍 / 已否决方案

- **在 Execute 边界短路**（注入、直接返回、不调 inner）：否决——会绕过重试/熔断/超时，只验证
  *调用方*的反应而非 resilience 栈。除非未来出现明确用例，否则不做。
- **镜像 resilience 的 `Driver` 注册表做 `FaultDriver`**：暂否决。当前只有一种进程内注入策略，
  注册表是投机性的。等出现第二个后端（如 chaos-mesh 驱动）再加。
- **所有匹配规则独立生效（叠加）**：曾尝试过，**已否决**。一次调用只能返回一个错误，多规则
  注入时后续注入被丢弃却仍扣预算；延迟相加无上限；同 service 两条规则重叠时是隐式叠加，
  难以解释。而进程内效果只有"延迟（可加）+ 错误（排他）"，叠加不产生新的表达能力。
  现采用**首条命中生效**，顺序即优先级。
  组合效果（如"慢 + 错"）由单条规则内的 `Latency` + `Rate/Error` 表达。
- **首批做 Dialer / RoundTripper 接缝**：推迟到有 HTTP 或 gRPC starter 试点放火时。包结构已留好
  形状，`WrapDialer`/`WrapRoundTripper` 可与 `WrapClientExecutor` 并列落下。
- **注入全局 `InjectorFor()` 解析器**：否决——每个职权对象在构造期一次建好（`gs.Provide`
  分别造唯一的 `*clientfault.Manager` 与 `*serverfault.Manager`），没有全局解析器。client 侧
  `clientfault.Manager.WrapClientExecutor` 无条件包装（bean 必然存在）；server 侧 per-call 调
  `srv.Fire(ctx, target)`，中间件无条件安装（inert 配置透通）。
