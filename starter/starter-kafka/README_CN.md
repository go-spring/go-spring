# starter-kafka

[English](README.md) | [中文](README_CN.md)

`starter-kafka` 基于 github.com/twmb/franz-go 提供了 Kafka 客户端封装,
方便在 Go-Spring 应用中集成和使用 Kafka。

## 安装

```bash
go get go-spring.org/starter-kafka
```

## 快速开始

### 1. 引入 `starter-kafka` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-kafka"
```

### 2. 配置 Kafka 客户端

在项目的[配置文件](example/conf/app.properties)中,在 `spring.kafka.instances.<name>`
下定义一个或多个具名客户端,例如:

```properties
spring.kafka.instances.a.brokers=127.0.0.1:9092
spring.kafka.instances.a.topic=hello
spring.kafka.instances.a.group=hello-group
spring.kafka.instances.b.brokers=127.0.0.1:9092
```

### 3. 注入 Kafka 客户端

参考 [example.go](example/example.go) 文件。每个具名实例都会以该名称注册为一个
`*kgo.Client` bean,按名称注入所需实例即可。

```go
import "github.com/twmb/franz-go/pkg/kgo"

type Service struct {
    Client *kgo.Client `autowire:"a"`
}
```

### 4. 使用 Kafka 客户端

参考 [example.go](example/example.go) 文件。同一个 `*kgo.Client` 即可生产与消费:
生产使用 `ProduceSync`,消费使用 `PollFetches`。

```go
rec := &kgo.Record{Topic: "hello", Value: []byte("value")}
_ = s.Client.ProduceSync(ctx, rec).FirstErr()

fetches := s.Client.PollFetches(ctx)
fetches.EachRecord(func(r *kgo.Record) {
    fmt.Println(string(r.Value))
})
```

## 高级特性

* **多 Kafka 客户端**:`spring.kafka.instances` 下的每一项都会成为一个独立配置的
  `*kgo.Client` bean,按名称注入即可访问不同的集群或消费者组。
