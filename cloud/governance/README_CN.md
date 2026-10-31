# governance

`cloud/governance` 是 go-spring 的治理运行时。它由**两个平面**加一个**组合根**组成：

| 角色 | 包 | 职责 |
| --- | --- | --- |
| 组合根 | [根包](runtime.go)（`governance.Runtime`） | 持有控制面与全部数据面职权，是进程唯一的注入点；它自己不属于任何一个平面。 |
| 控制面 | [controlplane/](controlplane/)：`policy` 各根（[timeoutretry](controlplane/timeoutretry/)、[circuitbreaker](controlplane/circuitbreaker/)、[ratelimit](controlplane/ratelimit/)、[loadbalance](controlplane/loadbalance/)、[admission](controlplane/admission/)）、[fault](controlplane/fault/) 与 [registration](controlplane/registration/) | 期望态的两端:**获取**各策略/实验文档（共用 [source](controlplane/source/) 传输）与**发布**本进程的端点态（`Registry` + `Server`）。不包含任何请求路径上的策略实现，也不暴露数据面职权。 |
| 数据面 | [dataplane/](dataplane/) | 在请求路径上执行策略，并持有进程内状态：`chain`、`traffic`、`discovery`（读侧）、`loadbalance`、`observe`、`resilience`、`admission`、`fault`、`breaker`、`ratelimit`、`label`。|

## 平面的判定标准

- **控制面 = 期望态的获取与发布**。规则文件的监听、HTTP 轮询、配置中心长连接、文档解析与分发都在这一侧;服务注册（`registration`）同样在这一侧——它把本进程的地址/权重/版本写入 naming center,是期望端点态的生产,由 `gs.Server` 生命周期驱动。etcd / nacos / 控制台等**传输适配器在 starter 模块里**。
- **数据面 = 进程内状态 + 请求路径动作**。服务发现的**读侧**（`Discovery` / `Resolver` / 本地 endpoint 目录）属于这一侧:`loadbalance.Pick` 每个请求都读取它。这是 SDK 式客户端发现模型——SDK 的 resolver + cache 是数据面的本地视图,而权威 registry 与发布动作在控制面。
- **归属按它服务的执行体系判定，不按对象角色。** 数据面是请求执行的依赖闭包,所以方向中立的叶子器官（`breaker`、`ratelimit`）和适配器共享的词汇（`label`、`traffic`）都属于它——"中立"不等于"在平面之外"。只有当请求路径之外的消费者（cache、lock、scheduler、config）也需要某个依赖时,它才上浮到最近公共祖先;`observability/attrs` 与 `observability/conf` 位于平面之外,正是这条规则的结果。
- **可观测层属于数据面，而且是最外层**。它实现了受治理的调用：出站包 `chain.Executor`，入站驱动 `admission` 的 Admit/Done。因此它允许依赖 `admission` / `breaker`，而请求路径上没有任何包依赖它。中立遥测工具已经移出平面（见下），所以这个层本身不携带跨平面词汇。
- **组合根既不是控制面也不是数据面**。它是 `governance.Runtime`：把两个平面的对象组装成一个对象图，并作为唯一注入点交给 starter。`Center` 不再扮演这个角色。

## 依赖方向

单向：控制面可以 import 数据面（它把各域 policy 类型聚合进 `Config`），组合根可以 import 两者，但数据面绝不能 import 控制面根包。控制面把整份期望态文档一次性送达；数据面在每次调用时读取当前代际，并在文档变化时整体重建每服务状态。

有两处耦合是刻意保留的,不是缺陷:控制面 `Config` 直接嵌入数据面策略类型（`resilience.Policy`、`admission.Policy`、`loadbalance.Selection`、`fault.Config`）,因为进程内控制面没有独立部署与 schema,类型/配置统一比 schema 独立更有价值;写侧 `registration.Instance` 与读侧 `discovery.Endpoint` 保持两套词汇,由后端 starter 负责同步。两者都不打算统一。

跨平面的中立词汇留在 `cloud/` 顶层：[propagate](../propagate/)（跨协议载体）、[mesh](../mesh/)（sidecar 开关），以及 [observability/attrs](../observability/attrs/) / [observability/conf](../observability/conf/)（遥测词汇与配置刷新插桩）。两个平面与所有能力都可以依赖它们，但没有谁拥有它们。其余能力 SDK——[cache](../cache/)、[lock](../lock/)、[messaging](../messaging/)、[scheduling](../scheduling/)、[security](../security/)——仍是顶层兄弟包；它们可以依赖这些中立包，但治理运行时永不依赖这些 SDK。

## 参考模型

这套划分遵循主流的双平面模型，不是项目自定义：

- [Istio 架构](https://istio.io/latest/zh/docs/ops/deployment/architecture/) —— 数据面（Envoy）负责动态服务发现、负载均衡、熔断、健康检查、故障注入与遥测；控制面（istiod）负责策略翻译与配置分发。
- [Envoy xDS](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol) —— 控制面推送带版本号的资源（含 EDS 端点列表）；数据面用最后一份已知的本地配置继续服务，这正是请求路径只读本地代际、不向中心查询的原因。
- [Dubbo Proxyless Service Mesh](https://apache.googlesource.com/dubbo-website/+/f16c1535592b73754a93fcd0e65b78865ce84ca3/content/zh-cn/overview/mannual/golang-sdk/tutorial/governance/service-mesh/proxyless_service_mesh.md) —— 进程内 SDK 即数据面，控制面在没有 sidecar 的情况下分发配置，与本治理运行时的部署模型一致。
