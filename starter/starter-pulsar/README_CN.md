# starter-pulsar

[English](README.md) | [中文](README_CN.md)

`starter-pulsar` 基于 github.com/apache/pulsar-client-go 提供了 Pulsar 客户端封装,
方便在 Go-Spring 应用中集成和使用 Apache Pulsar。

## 安装

```bash
go get go-spring.org/starter-pulsar
```

## 快速开始

### 1. 引入 `starter-pulsar` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-pulsar"
```

### 2. 配置 Pulsar 客户端

在项目的[配置文件](example/conf/app.properties)中,在 `spring.pulsar.instances.<name>`
下定义一个或多个具名客户端,例如:

```properties
spring.pulsar.instances.a.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.b.url=pulsar://127.0.0.1:6650
```

### 3. 注入 Pulsar 客户端

参考 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`pulsar.Client` bean,按名称注入所需实例即可。

```go
import "github.com/apache/pulsar-client-go/pulsar"

type Service struct {
    Client pulsar.Client `autowire:"a"`
}
```

### 4. 使用 Pulsar 客户端

参考 [example.go](example/example.go) 文件。从共享的客户端创建生产者或消费者,
使用完毕后关闭它们。

```go
producer, _ := s.Client.CreateProducer(pulsar.ProducerOptions{Topic: "hello"})
defer producer.Close()
_, _ = producer.Send(ctx, &pulsar.ProducerMessage{Payload: []byte("value")})

consumer, _ := s.Client.Subscribe(pulsar.ConsumerOptions{
    Topic:            "hello",
    SubscriptionName: "hello-sub",
    Type:             pulsar.Shared,
})
defer consumer.Close()
msg, _ := consumer.Receive(ctx)
consumer.Ack(msg)
```

## 高级特性

* **多 Pulsar 客户端**:`spring.pulsar.instances` 下的每一项都会成为一个独立配置的
  `pulsar.Client` bean,按名称注入即可访问不同的集群。
