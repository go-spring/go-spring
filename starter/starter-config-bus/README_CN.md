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
spring.nats.config-bus.url=nats://127.0.0.1:4222
```

### 3. 配置总线（可选）

所有配置项位于 `spring.config.bus` 前缀下:

| 配置项           | 默认值                  | 说明                                                                     |
|------------------|-------------------------|--------------------------------------------------------------------------|
| `subject`        | `spring.config.refresh` | 发布与订阅刷新事件的 NATS 主题。                                          |
| `nats-instance`  | `config-bus`            | 作为传输通道的 `spring.nats.*` 连接名。                        |
| `watch-prefixes` | (空)                    | 逗号分隔的前缀;设置后,仅当广播前缀与其中之一有交集（或为全量广播）时,本实例才刷新。 |

### 4. 广播刷新

通过 `autowire:"configBus"` 注入总线并发布:

```go
type Service struct {
    Bus *StarterConfigBus.ConfigBus `autowire:"configBus"`
}

// 全量刷新:所有订阅者都重载。
_ = svc.Bus.Publish("")

// 按前缀刷新:带前缀过滤的订阅者可跳过。
_ = svc.Bus.Publish("db")
```

订阅该主题的每个实例都会重新执行 `RefreshProperties`,所有绑定的 `gs.Dync` 字段随之
热更新。完整的广播 → 刷新流程见 [example](example/example.go)。

## 工作原理

- 启动时 `ConfigBus` bean 被急切创建（以名字 `configBus` 导出 `gs.Rooter`）,并在
  配置的 NATS 连接上订阅 `spring.config.bus.subject`。
- `Publish(prefix)` 在主题上发送一条精简的 JSON `RefreshEvent{prefix}`。空前缀表示
  全量刷新;非空前缀允许带前缀过滤的订阅者跳过。
- 收到消息后,每个订阅者调用框架的 `PropertiesRefresher`,重新加载所有配置源,并通过
  两阶段原子提交重新绑定每个 `gs.Dync` 字段。
