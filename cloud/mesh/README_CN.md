# mesh

[English](README.md) | [中文](README_CN.md)

`mesh` 在启动时回答一个问题：**当前进程是否跑在 service mesh sidecar 后面？**
如果在，发现和负载均衡已由 sidecar 完成，应用自己的客户端发现与负载均衡应该
让位，而不是把流量再均衡一遍。

starter 读取 `mesh.Enabled()`，为 on 时直连服务的稳定 DNS 地址，不再构建
discovery Resolver 或负载均衡 Pool。通常你不需要自己调用这个包——设置一个
环境变量，starter 会自行反应。

## 安装

```
go get go-spring.org/cloud
```

## 使用方式：你配置，starter 反应

mesh 模式是部署的固定属性，由环境承载，而非运行时配置或代码。设置 `GS_MESH`：

| 取值          | 行为                                       |
|---------------|--------------------------------------------|
| `on`          | 强制开启——sidecar 负责发现 + 负载均衡        |
| `off`         | 强制关闭——客户端发现/负载均衡保持生效        |
| `auto` / 未设置 | 检测到 sidecar 才开启（默认）              |

```bash
# 在注入了 Istio 的 Kubernetes 里——无需任何配置：
# GS_MESH 未设置，auto-detect 看到 ISTIO_META_* 即开启 mesh 模式。

# 同一个应用部署在 mesh 外——同样无需配置；什么也检测不到。

# auto-detect 不适用时强制指定（比如排查双重负载均衡）：
export GS_MESH=off
```

mesh 模式开启时，配置了 `service-name` 的 starter 会忽略客户端发现，直接拨号
稳定地址（`host:port` / ClusterDNS 名）。关闭时，一切如同这个包不存在。

## API

```go
import "go-spring.org/cloud/mesh"

mesh.Enabled() // bool — starter 唯一需要的调用
mesh.Detect()  // bool — 仅做 sidecar 探测（"auto" 的底层）；很少直接调用
```

- `Enabled()` 解析 `GS_MESH`：`on`/`off` 强制给出答案（大小写不敏感、忽略首尾
  空白）；其他任何取值——包括未设置和 `auto`——经 `Detect()` 从环境推断。
- `Detect()` 报告 sidecar 注入的环境变量是否存在（Istio/Envoy 的
  `ISTIO_META_*`、Linkerd 的 `LINKERD2_PROXY_*`）。无网络 I/O，启动期调用安全。

## 写一个感知 mesh 的 starter

```go
useDiscovery := c.ServiceName != "" && !mesh.Enabled()
```

用 `!mesh.Enabled()` 作为是否构建客户端发现/LB 的闸门；mesh 模式下回退到配置的
静态地址。对于经 `discovery` 解析的基础设施客户端，优先用
`discovery.NewResolver`——它已内置该检查，mesh 模式下返回 nil Resolver，调用方
直接拨号配置地址。
