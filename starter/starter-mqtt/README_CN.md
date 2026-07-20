# starter-mqtt

[English](README.md) | [中文](README_CN.md)

`starter-mqtt` 提供了基于 github.com/eclipse/paho.mqtt.golang 的 MQTT 客户端封装，
方便在 Go-Spring 服务中快速集成和使用 MQTT。

## 安装

```bash
go get go-spring.org/starter-mqtt
```

## 快速开始

### 1. 引入 `starter-mqtt` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-mqtt"
```

### 2. 配置 MQTT 客户端

在项目的[配置文件](example/conf/app.properties)中，在 `spring.mqtt.instances.<name>`
下定义一个或多个具名客户端，比如：

```properties
spring.mqtt.instances.a.broker=tcp://127.0.0.1:1883
spring.mqtt.instances.b.broker=tcp://127.0.0.1:1883
```

### 3. 注入 MQTT 客户端

参见 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`mqtt.Client` bean，按名称注入所需实例即可。

```go
import mqtt "github.com/eclipse/paho.mqtt.golang"

type Service struct {
    Client mqtt.Client `autowire:"a"`
}
```

### 4. 使用 MQTT 客户端

参见 [example.go](example/example.go) 文件。客户端在启动时建立连接、在关闭时断开连接，
因此可以直接进行发布和订阅。

```go
token := s.Client.Publish("go-spring/hello", 1, false, "value")
token.Wait()
_ = token.Error()
```

## 核心功能

[example](example/example.go) 演示了一次发布/订阅往返：以 QoS 1 订阅某个 topic，
向其发布一条消息，并断言消息被投递回订阅回调。发布前还会检查 `Client.IsConnected()`。

连接层事件（连接、连接丢失、重连中）会被桥接进 go-spring 日志。

## 可观测性

分布式链路追踪**不适用**于本 starter,这是刻意取舍而非缺失:

* `paho.mqtt.golang` 没有官方 OTel instrumentation。
* 更根本的原因是:该客户端所说的 MQTT 3.1.1 协议没有逐消息的元数据通道。
  `Client.Publish(topic, qos, retained, payload)` 没有任何位置可挂载 W3C
  `traceparent`,broker 也只投递原始 payload。能携带链路上下文的 User Properties
  仅存在于 MQTT 5,而此 paho v3 客户端并不支持。把链路上下文塞进业务 payload 或 topic
  会破坏消息契约,故不采用。

实际影响:生产者与消费者的 span 无法跨 broker 串联。连接层事件(连接、连接丢失、
重连中)仍会桥接进 go-spring 日志以供运维观测。

## 消息 Binder

除原生客户端外,本 starter 还可暴露一个 broker 中立的 `messaging.Binder`
(来自 `go-spring.org/spring/messaging`),让业务代码收发 `*messaging.Message`
信封而不依赖 paho MQTT API —— 底层换 broker 时业务代码无需改动。

从 `mqtt.Client` 注册一个 binder bean(用 `gs.TagArg` 选取具名实例):

```go
import (
    "go-spring.org/spring/gs"
    StarterMQTT "go-spring.org/starter-mqtt"
)

gs.Provide(StarterMQTT.NewBinder, gs.TagArg("a"))
```

然后通过信封收发:

```go
pub, _ := binder.NewPublisher(ctx, "orders")
defer pub.Close()
_ = pub.Publish(ctx, &messaging.Message{Payload: []byte("hello")})

sub, _ := binder.NewSubscriber(ctx, "orders", "")
defer sub.Close()
_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    // 处理 m.Payload
    return nil
})
```

`destination` 与 `source` 都是 topic,以 QoS 1 收发。该 binder 是 **payload-only**:
MQTT 3.1.1 无每消息元数据,所以 `Key`、`Headers`、`Timestamp` 不会上线,而且 ——
在所有 binder 中唯一地 —— **不发任何 trace context 或 span**(无处可骑)。`group`
不使用,因为 3.1.1 没有 shared subscription。paho 回调是 fire-and-forget、无 ack/nack,
因此 handler 出错只记日志。原生 `mqtt.Client` bean 仍可用于 retained 消息、自定义 QoS、
通配符 topic 等 binder 未建模的 MQTT 能力。

## 高级功能

* **多 MQTT 客户端**：`spring.mqtt.instances` 下的每一项都会成为一个独立配置的
  `mqtt.Client` bean，按名称注入即可访问不同的 broker。
* **TLS（MQTTS）**：设置 `spring.mqtt.instances.<name>.tls.enabled=true` 并使用
  `ssl://`/`tls://` broker URL 即可协商 TLS，可选地指定 CA 证书（`tls.ca-file`）
  并提供客户端证书（`tls.cert-file`/`tls.key-file`）以实现双向 TLS。
* **遗嘱消息（LWT）**：设置 `spring.mqtt.instances.<name>.will.topic`，当客户端非正常
  断开时由 broker 代为发布遗嘱消息。

## 配置项

`spring.mqtt.instances.<name>` 下每个客户端读取以下配置：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `broker` | （必填） | MQTT broker 地址，如 `tcp://127.0.0.1:1883`（MQTTS 用 `ssl://`）。 |
| `client-id` | `` | 客户端标识；为空时由库自动生成。 |
| `username` / `password` | `` | 鉴权凭据。 |
| `clean-session` | `true` | broker 是否在断连时丢弃会话状态。 |
| `keep-alive` | `30s` | PING 包之间的间隔。 |
| `connect-timeout` | `10s` | `Connect` 的超时上限；`0` 表示不限制。 |
| `tls.enabled` | `false` | 为 MQTTS 附加 `*tls.Config`。 |
| `tls.ca-file` | `` | 校验 broker 证书的 PEM CA 包；为空时用系统根证书。 |
| `tls.cert-file` / `tls.key-file` | `` | 双向 TLS 的客户端证书与私钥（须同时设置）。 |
| `tls.insecure-skip-verify` | `false` | 关闭 broker 证书校验（仅测试用）。 |
| `will.topic` | `` | 遗嘱 topic；为空则禁用遗嘱。 |
| `will.payload` | `` | 遗嘱消息内容。 |
| `will.qos` | `0` | 遗嘱投递 QoS（0、1 或 2）。 |
| `will.retained` | `false` | broker 是否保留遗嘱消息。 |
