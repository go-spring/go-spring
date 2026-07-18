# starter-kafka-sarama

[English](README.md) | [中文](README_CN.md)

`starter-kafka-sarama` 基于 github.com/IBM/sarama 提供了 Kafka 客户端封装,
方便在 Go-Spring 应用中集成和使用 Kafka。

它与基于 franz-go 的 [starter-kafka](../starter-kafka) 共用 `spring.kafka`
配置前缀,因此在两种实现之间切换时只需替换所导入的包。

## 安装

```bash
go get go-spring.org/starter-kafka-sarama
```

## 快速开始

### 1. 引入 `starter-kafka-sarama` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-kafka-sarama"
```

### 2. 配置 Kafka 客户端

在项目的[配置文件](example/conf/app.properties)中添加 Kafka 配置,例如:

```properties
spring.kafka.instances.a.brokers=127.0.0.1:9092
spring.kafka.instances.a.version=3.7.0
spring.kafka.instances.b.brokers=127.0.0.1:9092
```

> `spring.kafka.instances.<name>` 下的每个条目都会注册为一个以该名字命名、
> 独立配置的 `sarama.Client` bean。
> `version` 需与目标集群匹配,消费组等特性才能正常工作;
> 不填时使用 sarama 自带的默认版本。

### 3. 注入 Kafka 客户端

参考 [example.go](example/example.go) 文件,按名字注入对应实例。

```go
import "github.com/IBM/sarama"

type Service struct {
    Client sarama.Client `autowire:"a"`
}
```

### 4. 使用 Kafka 客户端

参考 [example.go](example/example.go) 文件。sarama 没有同时生产与消费的单一对象,
需从共享的 `sarama.Client` 通过 `*FromClient` 构造函数派生生产者或消费者:

```go
producer, _ := sarama.NewSyncProducerFromClient(s.Client)
defer producer.Close()
producer.SendMessage(&sarama.ProducerMessage{
    Topic: "hello",
    Value: sarama.StringEncoder("value"),
})

consumer, _ := sarama.NewConsumerFromClient(s.Client)
defer consumer.Close()
pc, _ := consumer.ConsumePartition("hello", 0, sarama.OffsetOldest)
defer pc.Close()
msg := <-pc.Messages()
fmt.Println(string(msg.Value))
```

## 高级特性

* **支持多个 Kafka 客户端**:可以在配置文件中的 `spring.kafka.instances` 下定义多个
  Kafka 客户端,并通过名称引用它们。
