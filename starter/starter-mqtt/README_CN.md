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
向其发布一条消息，并断言消息被投递回订阅回调。

## 高级功能

* **多 MQTT 客户端**：`spring.mqtt.instances` 下的每一项都会成为一个独立配置的
  `mqtt.Client` bean，按名称注入即可访问不同的 broker。
