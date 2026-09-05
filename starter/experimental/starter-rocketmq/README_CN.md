# starter-rocketmq

[English](README.md) | [中文](README_CN.md)

`starter-rocketmq` 为 Go-Spring 提供 [RocketMQ](https://rocketmq.apache.org/)
支持：多实例 `rocketmq.Client` bean、启动期 fail-fast 探针、可选 ACL 凭证、
协议无关的 `messaging.Driver`、OTel 追踪助手，以及同步发送路径上的可选
调用点韧性。基于官方
[rocketmq-client-go](https://github.com/apache/rocketmq-client-go) v2 客户端，
通过 NameServer 协议同时支持 RocketMQ 4.x 与 5.x 集群。

## 安装

```bash
go get go-spring.org/starter-rocketmq
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-rocketmq"
```

### 2. 配置

```properties
spring.rocketmq.a.name-servers=127.0.0.1:9876
spring.rocketmq.a.send-timeout=5s

# ACL（可选，access-key 与 secret-key 必须成对设置）
# spring.rocketmq.a.access-key=rocketmq
# spring.rocketmq.a.secret-key=12345678
```

### 3. 注入

```go
type Service struct {
    Client *StarterRocketmq.Client `autowire:"a"`
}
```

### 4. 使用

```go
// 生产者（原生 SDK 路径，已启动）
p, err := s.Client.NewProducer()
defer p.Shutdown()
res, err := p.SendSync(ctx, primitive.NewMessage("topic", []byte("hello")))

// 推送消费者（原生 SDK 路径：先 Subscribe 再 Start）
c, err := s.Client.NewPushConsumer(consumer.WithGroupName("g"))
err = c.Subscribe("topic", consumer.MessageSelector{Type: consumer.TAG, Expression: "*"}, handler)
err = c.Start()
```

通过 client 创建的生产者与消费者自动继承配置里的名字服务地址、凭证和
实例名；应用关闭时统一由 starter 优雅停机。

## 核心特性

- **多实例客户端** — 每个 `spring.rocketmq.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **fail-fast 启动探针** — 启动期对名字服务列表做 TCP 拨号，第一条消息
  之前就暴露配错的地址（`fail-fast=false` 关闭）。
- **生命周期管理** — 经 client 创建的所有生产者/消费者都被登记，应用
  关闭时统一停机。
- **日志桥接** — 客户端库的内部日志汇入 go-spring 的日志。

## 消息 Driver

`NewDriver` 把 client 适配到协议无关的 `cloud/messaging`
抽象，业务代码不依赖 RocketMQ API。destination/source 是主题，group 映射
为 RocketMQ 消费组（集群模式）。

```go
func ProvideDriver(cl *StarterRocketmq.Client) messaging.Driver {
    return StarterRocketmq.NewDriver(cl)
}
```

```go
sub, _ := driver.NewSubscriber(ctx, "orders", "order-service")
_ = sub.Subscribe(ctx, func(ctx context.Context, msg *messaging.Message) error {
    fmt.Println(string(msg.Payload), msg.Headers)
    return nil // 返回错误即请求 RocketMQ 重投
})
pub, _ := driver.NewPublisher(ctx, "orders")
_ = pub.Publish(ctx, &messaging.Message{Key: "o-1", Payload: []byte("...")})
```

driver 会在消息 user properties 里注入/提取 W3C trace 上下文与压测标记，
因此链路能跨服务串联、合成流量在下游可识别。

## 可观测性

- **追踪**：`StartProducerSpan` / `StartConsumerSpan` / `EndSpan` 把原生
  发送与处理包成 OTel span；driver 路径自动埋点。全部依赖
  `starter-otel` 安装的全局 provider，未导入时是 no-op。见
  `example-otel/`。

## 韧性

`GuardedSend` 让同步发送走 client 上挂载的治理执行器（限流、熔断、
故障注入）：

```go
res, err := StarterRocketmq.GuardedSend(ctx, s.Client, p, msg)
```

未导入 `starter-governance` 时，它和 `p.SendSync(ctx, msg)` 行为完全一致。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.rocketmq.orders.name-servers=10.0.0.1:9876
spring.rocketmq.events.name-servers=10.0.0.2:9876
```

```go
type Service struct {
    Orders *StarterRocketmq.Client `autowire:"orders"`
    Events *StarterRocketmq.Client `autowire:"events"`
}
```

**自定义 driver** — 注册自己的 `Driver` 并用 `driver=<name>` 选中，替换
客户端装配过程（例如注入自定义 `primitive.NsResolver`）：

```go
func init() {
    StarterRocketmq.RegisterDriver("my-driver", myDriver{})
}
```

**健康检查** — 与其它 MQ starter 一致，这里不注册 `health.Indicator`
（没有在所有集群拓扑下都廉价可用的探针）；使用 `starter-actuator` 时由
应用自己导出（例如一次 `NewProducer`/`Shutdown` 往返）。
