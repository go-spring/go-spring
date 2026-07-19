# starter-config-bus 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-bus` 是集成层的一个**组合式** starter：它不开监听、也不跑
配置 provider，而是在 `starter-nats` 已有的 NATS 连接之上添加一层
**Spring Cloud Bus 风格的刷新广播**——广播一次即让全 fleet 每个实例都做一次
配置刷新。

## 1. 职责与边界

- 订阅一个共享 NATS subject，收到任一信号后通过
  `*gs.PropertiesRefresher` 重跑应用级属性刷新。
- 通过 `ConfigBus.Publish` 在同一 subject 上发送刷新信号；管理动作、
  webhook、运维端点都可以借此强制协同刷新。
- **只搬信号，不搬内容**。远程配置中心 starter（nacos/etcd/consul/vault/k8s/file）
  仍是唯一事实源；bus 只告诉订阅者“现在从你自己的源重新拉一次”。
- **不拥有** NATS 连接。它按名注入 `*StarterNats.Conn`（默认 `config-bus`），
  生命周期与关闭都交给 `starter-nats`。

## 2. 关键抽象与缝隙

- **`ConfigBus` bean。** 以命名 root 对象 `configBus` 注册、导出为
  `gs.Rooter`，即使应用不显式注入也总会被实例化。`Init` 调 `subscribe`；
  `Destroy` 调 `Unsubscribe`。
- **`RefreshEvent` 负载。** `{prefix, origin}`。只有 `Prefix` 影响分发；
  `Origin` 是可选观测元数据，永远不影响行为。
- **前缀作用域订阅。** `Config.WatchPrefixes` 是逗号分隔前缀表。事件对某个
  实例生效的条件：事件 `Prefix` 空（全 fleet 刷新），或订阅者 `WatchPrefixes`
  空（订阅一切），或事件前缀与某个已订阅前缀双向 `HasPrefix`——所以
  `"db"` 订阅者会响应 `"db.pool"` 事件，反之亦然。
- **按实例名选传输。** `Conn` 通过
  `autowire:"${spring.config.bus.nats-instance:=config-bus}"` 注入，让应用
  决定 `spring.nats.instances.*` 下的哪个 NATS 实例承载 bus。

## 3. 约束

- **只搬信号不搬负载。** 报文格式错误只记 warn 并丢弃；空 prefix 表示“全员
  刷新”。应用永远不能把消息体当配置。
- **刷新失败记录不上抛。** `RefreshProperties` 错误只记 error 日志，bus 仍继续
  接收后续信号；不能因为一次刷新失败就让实例静默脱离 fleet。
- **命名 bean 是关键。** 与 config-provider starter 同理：`gs.Rooter` 是 `any`，
  `configBus` 不能落在 `__default__`。
- **依赖 `starter-nats`。** 引用 `go-spring.org/starter-nats`；当底层连接带
  JetStream 时 `Conn` 也可访问 JetStream，但本 starter 只用核心 pub/sub。

## 4. 权衡 / 已否决方案

- **广播配置内容——否决。** 会让 bus 变成第二事实源，且与每个订阅者自己的
  配置中心 watch 竞争。只搬 hint 保住“每个 key 只有一个事实源”。
- **换传输（Kafka/Redis）——搁置。** 首版选 NATS，因为使用 Go-Spring 的应用
  很可能已经跑了它；后续可以另起一个使用相同 subject/prefix 模型的对等
  starter。
- **JetStream 持久订阅——刻意不用。** 漏掉一次广播是可恢复的：实例自己的
  远程配置 watcher 会在下一轮观察到底层变化，运维也可以随时再发一次。持久
  性会增加运维成本，却不改变正确性模型。
