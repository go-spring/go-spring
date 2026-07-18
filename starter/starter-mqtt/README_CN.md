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

在项目的[配置文件](example/conf/app.properties)中添加 MQTT 配置，比如：

```properties
spring.mqtt.broker=tcp://127.0.0.1:1883
```

### 3. 注入 MQTT 客户端

参见 [example.go](example/example.go) 文件。

```go
import mqtt "github.com/eclipse/paho.mqtt.golang"

type Service struct {
    Client mqtt.Client `autowire:"__default__"`
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

* **支持多 MQTT 客户端**：可以在配置文件的 `spring.mqtt.instances` 下定义多个 MQTT 客户端，并在项目中使用 name 进行引用。
