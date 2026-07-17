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

在项目的[配置文件](example/conf/app.properties)中添加 Kafka 配置,例如:

```properties
spring.kafka.brokers=127.0.0.1:9092
spring.kafka.topic=hello
spring.kafka.group=hello-group
```

### 3. 注入 Kafka 客户端

参考 [example.go](example/example.go) 文件。

```go
import "github.com/twmb/franz-go/pkg/kgo"

type Service struct {
    Client *kgo.Client `autowire:"__default__"`
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

* **支持多个 Kafka 客户端**:可以在配置文件中的 `spring.kafka.instances` 下定义多个
  Kafka 客户端,并通过名称引用它们。
