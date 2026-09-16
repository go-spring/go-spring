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
spring.config.import=optional:consul:127.0.0.1:8500/gs-config-demo?format=properties
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
[example-config](example/example.go)。

## 工作原理

- 启动时，`spring.config.import` 会调用 `consul` Provider：它从 source 串自建
  Consul API 客户端、读取 KV 路径，并对该路径启动一个阻塞查询 watcher。
- KV 变更会让阻塞查询的 `LastIndex` 递增，watch goroutine 的回调直接调用框架的
  进程级门面 `gs.RefreshProperties()`：重新加载所有配置源（重跑本 Provider），
  并通过两阶段原子提交重新绑定所有 `gs.Dync` 字段。

## 设计要点

**先注册 watcher 再做首次读取。** `registerWatch` 先于初始 `KV().Get` 执行，因此
带 `optional:` 且 key 尚不存在的 import 也能热更新——后续发布会触发刷新、重跑
provider。顺序颠倒是一次静默倒退，该先后次序被视为关键约束。

**配置与发现放在不同 starter。** `config` 与 `discovery` 位于不同层（provider
注册 vs. bean 装配），一个同时覆盖两者的大而全 starter 会搅浑模块依赖图。该拆分
对齐 Spring Cloud Alibaba 的 `nacos-config` / `nacos-discovery`。

**手写 blocking query，不用 `consul/api/watch` 库。** provider 只需要“索引自上次
是否变化”，一个阻塞 `KV().Get` 循环约 30 行即可表达，并让 watcher 结构与 etcd
版本保持一致。

### 日志 tag

本模块的运行期日志使用 tag `_app_config_consul`（consul 配置源）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.config_consul.type=Logger
logger.config_consul.level=WARN
logger.config_consul.tag=_app_config_consul
```
