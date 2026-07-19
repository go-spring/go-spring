# messaging
[English](README.md) | [中文](README_CN.md)

`messaging` 是与框架无关、零依赖的发布/订阅抽象——Spring Cloud Stream binder
模型的 Go 惯用等价物。业务代码通过统一的 `Publisher` / `Subscriber` 对处理
`Message` 信封,切换 broker(NATS、Kafka、Pulsar、RabbitMQ、MQTT ...)只是
装配层变更,不需要改业务代码。

## 特性

- 抽象层零第三方依赖。
- `Message{Key, Payload, Headers, Timestamp}` broker 中立信封。`Headers` 天
  然作为 W3C trace-context 载体供可观测使用。
- `Publisher` / `Subscriber` 构造时即绑定 destination / source;`Binder`
  基于一个 broker 连接打开它们。
- `RegisterBinder` / `GetBinder` / `MustGetBinder`——driver-registry 范式,
  用于按配置名选择进程级 binder。broker starter 通常通过活 client 装配为 bean
  而不走注册表。
- 已有实现 `Binder` 的 broker starter:`starter-nats`、`starter-kafka`、
  `starter-kafka-sarama`、`starter-pulsar`、`starter-rabbitmq`、
  `starter-mqtt`。

## 快速开始

Import 路径: `go-spring.org/stdlib/messaging`。

```go
package main

import (
    "context"
    "log"

    "go-spring.org/stdlib/messaging"
)

func run(ctx context.Context, binder messaging.Binder) error {
    pub, err := binder.NewPublisher(ctx, "orders")
    if err != nil {
        return err
    }
    defer pub.Close()

    sub, err := binder.NewSubscriber(ctx, "orders", "order-workers")
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
}
```

`Binder` 由 broker starter(`starter-nats`、`starter-kafka` ...)提供;这些
starter 也会把原生 client bean(如 `*nats.Conn`、`*kgo.Client`)导出,作为
本抽象刻意不覆盖的 broker 专有能力的逃生舱。
