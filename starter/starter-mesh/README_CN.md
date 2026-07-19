# starter-mesh

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布,欢迎使用!

`starter-mesh` 提供一个进程级全局开关:当应用运行在服务网格中时,把 Go-Spring 的
客户端服务发现(`stdlib/discovery`)与客户端负载均衡(`stdlib/loadbalance`)退化为
直通(pass-through)。

## 为什么需要

注入 sidecar(Istio/Envoy、Linkerd 等)后,它已经替你完成了服务发现与负载均衡。
如果应用自带的客户端发现与负载均衡继续叠加在上面,会导致:

- **双重负载均衡** —— 应用选一次、sidecar 再选一次,破坏 sidecar 的路由策略。
- **拓扑感知 / 离群剔除失效** —— 应用的 zone 亲和路由和离群剔除与网格的相互打架,
  故障域判断错乱。

打开 mesh 模式后,这两件事都交还给 sidecar:服务名解析为稳定的 Kubernetes Service
地址(即 sidecar 拦截的 ClusterIP),负载均衡器不再做选择。

## 安装

```bash
go get go-spring.org/starter-mesh
```

## 快速开始

### 1. 导入 `starter-mesh` 包

```go
import _ "go-spring.org/starter-mesh"
```

### 2. 打开 mesh 模式

```properties
spring.mesh.enabled=true
```

就这么简单。凡是通过 `stdlib/discovery` 用 `service-name` 解析的 client starter,
以及所有 `stdlib/loadbalance` 的 Pool,都会自动退化 —— 无需逐个组件改动。开关在启动
时被读取一次,早于任何 client 构造其 dialer。

## 何时开启

| 部署形态 | `spring.mesh.enabled` | 原因 |
| --- | --- | --- |
| Kubernetes **且注入了** sidecar(Istio/Envoy、Linkerd) | `true` | sidecar 已负责发现与负载均衡,应用不能再选一次。 |
| 虚拟机 / 裸机 / 任何**无网格**的部署 | `false`(默认) | 没有 sidecar,应用自带的客户端发现与负载均衡必须保持生效。 |

## 开启后有何变化

- **`stdlib/discovery`** —— `NewClientDialer` / `NewLiveDialer` 跳过对后端的
  Resolve 与 Watch,只暴露一个稳定端点,其地址即服务名。在 Kubernetes 中该名字经
  DNS 解析到 Service ClusterIP,由 sidecar 拦截并在各 Pod 间做负载均衡。
- **`stdlib/loadbalance`** —— `Pool` 直接返回这唯一端点,不做算法选择、不做离群剔除
  (唯一的网格端点绝不能被剔除,否则流量黑洞)。
- **链路追踪不受影响**:OTel 全局 propagator 仍然注入 header,应用与网格的 span 保持
  关联。
- **就绪语义不变** —— 无论是否开启 mesh 模式,探针行为一致。

## 不做的事

- 不对接网格控制面,不生成 `VirtualService` / `DestinationRule` —— 那属于部署脚手架,
  不在本 starter 范围内。
- 不删除客户端负载均衡代码,只在运行时退化;把开关关回去即恢复完整的客户端行为。

## 配置项

| 配置 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.mesh.enabled` | `false` | 打开服务网格模式。仅在注入了 sidecar 时开启。 |

## 示例

[example/main.go](example/main.go) 是一个自包含的冒烟测试(无需 docker、无需外部
服务)。它用同一份 client 代码跑两遍 —— mesh 关与 mesh 开 —— 并断言:关时请求均匀
打到三个真实端点,开时所有请求打到单个稳定端点且从不解析发现后端。用
`bash example/check.sh` 运行。

## 许可证

本项目基于 Apache License 2.0 许可证。
