# starter-rocketmq 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

一个 Client 原型 starter（`starter/DESIGN.md` §2.2），面向 RocketMQ，并针对
rocketmq-client-go v2 没有统一 client 实体的事实做了适配。整体沿用
starter-pulsar / starter-kafka 确立的 MQ starter 家族规约。

## 1. 职责与边界

- **负责**：`spring.rocketmq.instances.<name>` 组的 bean 生命周期（多实例、容器托管
  停机）、公共选项应用到其创建的每个生产者/消费者、rlog 到 go-spring 日志
  的桥接、fail-fast 名字服务探针、`messaging.Driver` 适配、OTel span 助手、
  `GuardedSend` 韧性 seam。
- **不负责**：消息序列化（payload 保持 `[]byte`）、顺序语义（driver 是并发
  消费的；顺序/事务消息走原生 client）、topic 管理、broker 健康探测（见
  §3）。

## 2. 关键抽象与 Seam

- **`Client` 包装 bean** — rocketmq-client-go 的生产者与消费者是独立构造的，
  没有 pulsar 那样的共享 `rocketmq.Client` 对象。因此 bean 是 starter 自己
  的包装类型：把名字服务地址、凭证、实例名只存一份，应用到它创建的一切，
  并登记每个生产者/消费者以便 Close 时统一停机。
- **`Driver`（driver.go）** — 构造 seam：可选的容器 bean。公司/伞级
  starter 可提供自己的 `Driver` bean；没有时装配回退到内置 `DefaultDriver`。
  rlog 桥是进程级全局的，所以在 DefaultDriver 内部只安装一次，而非每实例
  一次。
- **`NewDriver`（driver.go）** — 适配 `messaging.Driver`：每个 publisher 一
  个已启动生产者，每个 subscriber 一个已启动推送消费者（按 SDK 契约先
  `Subscribe` 后 `Start`）。handler 错误映射为 `ConsumeRetryLater`（broker
  重投），nil 映射为 `ConsumeSuccess`。W3C trace 上下文与压测标记经
  `msgCarrier`（`primitive.Message` 上的 `propagation.TextMapCarrier` 适配，
  对应 kafka-go 的 `recordCarrier`）走 user properties。
- **`GuardedSend`（command.go）** — 韧性 seam：SDK 没有可拒绝的中间件，
  治理执行器以可选包装的形式挂在同步 `SendSync` 路径上，经中立的
  `resilience.ExecutorFor` / `fault.InjectorFor` seam 解析（与
  starter-governance 零耦合）。

## 3. 约束

- 实例名留空时，SDK 会把默认值 "DEFAULT" 改写为 `PID#nano`
  （`internal.ChangeInstanceNameToPID`），因此多生产者进程留空是安全的；
  显式设置 `instance-name` 则让底层远端客户端共享同一个连接池。
- 推送消费者必须先 `Subscribe` 再 `Start`；包装器的 `NewPushConsumer`
  因此返回未启动的消费者，由 driver 在内部完成这套顺序。
- fail-fast 探针是 TCP 拨号而非 broker 往返：RocketMQ 远端层是惰性连接，
  拨号是唯一廉价、无副作用、与拓扑无关的探针。它能抓出配错的地址，抓
  不出 ACL 错误。
- 不注册 `health.Indicator`：与 starter-kafka/starter-pulsar 一致 —— 没有
  在 ACL 与路由等各种部署下都廉价可用的探针；README 记录了应用层探针
  （生产者往返）的做法。
- ACL 两个 key 在绑定期成对校验，单边凭证带解释地快速失败。

## 4. 权衡 / 已否决的备选

- **包装 bean vs. 原生生产者 bean**：默认生产者 bean 会把消费者构造藏起来、
  把消费组选择逼进配置；包装 bean 让原生 SDK 在两个方向上都是逃生门，
  且与 SDK 的真实组合方式一致。
- **rocketmq-client-go v2（remoting）vs. rocketmq-clients gRPC（5.x 代理）**：
  gRPC 客户端还是 RC 质量、模块布局别扭（根模块 `+incompatible`）、且必须
  5.x 代理；v2 客户端是 Apache 官方稳定路径，经 NameServer 协议同时支持
  4.x 与 5.x 集群。
