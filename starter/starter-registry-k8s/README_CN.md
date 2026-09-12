# starter-registry-k8s

[English](README.md) | [中文](README_CN.md)

`starter-registry-k8s` 为 Go-Spring 提供 **Kubernetes 原生的客户端服务发现**。在
集群内，平台已经把每个 Pod 注册在 Service 之后，因此应用应当直接借助这套平台能力发
现对端，而不必再额外架一套外部注册中心（Nacos/Consul）造成能力重复。

匿名导入本 starter 并声明一个 `spring.registry.k8s.<name>` 命名块，即可得到一个名为
`k8s.<name>` 的 `discovery.Discovery` 后端 bean（来自 `cloud/discovery`）——块到
bean 的命名与家族其他注册中心后端（`consul.<name>`、`etcd.<name>`……）一致。任何支持
服务发现的 client starter——Redis、GORM 等——只需把自己的 `discovery: k8s.<name>`
字段指向它，就能把 Kubernetes **Service 名**解析成一组存活的 Pod 端点。本 starter
**只做客户端发现**，不提供 registrar（Pod 的注册由平台负责）。

## 两种模式

| 模式 | 机制 | 依赖 | RBAC | 取舍 |
| --- | --- | --- | --- | --- |
| `dns`（默认） | 通过 `net.Resolver` 解析 headless Service 的 DNS SRV/A 记录 | 无 | 无 | 零权限、简单；但变更感知受 DNS TTL + `refresh-interval` 限制，且无逐端点元数据。 |
| `endpointslice` | client-go informer 监听 `discovery.k8s.io/v1` EndpointSlice | client-go | `get/list/watch endpointslices` | 实时（扩缩容即刻触发），携带 Pod 元数据（zone、ready 状态）；需要 Kubernetes 客户端与 RBAC。 |

## 安装

```bash
go get go-spring.org/starter-registry-k8s
```

## 快速开始

### 1. 导入包

```go
import _ "go-spring.org/starter-registry-k8s"
```

### 2. 声明一个后端

DNS 模式，面向 headless Service，用命名端口做 SRV 查询：

```properties
spring.registry.k8s.main.mode=dns
spring.registry.k8s.main.namespace=default
spring.registry.k8s.main.port-name=grpc
spring.registry.k8s.main.cluster-domain=cluster.local
spring.registry.k8s.main.refresh-interval=5s
```

EndpointSlice 模式（实时；需要 RBAC——见 [example/deploy/rbac.yaml](example/deploy/rbac.yaml)）：

```properties
spring.registry.k8s.main.mode=endpointslice
spring.registry.k8s.main.namespace=default
spring.registry.k8s.main.port-name=grpc
# kubeconfig 留空则用集群内 ServiceAccount 鉴权；集群外运行时填写路径。
```

### 3. 在 client 中消费

上面的块声明的 bean 是 `k8s.main`，这就是 client 要引用的名字。例如 Redis 客户端
通过它解析地址：

```properties
spring.go-redis.instances.cache.service-name=my-redis   # Kubernetes Service 名
spring.go-redis.instances.cache.discovery=k8s.main       # 本后端
```

此时 Redis 客户端会拨向 `my-redis` Service 的存活 Pod，并随 Pod 上下线刷新。直接通过
注入后端 bean 解析 Service 的用法见 [example/example.go](example/example.go)。

**不要**为了让它注册本实例而设置 `spring.registry.service-name` / `.addr`：本后端不
提供 registrar，且在没有任何 registrar 后端时会由 registry 核心启动失败，而不是静默
地什么都不发布。

## 配置项

绑定在 `spring.registry.k8s.<name>` 下：

| 键 | 默认值 | 适用模式 | 说明 |
| --- | --- | --- | --- |
| `mode` | `dns` | 两者 | `dns` 或 `endpointslice`。 |
| `namespace` | `default` | 两者 | 目标 Service 所在命名空间。 |
| `port-name` | （空） | 两者 | 选择的命名端口。`dns` 模式下非空触发 SRV 查询，空则退回用 `port` 的 A 查询。 |
| `port` | `0` | 两者 | `port-name` 为空时使用的数字端口（`dns` A 记录模式必填）。 |
| `cluster-domain` | `cluster.local` | dns | 构造 Service FQDN 用的集群 DNS 后缀。 |
| `refresh-interval` | `10s` | dns | DNS watcher 重新解析以感知变更的间隔。 |
| `kubeconfig` | （空） | endpointslice | kubeconfig 路径；空则用集群内 ServiceAccount 鉴权。 |
| `resync-period` | `0` | endpointslice | informer 重新同步周期；`0` 表示纯事件驱动。 |

## 工作原理

- 块到 bean 的映射在容器的 Bean 注册阶段声明，早于任何 client 构造函数运行——因此
  Redis/GORM 客户端在配置里写 `k8s.<name>` 时，容器已经知道这个 bean。后端本身在注入
  时才构建，所以没人引用的块不花任何代价。
- **DNS 模式** 解析 `<service>.<namespace>.svc.<cluster-domain>`。设置了
  `port-name` 时发起 SRV 查询（`_<port-name>._tcp.<fqdn>`）拿到地址+端口；否则用 A
  查询并配上 `port`。watcher 按 `refresh-interval` 轮询，仅在端点集合变化时才推送快照。
- **EndpointSlice 模式** 运行一个 client-go 共享 informer，范围限定为该 Service 的
  EndpointSlice（标签 `kubernetes.io/service-name=<service>`）。每次 add/update/delete
  都从 informer 缓存重算快照。端点的 `Healthy` 来自 slice 的 `Ready` 条件，并带
  `zone` 元数据。
- 关闭时由后端 bean 的析构函数关闭后端；只有 informer 后端持有需要释放的资源。

### 日志 tag

本模块的运行期日志使用 tag `_app_registry_k8s`（k8s 注册中心）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.registry_k8s.type=Logger
logger.registry_k8s.level=WARN
logger.registry_k8s.tag=_app_registry_k8s
```

## 在集群内验证

单元测试用 fake resolver 与 client-go fake clientset 覆盖了两种模式。完整的端到端验证
需要真实集群：

```bash
kubectl apply -f example/deploy/demo-service.yaml   # 目标 Deployment + headless Service
kubectl apply -f example/deploy/rbac.yaml           # 仅 endpointslice 模式需要
# 为 example/ 构建并推送镜像，apply example/deploy/consumer.yaml，然后：
kubectl logs deploy/registry-k8s-example            # 打印解析出的 Pod 端点
kubectl scale deploy/demo --replicas=4              # 候选池实时更新
```
