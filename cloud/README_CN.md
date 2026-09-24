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
| [cache](cache/) | 缓存 API：配置声明一次（`spring.cache`），由 go-redis / redigo / bigcache / memcached 等 starter 提供后端实现。 |
| [discovery](discovery/) | 「这个逻辑名此刻有哪些存活的 `host:port`？」——命名服务的读取侧契约，每种后端适配一次。 |
| [governance](governance/) | 运行期服务治理：集中式、可热更的策略中心，fan-out 到 [resilience](governance/resilience/)（熔断 / 限流 / 重试）、[fault](governance/fault/)（故障注入）与 [traffic](governance/traffic/)（压测流量识别）。 |
| [loadbalance](loadbalance/) | 「给定存活实例集合，这一发请求给谁？」——客户端负载均衡，附带失败摘除。 |
| [lock](lock/) | 「这个副本现在可以跑这件独占的工作吗？」——分布式锁与 leader 选举，一个契约。 |
| [mesh](mesh/) | 「我在不在 service mesh sidecar 后面？」——在的话，应用自己的发现/负载均衡让位。 |
| [messaging](messaging/) | 通过一对 `Publisher`/`Subscriber` 收发 `Message` 信封；换消息中间件只是换接线。 |
| [observability](observability/) | 把每请求属性挂在 context 上，框架代你启动的 span 也能带上它们。 |
| [scheduling](scheduling/) | 周期与 cron 后台任务：触发器（`FixedRate`、`FixedDelay`、`After`、cron、`DailyWindow`）、并发策略、分布式锁去重。 |
| [security](security/) | 与框架无关的认证与授权——身份模型加各协议族的中间件壳。 |
| [experimental](experimental/) | 尚未承诺进稳定面的演进中能力：[batch](experimental/batch/)、[contract](experimental/contract/) 契约测试、[loadtest](experimental/loadtest/)、[outbox](experimental/outbox/)、[session](experimental/session/)、[transaction](experimental/transaction/)。 |

每个包有自己的 `README.md`（及中文 `README_CN.md`），讲设计取舍与用法；
带模块级设计规则的家族另存 `DESIGN.md`。

## 与其他部分的关系

这里的包是普通 Go 库：可以直接构造使用，也可以由对应的 `starter-*` 模块从
配置注册。后端（Redis、etcd、Nacos……）从不被这里引入——本模块只定义契约，
SDK 实现由对应的 starter 供给。
