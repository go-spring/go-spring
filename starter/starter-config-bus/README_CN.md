# starter-config-bus

[English](README.md) | [中文](README_CN.md)

`starter-config-bus` 在已有的 [NATS](https://nats.io/) 连接（来自 `starter-nats`）
之上增加**配置刷新总线**。空导入该包即注册一个 `ConfigBus` bean：它订阅一个刷新
主题，一旦收到信号就重新执行应用级属性刷新——因此一次广播即可刷新集群中的**所有**
实例，等效于 Spring Cloud Bus 的刷新广播。

它与远程配置中心 starter（`starter-config-{nacos,etcd,consul}`）互补:后者已能通过
自身 watch 刷新单个实例,而总线负责跨实例广播,以及来自配置中心之外的刷新触发（例如
强制全集群重载）。总线只传递刷新**信号**,绝不传递配置内容——配置内容仍以配置中心
或本地文件为准。

## 安装

```bash
go get go-spring.org/starter-config-bus
```

## 快速开始

### 1. 引入包（连同 starter-nats）

```go
import (
    _ "go-spring.org/starter-config-bus"
    _ "go-spring.org/starter-nats"
)
```

### 2. 指定总线使用的 NATS 连接

定义一个名字与 `spring.config.bus.nats-instance`（默认 `config-bus`）一致的 NATS
实例:

```properties
spring.nats.instances.config-bus.url=nats://127.0.0.1:4222
```

### 3. 配置总线（可选）

所有配置项位于 `spring.config.bus` 前缀下:

| 配置项           | 默认值                  | 说明                                                                     |
|------------------|-------------------------|--------------------------------------------------------------------------|
| `subject`        | `spring.config.refresh` | 发布与订阅刷新事件的 NATS 主题。                                          |
| `nats-instance`  | `config-bus`            | 作为传输通道的 `spring.nats.instances.*` 连接名。                        |
| `watch-prefixes` | (空)                    | 逗号分隔的前缀;设置后,仅当广播前缀与其中之一有交集（或为全量广播）时,本实例才刷新。 |
| `origin`        | (主机名)                | 发布方标识，写入每次广播的 `RefreshEvent.Origin` 与 producer span，用于区分"本实例刷新了"和"某个实例刷新了"。 |

### 4. 广播刷新

通过 `autowire:"configBus"` 注入总线并发布:

```go
type Service struct {
    Bus *StarterConfigBus.ConfigBus `autowire:"configBus"`
}

// 全量刷新：所有订阅者都重载。ctx 承载你的 trace，
// 从 HTTP handler 触发的刷新会出现在该请求的链路里。
_ = svc.Bus.Publish(ctx, "")

// 按前缀刷新：带前缀过滤的订阅者可跳过。
_ = svc.Bus.Publish(ctx, "db")
```

订阅该主题的每个实例都会重新执行 `RefreshProperties`,所有绑定的 `gs.Dync` 字段随之
热更新。完整的广播 → 刷新流程见 [example](example/example.go)。

## 工作原理

- 启动时 `ConfigBus` bean 被急切创建（以名字 `configBus` 导出 `gs.Rooter`）,并在
  配置的 NATS 连接上订阅 `spring.config.bus.subject`。
- `Publish(ctx, prefix)` 在主题上发送一条精简的 JSON `RefreshEvent{prefix, origin}`。
  空前缀表示全量刷新;非空前缀允许带前缀过滤的订阅者跳过（事件前缀与某个已订阅前缀
  双向有交集即生效——`db` 订阅者会响应 `db.pool` 事件，反之亦然）。ctx 把 producer span
  挂到你的 trace 下。
- 收到消息后,每个订阅者直接调用框架的进程级门面 `gs.RefreshProperties()`,重新加载
  所有配置源,并通过两阶段原子提交重新绑定每个 `gs.Dync` 字段。
- 总线不拥有 NATS 连接：它按实例名注入 `*StarterNats.Conn`，生命周期与关闭都交给
  `starter-nats`。

## 设计要点

**只搬信号，不搬配置内容。** 总线只告诉订阅者"现在从你自己的源重新拉一次"；配置中心
或本地文件仍是唯一事实源。报文格式错误只记 warn 并丢弃；空负载是合法的全量刷新。应用
永远不能把消息体当配置。

**刷新失败绝不让实例退订。** `RefreshProperties` 失败会计入 `refresh_error`、记日志，
并回传给传输层使 consumer span 标记失败——但订阅保持不变。一次刷新失败不能让实例
静默脱离集群。

**命名 root 对象是关键。** `gs.Rooter` 是 `any`，因此 bean 以显式名字 `configBus`
注册（绝不落入 `__default__`）；该名字同时也是 `Publish` 调用方的 autowire 句柄。

**广播配置内容——否决。** 会让总线变成第二事实源，且与每个订阅者自己的配置中心
watch 竞争。

**换传输（Kafka/Redis）——搁置。** 首版选 NATS，因为使用 Go-Spring 的应用很可能
已经运行它；后续可以另起一个使用相同 subject/prefix 模型的对等 starter。

**JetStream 持久订阅——刻意不用。** 漏掉一次广播是可恢复的：实例自己的远程配置
watcher 会在下一轮观察到底层变化，运维也可以随时重发。持久性会增加运维成本，却不
改变正确性模型。

## 可观测性

广播是即发即忘的——core NATS 不向发布方返回确认——因此除了订阅方，没人能报告刷新是否
真的发生。总线为此对每条事件报告一个 outcome，并由同一个值同时驱动 metric 与日志行，
两者不可能打架：

| Metric | label | `status` 取值 |
| --- | --- | --- |
| `config.bus.events` | `status`、`prefix` | `refreshed` · `ignored_prefix` · `malformed` · `refresh_error` |
| `config.bus.refresh.duration` | `status`、`prefix` | `refreshed` · `refresh_error` |
| `config.bus.publishes` | `status` | `ok` · `error` |

各 status 互斥，因此 `config.bus.events` 按 `status` 求和即为收到的广播数，不需要
另一个可能重复计数的计数器。其中 `refresh_error` 最值得告警：信号收到了也被放行，但
属性重载失败，实例停留在过期配置上。

`prefix` 是配置命名空间（如 `db`），不是属性 key——它的基数是舰队发布的命名空间数量，
由配置决定、有界。它同时挂在 `config.bus.events` / `config.bus.refresh.duration` 与
对应的日志行上，因此「哪个命名空间的刷新在失败」既能在看板上选出，也能 join 到解释它
的那一行。`config.bus.publishes` 属发布侧，不带 `prefix`。

trace 来自传输层：`Publish` 打开 producer span（父节点是你的 `ctx`）并把 W3C 上下文注入
消息 header，订阅侧的 consumer span 延续它，因此一次广播在追踪里表现为横跨发布方与
每个订阅者的单条链路。未引入 starter-otel 时两者都是 no-op。

健康检查：总线注册名为 `config-bus:configBus` 的 `health.Indicator`，报告订阅是否仍然
有效。它刻意不做连通性检查——订阅能挺过重连，所以它只在监听真的死掉时才为假，而这正是
NATS 连通性探针看不见、且会让实例静默停留在过期配置上的那种情况。连接层健康由
starter-nats 自己的指标负责，带有它自己的按实例 `health.enabled` 开关。

### 日志 tag

本模块的运行期日志使用 tag `_app_config_bus`（配置总线）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.config_bus.type=Logger
logger.config_bus.level=WARN
logger.config_bus.tag=_app_config_bus
```
