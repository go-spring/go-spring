# starter-config-nacos

[English](README.md) | [中文](README_CN.md)

`starter-config-nacos` 基于 github.com/nacos-group/nacos-sdk-go/v2，将
[Nacos](https://nacos.io/) 接入为 Go-Spring 的**远程配置中心**。空导入该包即注册一个
`nacos` 配置 Provider：启动时直接从 Nacos 配置中心拉取应用配置，并在运行时热更新——
无需重启。

本 starter 只承担配置中心角色。服务发现（Nacos naming）是独立能力，此处不提供。

## 安装

```bash
go get go-spring.org/starter-config-nacos
```

## 快速开始

### 1. 引入包

```go
import _ "go-spring.org/starter-config-nacos"
```

### 2. 从 Nacos 导入配置

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

### 3. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

远程配置变更时，Provider 的变更监听器会触发一次应用属性刷新，所有绑定的 `gs.Dync`
字段都会被原子更新。完整的“发布 → 热更新”流程参见
[example-config](example-config/example.go)。

## 工作原理

- 启动时，`spring.app.imports` 会调用 `nacos` Provider：它从 source 串自建配置客户端、
  拉取 data id，并注册变更监听器。
- 远端变更触发监听器，回调框架的 `PropertiesRefresher`：重新加载所有配置源（重跑本
  Provider），并通过两阶段原子提交重新绑定所有 `gs.Dync` 字段。
