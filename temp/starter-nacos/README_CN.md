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

## 核心功能

[示例](example/example.go) 端到端演示了 Nacos 的三大核心能力：

1. **配置发布 + 获取**：`PublishConfig` 写入配置，`GetConfig` 读取（轮询以容忍 Nacos 的异步传播）。
2. **配置监听**：`ListenConfig` 注册 `OnChange` 回调，随后的 `PublishConfig` 触发回调并推送新值。
3. **服务注册 + 发现**：`RegisterInstance` 注册服务实例，`GetService` 通过命名客户端完成发现。

## 远程配置 Provider

除了可注入的配置客户端外，`starter-nacos` 还向 Go-Spring 配置系统注册了一个 `nacos`
远程配置 Provider。它可以在启动时直接从 Nacos 配置中心拉取应用配置，并在运行时热更新——
无需重启。

### 1. 从 Nacos 导入配置

在配置文件中使用 Provider 语法声明导入
`[optional:]nacos:<host>:<port>/<dataId>?<query>`：

```properties
spring.app.imports=optional:nacos:127.0.0.1:8848/gs-config-demo?group=DEFAULT_GROUP&format=properties
```

查询参数：

| 参数         | 默认值           | 说明                                     |
|--------------|------------------|------------------------------------------|
| `group`      | `DEFAULT_GROUP`  | Nacos 配置分组                           |
| `namespace`  | （public）       | 命名空间 id                              |
| `format`     | data id 后缀，否则 `properties` | 内容格式：`properties`/`yaml`/`toml`/`json` |
| `username`   | （空）           | 鉴权用户名                               |
| `password`   | （空）           | 鉴权密码                                 |
| `timeout-ms` | `5000`           | 请求超时（毫秒）                         |

加上 `optional:` 前缀后，即使 data id 尚不存在应用也能正常启动；发布后其值会被自动补全。

### 2. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

远程配置变更时，Provider 的变更监听器会触发一次应用属性刷新，所有绑定的 `gs.Dync`
字段都会被原子更新。完整的“发布 → 热更新”流程参见
[example-config](example-config/example.go)。

## 高级功能

* **命名 + 配置客户端**：`INamingClient` 与 `IConfigClient` 都会注册为 `__default__` Bean（接口类型不同）。
* **支持多命名实例**：可以在配置文件的 `spring.nacos.instances` 下定义多个命名客户端，并在项目中使用 name 进行引用。
