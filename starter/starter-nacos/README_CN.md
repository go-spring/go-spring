# starter-nacos

[English](README.md) | [中文](README_CN.md)

`starter-nacos` 提供了基于 github.com/nacos-group/nacos-sdk-go/v2 的 Nacos 客户端封装，
同时暴露命名（服务发现）客户端与配置（配置管理）客户端，方便在 Go-Spring 服务中使用。

## 安装

```bash
go get go-spring.org/starter-nacos
```

## 快速开始

### 1. 引入 `starter-nacos` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-nacos"
```

### 2. 配置 Nacos 实例

在项目的[配置文件](example/conf/app.properties)中添加 Nacos 配置，比如：

```properties
spring.nacos.ip-addr=127.0.0.1
spring.nacos.port=8848
```

### 3. 注入 Nacos 客户端

参见 [example.go](example/example.go) 文件。命名客户端与配置客户端都会被注册，
按接口类型注入所需的即可。

```go
import (
    "github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
    "github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
)

type Service struct {
    Config config_client.IConfigClient `autowire:"__default__"`
    Naming naming_client.INamingClient `autowire:"__default__"`
}
```

### 4. 使用 Nacos 客户端

参见 [example.go](example/example.go) 文件。

```go
_, err := s.Config.PublishConfig(vo.ConfigParam{DataId: "key", Group: "DEFAULT_GROUP", Content: "value"})
content, err := s.Config.GetConfig(vo.ConfigParam{DataId: "key", Group: "DEFAULT_GROUP"})
```

## 高级功能

* **命名 + 配置客户端**：`INamingClient` 与 `IConfigClient` 都会注册为 `__default__` Bean（接口类型不同）。
* **支持多命名实例**：可以在配置文件的 `spring.nacos.instances` 下定义多个命名客户端，并在项目中使用 name 进行引用。
