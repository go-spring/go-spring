# starter-config-etcd

[English](README.md) | [中文](README_CN.md)

`starter-config-etcd` 基于 go.etcd.io/etcd/client/v3，将 [etcd](https://etcd.io/)
接入为 Go-Spring 的**远程配置中心**。空导入该包即注册一个 `etcd` 配置 Provider：
启动时从 etcd 集群拉取应用配置，并在运行时热更新——无需重启。

本 starter 只承担配置中心角色。服务发现（etcd naming）是独立能力，此处不提供。

## 安装

```bash
go get go-spring.org/starter-config-etcd
```

## 快速开始

### 1. 引入包

```go
import _ "go-spring.org/starter-config-etcd"
```

### 2. 从 etcd 导入配置

在配置文件中使用 Provider 语法声明导入
`[optional:]etcd:<host>:<port>/<key>?<query>`：

```properties
spring.app.imports=optional:etcd:127.0.0.1:2379/gs-config-demo?format=properties
```

查询参数：

| 参数           | 默认值                     | 说明                                              |
|----------------|----------------------------|---------------------------------------------------|
| `format`       | key 扩展名，否则 `properties` | 内容格式：`properties`/`yaml`/`toml`/`json`     |
| `username`     | （空）                     | 鉴权用户名                                        |
| `password`     | （空）                     | 鉴权密码                                          |
| `dial-timeout` | `5s`                       | 客户端拨号超时                                    |

加上 `optional:` 前缀后，即使 key 尚不存在应用也能正常启动；写入后其值会被
自动补全。

### 3. 绑定动态字段

将导入的配置项绑定到 `gs.Dync[T]` 字段即可实现实时更新：

```go
type Demo struct {
    Message gs.Dync[string] `value:"${demo.message:=none}"`
}
```

etcd key 变更时，Provider 的 watcher 会触发一次应用属性刷新，所有绑定的
`gs.Dync` 字段都会被原子更新。完整的“发布 -> 热更新”流程参见
[example-config](example-config/example.go)。

## 工作原理

- 启动时，`spring.app.imports` 会调用 `etcd` Provider：它从 source 串自建 clientv3、
  读取 key，并对该 key 安装一个 `etcd Watch`。
- key 变更会推送一次 watch 事件，触发框架的 `PropertiesRefresher`：重新加载所有
  配置源（重跑本 Provider），并通过两阶段原子提交重新绑定所有 `gs.Dync` 字段。
