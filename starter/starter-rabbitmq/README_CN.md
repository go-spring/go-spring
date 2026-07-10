# starter-rabbitmq

[English](README.md) | [中文](README_CN.md)

`starter-rabbitmq` 提供了基于 github.com/rabbitmq/amqp091-go 的 RabbitMQ 连接封装，
方便在 Go-Spring 服务中快速集成和使用 RabbitMQ。

## 安装

```bash
go get go-spring.org/starter-rabbitmq
```

## 快速开始

### 1. 引入 `starter-rabbitmq` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-rabbitmq"
```

### 2. 配置 RabbitMQ 实例

在项目的[配置文件](example/conf/app.properties)中添加 RabbitMQ 配置，比如：

```properties
spring.rabbitmq.url=amqp://guest:guest@127.0.0.1:5672/
```

### 3. 注入 RabbitMQ 连接

参见 [example.go](example/example.go) 文件。

```go
import amqp "github.com/rabbitmq/amqp091-go"

type Service struct {
    Conn *amqp.Connection `autowire:"__default__"`
}
```

### 4. 使用 RabbitMQ 连接

参见 [example.go](example/example.go) 文件。Channel 开销很小且非并发安全，
应从共享连接上按 goroutine/操作各自打开一个。

```go
ch, err := s.Conn.Channel()
defer ch.Close()
_, _ = ch.QueueDeclare("hello", false, false, false, false, nil)
_ = ch.PublishWithContext(ctx, "", "hello", false, false, amqp.Publishing{Body: []byte("value")})
```

## 高级功能

* **支持多 RabbitMQ 实例**：可以在配置文件的 `spring.rabbitmq.instances` 下定义多个 RabbitMQ 实例，并在项目中使用 name 进行引用。
