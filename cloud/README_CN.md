# cloud

[English](README.md) | [中文](README_CN.md)

`cloud` 是 go-spring 的分布式应用构建块库——服务发现、负载均衡、分布式锁、
服务治理、缓存、消息、定时任务、安全……每个包都是围绕一个问题的显式小契约
（「此刻哪些实例存活？」「这个副本现在可以跑这件独占的工作吗？」），只拿一个
`context.Context` 就能用，不要求 IoC 容器。各家族的容器装配在 `starter-*`
模块里，不在这里。

## 包清单

| 包 | 回答什么问题 |
| --- | --- |
| [actuator](actuator/) | 管理端点：探针路径（[endpoint](actuator/endpoint/)）、组件健康检查（[health](actuator/health/)）、Kubernetes Pod 元数据（[podinfo](actuator/podinfo/)）。 |
| [breaker](runtime/dataplane/breaker/) | 方向中立的熔断状态机，由 [resilience](runtime/dataplane/resilience/)（出站）与 [guard](runtime/dataplane/guard/)（入站）共享。 |
| [cache](cache/) | 缓存 API：配置声明一次（`spring.cache`），由 go-redis / redigo / bigcache / memcached 等 starter 提供后端实现。 |
| [chain](runtime/dataplane/chain/) | 各层共享的受治理调用词汇：`Executor`、`Operation`、`Recorder` 与拒绝哨兵。 |
| [discovery](runtime/dataplane/discovery/) | 「这个逻辑名此刻有哪些存活的 `host:port`？」——命名服务的**读侧**（resolver + 本地 endpoint 目录，数据面）；写侧见 [registration](runtime/controlplane/registration/)。 |
| [registration](runtime/controlplane/registration/) | 服务发现的**写侧**（控制面）：把本进程发布到 naming center，随 `gs.Server` 生命周期注册/反注册/广播权重。 |
| [fault](runtime/dataplane/fault/) | 进程内故障注入：包装 [resilience](runtime/dataplane/resilience/) 的 `ClientExecutor` 接缝，按需让调用失败、变慢或返回错误。 |
| [governance](runtime/controlplane/governance/) | 运行期服务治理的**控制面**：从远端取回可热更的策略文档并 fan-out 到 [resilience](runtime/dataplane/resilience/)（出站）、[guard](runtime/dataplane/guard/)（入站）与 [fault](runtime/dataplane/fault/)（故障注入）。它不暴露任何数据面职权；进程唯一注入点是 `runtime.Runtime`。 |
| [label](runtime/dataplane/label/) | 服务 label 拼接（`ServiceLabel`）——治理规则与 manager 匹配用的键。 |
| [loadbalance](runtime/dataplane/loadbalance/) | 「给定存活实例集合，这一发请求给谁？」——客户端负载均衡，附带失败摘除。 |
| [lock](lock/) | 「这个副本现在可以跑这件独占的工作吗？」——分布式锁与 leader 选举，一个契约。 |
| [mesh](mesh/) | 「我在不在 service mesh sidecar 后面？」——在的话，应用自己的发现/负载均衡让位。 |
| [messaging](messaging/) | 通过一对 `Publisher`/`Subscriber` 收发 `Message` 信封；换消息中间件只是换接线。 |
| [observability](observability/) | **跨平面的遥测词汇**：[attrs](observability/attrs/)（每请求属性载体、共享 duration 分桶）与 [conf](observability/conf/)（属性刷新漏斗）。两者都不 import 治理代码，cache/lock/scheduling/配置 starter 用它们时不会拖入治理栈。 |
| [observability 层](runtime/dataplane/observability/) | 数据面**最外层**：出站包装 `chain.Executor`、入站驱动 guard 的 Admit/Done。它允许依赖 chain/breaker/guard；上面的中立遥测在平面之外。 |
| [propagate](propagate/) | 把领域标记——比如压测标记——跨协议边界传递：一个小的 `Carrier` 接缝，适配 HTTP header、gRPC metadata、Kafka 记录 header、dubbo attachment。 |
| [ratelimit](runtime/dataplane/ratelimit/) | 方向中立的限流阶段：计数算法、`Spec`、进程内预算与 `ClusterLimiter` 存储接缝，两个方向共享。 |
| [resilience](runtime/dataplane/resilience/) | 客户端容错，一个 `ClientExecutor` 接缝：熔断、限流、重试、退避，`Driver` 可插拔。 |
| [runtime](runtime/) | 治理运行时：根包的 `Runtime` 是**组合根与唯一注入点**（不属于任何平面）；[controlplane](runtime/controlplane/) 负责期望态的获取与发布（`governance` / `registration`），[dataplane](runtime/dataplane/) 负责请求路径执行与进程内状态（`chain`、`traffic`、`discovery` 读侧、`loadbalance`、`observability`、`resilience`、`guard`、`fault`、`breaker`、`ratelimit`、`label`）。 |
| [scheduling](scheduling/) | 周期与 cron 后台任务：触发器（`FixedRate`、`FixedDelay`、`After`、`ParseCron`、`DailyWindow`）、并发策略、分布式锁去重。 |
| [security](security/) | 与框架无关的认证与授权——身份模型加各协议族的中间件壳。 |
| [traffic](runtime/dataplane/traffic/) | 「在途请求是不是压测流量？」——一个进程级标记，经 [propagate](propagate/) 跨协议跳传播。 |
| [experimental](experimental/) | 尚未承诺进稳定面的演进中能力：[batch](experimental/batch/)、[contract](experimental/contract/) 契约测试、[loadtest](experimental/loadtest/)、[outbox](experimental/outbox/)、[session](experimental/session/)、[transaction](experimental/transaction/)。 |

每个包有自己的 `README.md`（及中文 `README_CN.md`），讲用法与设计取舍；
设计面较大的家族——`resilience`、`fault`、`security`——把完整的 `## Design`
一节放在自己的 README 里，而不是单开文件。

## 与其他部分的关系

这里的包是普通 Go 库：可以直接构造使用，也可以由对应的 `starter-*` 模块从
配置注册。后端（Redis、etcd、Nacos……）从不被这里引入——本模块只定义契约，
SDK 实现由对应的 starter 供给。
