# starter-config-consul

[English](README.md) | [中文](README_CN.md)

`starter-config-consul` 基于 github.com/hashicorp/consul/api，将
[Consul KV](https://developer.hashicorp.com/consul/docs/dynamic-app-config/kv)
接入为 Go-Spring 的**远程配置中心**。空导入该包即注册一个 `consul` 配置 Provider：
启动时从 Consul agent 拉取应用配置，并在运行时热更新——无需重启。

本 starter 只承担配置中心角色。基于 Consul catalog 的服务发现是独立能力，此处
不提供。

## 安装

```bash
go get go-spring.org/starter-config-consul
```

## 快速开始

### 1. 引入包

```go
import _ "go-spring.org/starter-config-consul"
```

### 2. 从 Consul 导入配置

在配置文件中使用 Provider 语法声明导入
`[optional:]consul:<host>:<port>/<kv-path>?<query>`：

```properties
spring.app.imports=optional:consul:127.0.0.1:8500/gs-config-demo?format=properties
```

查询参数：

| 参数         | 默认值                          | 说明                                                  |
|--------------|---------------------------------|-------------------------------------------------------|
| `format`     | KV 路径扩展名，否则 `properties` | 内容格式：`properties`/`yaml`/`toml`/`json`           |
| `scheme`     | `http`                          | `http` 或 `https`                                     |
| `token`      | （空）                          | ACL token；空即匿名                                    |
| `datacenter` | （agent 默认）                  | 数据中心覆盖                                          |

加上 `optional:` 前缀后，即使 KV 路径尚不存在应用也能正常启动；发布后其值会
被自动补全。

### 3. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

KV 值变更时，Provider 的阻塞查询 watcher 会触发一次应用属性刷新，所有绑定的
`gs.Dync` 字段都会被原子更新。完整的“发布 -> 热更新”流程参见
[example-config](example-config/example.go)。

## 工作原理

- 启动时，`spring.app.imports` 会调用 `consul` Provider：它从 source 串自建
  Consul API 客户端、读取 KV 路径，并对该路径启动一个阻塞查询 watcher。
- KV 变更会让阻塞查询的 `LastIndex` 递增，触发框架的 `PropertiesRefresher`：
  重新加载所有配置源（重跑本 Provider），并通过两阶段原子提交重新绑定所有
  `gs.Dync` 字段。
