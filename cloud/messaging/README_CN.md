# messaging
[English](README.md) | [中文](README_CN.md)

`messaging` 是与 broker 无关的发布/订阅抽象。业务代码通过统一的
`Publisher` / `Subscriber` 收发 `Message` 信封,换 broker(NATS、Kafka、
Pulsar、RabbitMQ、MQTT、...)只改接线,不改业务代码。

## API

| API | 作用 |
| --- | --- |
| `Message{Key, Payload, Headers, Timestamp}` | broker 无关的信封。`Payload` 是不透明 `[]byte`——编码在上一层做。`Key` 是可选的分区/有序键。`Headers` 承载元数据并兼作 W3C trace 上下文载体(`Header`/`SetHeader` nil 安全)。 |
| `Driver` | 在一条 broker 连接上开 publisher/subscriber;destination/source/group 字符串**按 broker 自己的术语**解释(subject、topic、queue、consumer group)。 |
| `Publisher` / `Subscriber` | 创建时各自绑定一个 destination/source。`Subscribe` 在投递建立后返回(不阻塞到投递结束);`Close` 释放该 subscriber/publisher,不动共享客户端。 |
| `Handler` | `func(ctx, *Message) error`——非 nil 返回表示投递失败;如何呈现(nack、重投、日志)是 broker 特定的,由各 starter 文档说明。 |
| `RegisterDriver` / `GetDriver` | 驱动注册表惯用法(空名/nil/重复即 panic),面向按配置名选进程级 driver 的调用方。starter 通常把 driver 作为 bean 接在活连接上。 |
| `Retry(h, RetryPolicy)` | 进程内按指数退避重试(`MaxRetries`、`InitialInterval`、`Multiplier`、`MaxInterval`);任一次成功即 ack,耗尽后返回错误交给 broker。零值 = 只跑一次。 |
| `DeadLetter(h, dlq, RetryPolicy)` | 重试耗尽后把消息发到 `dlq` publisher(绑到如 `"orders.dlq"`)并 ack 原消息。副本保留原 headers,另加 `x-dlq-error` / `x-dlq-retries` / `x-dlq-key`。DLQ 发送本身失败则返回原错误——丢死信比重投更糟。 |
| `Recover(h)` | 把 panic 的 handler 转成错误,一条坏消息杀不死投递循环。 |
| `NewMessageID()` / `HeaderMessageID` | 生成 32 位 hex id + 它所属的保留 header。至少一次投递下消费端去重是生产必做;以这个 id 为键。driver 的 publisher 侧经 `EnsureMessageID` 缺失即自动盖章——自己设了(或用业务键去重)它就是 no-op。 |
| `HeaderDeliveryAttempt` | 保留 header,driver 消费侧从 broker 重投计数填入(1 起;缺失或 "1" = 首次投递)。读它可以决定"重试"还是"直接进死信",不依赖进程内状态(重启即失)。 |
| `PublishBatch(ctx, p, msgs...)` | 一次调用批量发送。publisher 实现了可选的 `BatchPublisher` 能力则走批量,否则按序逐条发,遇第一个错误停止(非事务)。 |

实现 `Driver` 的 starter:`starter-nats`、`starter-kafka`、
`starter-kafka-sarama`、`starter-pulsar`、`starter-rabbitmq`、`starter-mqtt`、
`starter-rocketmq`。每个 starter 同时暴露原生 client bean(`*nats.Conn`、
`*kgo.Client`、...)作为逃生舱,承接本抽象刻意不建模的 broker 特性。

## 用法

### 1. 收发

```go
pub, err := driver.NewPublisher(ctx, "orders")
if err != nil {
    return err
}
defer pub.Close()

sub, err := driver.NewSubscriber(ctx, "orders", "order-workers")
if err != nil {
    return err
}
defer sub.Close()

_ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
    log.Printf("received %s: %s", m.Key, m.Payload)
    return nil
})

return pub.Publish(ctx, &messaging.Message{
    Key:     "order-1",
    Payload: []byte(`{"id":1}`),
})
```

`group` 留空表示广播(每个订阅者都收到),broker 支持才有;队列天然是
competing-consumer 的 broker(RabbitMQ)忽略该参数——各 starter 文档写明自
己的解释。

### 2. 幂等消费

```go
// driver 的 publisher 缺失时已自动盖 message-id;只有自己持有 id 才需要设:
msg.SetHeader(messaging.HeaderMessageID, messaging.NewMessageID())
// 消费侧——至少一次投递意味着同一个 id 可能到两次:
if seen := store.CheckOnce(m.Header(messaging.HeaderMessageID)); seen {
    return nil // 重复投递,ack 跳过
}
if m.Header(messaging.HeaderDeliveryAttempt) == "5" {
    // broker 已重投 5 次——跳过重试,直接进死信
}
```

### 3. 重试、死信、panic 防护

```go
dlq, _ := driver.NewPublisher(ctx, "orders.dlq")
_ = sub.Subscribe(ctx, messaging.DeadLetter(
    messaging.Recover(handle),
    dlq,
    messaging.RetryPolicy{MaxRetries: 2, InitialInterval: 100 * time.Millisecond},
))
```

包装顺序有讲究:`Retry(Recover(h), p)`(或如上 `DeadLetter` 包
`Recover`)让 panic 只转换一次错误,之后按普通失败重试。

带原生死信机制的 broker(RabbitMQ DLX、RocketMQ DLQ topic)可以配置成走原
生——两条路都保留原始 payload,只是失败元数据不同。

## 抽象刻意不建模的部分

- **不做延迟/定时消息。** 各家支持参差不齐;要 broker 无关的延迟投递,用
  `cloud/experimental/outbox` + `cloud/scheduling` 组合。
- **不做批量消费。** `Publish` 一次一条(发送侧可用 `PublishBatch` 批量);
  消费要吞吐,由 driver 内部批量、对外仍是逐条 Handler。
- **不做 schema registry、不做类型化 payload。** `Payload` 不透明,发布处自
  己序列化。
- **不做 Supplier/Function/Consumer 糖层**——原生 client 逃生舱始终一个
  import 之遥。
- MQTT(3.1.1)线上没有逐条消息元数据:其 starter 只传 payload、不做 trace
  透传。Kafka(franz-go)的 topic/group 固定在 client 上:一个 client bean =
  一个逻辑消费者;要多 group 就建多个 client(或用 sarama 变体)。
