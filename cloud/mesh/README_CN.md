# mesh

[English](README.md) | [中文](README_CN.md)

`mesh` 在启动时回答一个问题：**当前进程是否跑在 service mesh sidecar 后面？**
如果在，发现和负载均衡已由 sidecar 完成，应用自己的客户端发现与负载均衡应该
让位，而不是把流量再均衡一遍。

[runtime 组合根](../runtime) 在启动时读取一次 `mesh.Enabled()`，把答案暴露成
`Runtime.UsesDiscovery(serviceName)`。client starter 只问这一个问题；回答为否时
直连配置的稳定地址，不再构建 discovery Resolver 或负载均衡 Pool。discovery 链路
本身（backend、`discovery.NewResolver`、`loadbalance`）零 mesh 引用——mesh 是一个
**寻址决策**，不是另一种服务发现。

## 安装

```
go get go-spring.org/cloud
```

## 使用方式：你配置，starter 反应

mesh 模式是部署的固定属性，由环境承载，而非运行时配置或代码。设置 `GS_MESH_MODE`：

| 取值          | 行为                                       |
|---------------|--------------------------------------------|
| `on`          | 强制开启——sidecar 负责发现 + 负载均衡        |
| `off`         | 强制关闭——客户端发现/负载均衡保持生效        |
| `auto` / 未设置 | 检测到 sidecar 才开启（默认）              |

```bash
# 在注入了 Istio 的 Kubernetes 里——无需任何配置：
# GS_MESH_MODE 未设置，auto-detect 看到 ISTIO_META_* 即开启 mesh 模式。

# 同一个应用部署在 mesh 外——同样无需配置；什么也检测不到。

# auto-detect 不适用时强制指定（比如排查双重负载均衡）：
export GS_MESH_MODE=off
```

mesh 模式开启时，配置了 `service-name` 的 entry 会忽略进程内发现，直接拨号自己的
稳定地址（`host:port`、ClusterDNS 名或固定 endpoint）。此时 `service-name` 只是
与 mesh 外部署共用配置时的遗留物——**忽略即可，不是错误**。关闭时，一切如同这个
包不存在。

## API

```go
import "go-spring.org/cloud/mesh"

mesh.Enabled() // bool — 由 Runtime.UsesDiscovery 查询，client 代码不直接调用
mesh.Detect()  // bool — 仅做 sidecar 探测（"auto" 的底层）；很少直接调用
```

- `Enabled()` 解析 `GS_MESH_MODE`：`on`/`off` 强制给出答案（大小写不敏感、忽略首尾
  空白）；其他任何取值——包括未设置和 `auto`——经 `Detect()` 从环境推断。
- `Detect()` 报告 sidecar 注入的环境变量是否存在（Istio/Envoy 的
  `ISTIO_META_*`、Linkerd 的 `LINKERD2_PROXY_*`）。无网络 I/O，启动期调用安全。

## 写一个感知 mesh 的 starter

```go
if center.UsesDiscovery(c.ServiceName) {
    // 进程内发现 + 负载均衡
} else {
    // 直连配置的稳定地址
}
```

整合就这么多：在 entry 决定寻址模式的地方问一次。不要在 client driver 里 import
`mesh`，也不要在 `discovery.NewResolver` 或任何内层 resolver/pool helper 里判断
——它们只会在 discovery 分支上运行。只要配置了稳定地址（`addr`/DSN/ClusterDNS
名），mesh 就让 entry 走直连分支。

## 设计说明

- `mesh.Enabled()` 是纯函数、无状态：不缓存、无副作用，只用 `os`/`strings`。
  空串与未设置在 `os.Getenv` 里无从区分，均落到 auto。
- 它只在 `Runtime.UsesDiscovery` 一处被查询，发生在 client 构造期而不是每次请求。
  因为是纯函数，不需要缓存、也不在 `Runtime` 上存字段。client 代码从不读环境变量。
- 没有 `SetEnabled`、没有三态 `atomic.Value`、没有 `starter-mesh` 模块。
  `cloud/mesh` 不塞 context、log 或 `sync.Once` 启动日志——可观测性归消费侧
  starter。
- 绝不在请求热路径上：`loadbalance` 零 mesh 引用，决策发生在 client 构造期，
  不在 `RoundTrip`/`Pick` 里。
- 测试在构造 runtime 前用 `t.Setenv("GS_MESH_MODE", ...)` 驱动；
  `Runtime.UsesDiscovery` 本身对字段做单测。
