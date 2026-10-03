# starter-rocketmq

[English](README.md) | [中文](README_CN.md)

`starter-rocketmq` 为 Go-Spring 提供 [RocketMQ](https://rocketmq.apache.org/)
支持：多实例 `rocketmq.Client` bean、可选的启动期 ping 探针、可选 ACL 凭证、
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
spring.rocketmq.instances.a.name-servers=127.0.0.1:9876
spring.rocketmq.instances.a.send-timeout=5s

# ACL（可选，access-key 与 secret-key 必须成对设置）
# spring.rocketmq.instances.a.access-key=rocketmq
# spring.rocketmq.instances.a.secret-key=12345678
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

- **多实例客户端** — 每个 `spring.rocketmq.instances.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **启动 ping 探针（可选）** — `ping=true` 时启动期对名字服务列表做 TCP 拨号，第一条消息
  之前就暴露配错的地址；默认关闭，broker 未就绪不阻塞启动。
- **生命周期管理** — 经 client 创建的所有生产者/消费者都被登记，应用
  关闭时统一停机。
- **日志桥接** — 客户端库的内部日志汇入 go-spring 的日志。

## 消息 Driver

`NewDriver` 把 client 适配到协议无关的 `cloud/messaging`
抽象，业务代码不依赖 RocketMQ API。destination/source 是主题，group 映射
为 RocketMQ 消费组（集群模式）。

```go
func ProvideDriver(cl *StarterRocketmq.Client) messaging.Driver {
    return StarterRocketmq.NewDriver(cl, nil)
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

每次 publish 与 consume 都会声明其操作并在 client 的 resilience executor（唯一发射点，
见[可观测性](#可观测性)）下执行，同时在消息 user properties 里注入/提取 W3C trace 上下文
与压测标记，因此链路能跨服务串联、合成流量在下游可识别。

## 可观测性

所有经由本 starter 的 publish 与 consume 都会在调用上**声明（declare）**该操作的语义身份
——方向（`messaging.operation`）、后端（`messaging.system=rocketmq`）以及作为逐调用细节的
topic —— 而由 resilience 层（executor 链上**唯一的发射点（emitter）**）依据声明发射信号：
调用 span、调用级 `messaging.client.operation.duration` 直方图、尝试级
`messaging.client.attempt.duration` 直方图（每次重试一条记录，因此重试/退避开销不会
计入下游时延）、in-flight 计量、`resilience.client.calls` 计数器，以及每次调用一条访问
日志。messaging.Driver 路径与原生 `GuardedSend` 接缝都走同一个声明辅助函数，因此两者
不会对同一条消息重复上报。这些信号依赖 [starter-otel](../starter-otel) 安装的全局
`TracerProvider`；未引入 starter-otel 时 span 与指标为空操作，访问日志则始终经 go-spring
的 log 写出。

starter 自身不再做任何逐调用发射 —— 它只声明身份，由 resilience 层发射。

自行驱动原生发送或消费者时，可用手动辅助函数 `StartProducerSpan` /
`StartConsumerSpan` / `EndSpan` 将其包成 OTel span（W3C 上下文随消息 user properties
传递）。见 `example-otel/`。

## 韧性

`GuardedSend` 让同步发送走 client 上挂载的治理执行器（限流、熔断、
故障注入）：

```go
res, err := StarterRocketmq.GuardedSend(ctx, s.Client, p, msg)
```

治理未接入（容器里没有 `cloud/governance` 的 bean）时，它和 `p.SendSync(ctx, msg)` 行为完全一致。

[消息 Driver](#消息-driver)的 publish 与 consume 路径同样经由该 executor，因此
driver 流量也受到保护并被声明。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.rocketmq.instances.orders.name-servers=10.0.0.1:9876
spring.rocketmq.instances.events.name-servers=10.0.0.2:9876
```

```go
type Service struct {
    Orders *StarterRocketmq.Client `autowire:"orders"`
    Events *StarterRocketmq.Client `autowire:"events"`
}
```

**自定义 driver** — 提供自己的 `Driver` bean（其构造函数返回
`StarterRocketmq.Driver`）替换客户端装配过程（例如注入自定义
`primitive.NsResolver`）。它是可选的容器 bean：`spring.rocketmq.instances.*` 下每个
客户端都经它构建，仅当没有 `Driver` bean 时才回退到内置 `DefaultDriver`。
内嵌 `StarterRocketmq.DefaultDriver` 并委托 `CreateClient` 以保留默认装配。
`CreateClient` 接收容器的 `cloud.ClientParams` bundle（见
`go-spring.org/cloud`）并原样传递，因此客户端在一步内装配完整——
身份与 executor 同时就位；照原样委托即可让治理继续生效：

```go
func init() {
    gs.Provide(func() StarterRocketmq.Driver {
        return myDriver{}
    })
}

type myDriver struct {
    StarterRocketmq.DefaultDriver
}

func (d myDriver) CreateClient(ctx context.Context, c StarterRocketmq.Config,
    params cloud.ClientParams) (*StarterRocketmq.Client, error) {
    return d.DefaultDriver.CreateClient(ctx, c, params)
}
```

**健康检查** — 与其它 MQ starter 一致，这里不注册 `health.Indicator`
（没有在所有集群拓扑下都廉价可用的探针）；使用 `starter-actuator` 时由
应用自己导出（例如一次 `NewProducer`/`Shutdown` 往返）。

## 设计说明

* **顺序与事务消息留在原生 SDK。** `messaging.Driver` 适配器是并发消费的；顺序或事务发送请用原生
  `Client`。消息 payload 保持 `[]byte`——序列化归你。
* **`instance-name` 控制连接池共享。** 留空时 SDK 把共享的 "DEFAULT" 改写为每个 producer/consumer
  的 `PID#nano`，在多生产者进程里是安全的；显式设置则让所有 remoting 客户端共享一个连接池。
* **ping 探针是 TCP 拨号，不是 broker 往返。** 它能在启动期抓出配错的地址，但抓不到 ACL 或凭据错误——
  刻意廉价、无副作用、与拓扑无关。
