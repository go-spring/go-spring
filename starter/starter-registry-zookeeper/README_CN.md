# starter-registry-zookeeper

[English](README.md) | [中文](README_CN.md)

`starter-registry-zookeeper` 把 ZooKeeper 适配为服务注册中心：每一个配置的集群都成为
一个后端 bean，同时服务命名习语的两半 —— 写侧（把**当前实例**注册为临时 znode）与读侧
（供解析其他实例的 `cloud/discovery` 后端）。它相当于 Spring Cloud 的 `ServiceRegistry` +
`DiscoveryClient` 的 Go-Spring 对应物，以临时节点（ephemeral znode）为底座。

适用于**虚机 / 裸机 / 混合**部署，即平台不替你注册实例的场景。**纯 Kubernetes**
下则完全用不到本 starter：平台已把每个 Pod 注册在 Service 之后，你用
[starter-registry-k8s](../starter-registry-k8s)（家族中只做发现的后端）去**发现**对端即可，无需注册。

本 starter 注册的是一个**朴素实例**（任意传输协议 —— HTTP、gRPC……）。RPC 框架的
provider 注册仍保持框架原生，不在本 starter 范围内（见
[starter/DESIGN_CN.md §3](../../DESIGN_CN.md)）。

## 命名块，一个中心一个 bean

配置是**命名块**：每个 ZooKeeper 集群一个 `spring.registry.zookeeper.<name>.*` 块。没有
默认块、没有匿名块 —— 名字就是地址的一部分。每个块成为**一个**名为
`zookeeper.<name>` 的后端 bean，同时实现 `discovery.Registrar`（写）与
`discovery.Discovery`（读），共享该块的会话与 base-path：

```properties
spring.registry.zookeeper.bz.servers=10.1.0.1:2181,10.1.0.2:2181
spring.registry.zookeeper.dr.servers=10.2.0.1:2181   # 第二个集群，bean 名 zookeeper.dr
```

两个块 = 一次注册同时写入两个集群；不同后端（zookeeper + nacos……）的块也可以在同一
应用里自由混用。

## 注册归 starter-registry 所有

导入本 starter 会传递导入注册核心 [starter-registry](../starter-registry)：唯一的
`registryServer`（`gs.Server`）收集**所有**后端的全部 registrar bean，在应用就绪后把实例
注册进**每一个**已配置的中心，停机开始时（经 `PreStop`）从所有中心注销，并广播
`UpdateWeight`。任一中心注册失败即启动失败，各中心的消费侧视图因此不会分裂。

注册意图由 `spring.registry.service-name`（连同 `spring.registry.addr`）表达，所有中心
共享。纯消费方应用只配置连接块，不注册任何实例。

## 安装

```bash
go get go-spring.org/starter-registry-zookeeper
```

## 快速上手

### 1. 匿名导入

```go
import _ "go-spring.org/starter-registry-zookeeper"
```

### 2. 配置 ZooKeeper 集群与实例

```properties
# 一个集群 = 一个命名块（设置 servers 即启用该块）。
spring.registry.zookeeper.main.servers=127.0.0.1:2181
spring.registry.zookeeper.main.session-timeout=10s
spring.registry.zookeeper.main.base-path=/services

# 要注册的实例（与后端无关，所有中心共享）。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可：启动时把实例写进每个已配置块，成为生命周期绑定该块会话的**临时节点**；停机时
删除节点。若进程异常退出，会话过期后 ZooKeeper 自动删除这些节点。

## 发现

消费方引用块的 **bean 名** —— 不存在任何 discovery 专属 key。该 bean 读侧惰性：从不引用
它的应用不为发现半区付出任何成本。

```properties
spring.registry.zookeeper.main.servers=127.0.0.1:2181
spring.http-client.backends.users.discovery=zookeeper.main
```

读写共享该块的 base-path，天然不会漂移。健康度由 znode 存活推导：实例是一个临时节点，
注册方会话一死 ZooKeeper 就把它删除，因此列举到的每个节点都是存活实例 —— 无需探测协议。
注册实例上可选的 `scheme` metadata 键承载传输选择。

## 配置项

连接配置，按块绑定于 `spring.registry.zookeeper.<name>`：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `servers` | （必填） | 集群成员；设置它即启用该块。 |
| `session-timeout` | `10s` | 会话超时；临时节点在会话存续期间存活。 |
| `base-path` | `/services` | 创建服务目录所在的持久父 znode。 |
| `username` | （空） | digest 认证用户名，设置即启用认证。 |
| `password` | （空） | digest 认证密码。 |

实例配置，绑定于 `spring.registry`（描述实例本身，与注册中心后端无关）：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | （空） | 要发布的逻辑名；设置它即表达注册意图。 |
| `addr` | （注册时必填） | 对外通告的可连 `host:port`。 |
| `id` | （空） | 实例 id 覆盖；空则由 `service-name` + `addr` 推导出稳定 id。 |
| `weight` | `100` | 随实例存储的负载均衡权重。 |
| `metadata.*` | （无） | 随实例存储的任意键值属性。 |

实例以 JSON（`service_name`、`addr`、`weight`、`metadata`）存储于
`<base-path>/<service-name>/<id>`，列举同一 base 路径的 discovery 后端即可还原成
`Endpoint`。

## 工作原理

- 每个块的后端 bean 立即构造：拨号该集群（探测 —— 一次 `Exists` 调用会阻塞到会话连上
  —— 不可达的 ZooKeeper 会让启动失败）并持有该块的会话，两个半区共享它。
- starter-registry 核心的 `registryServer` 收集每个后端的 registrar，等待就绪，然后把实例
  注册进每个中心：按需创建持久父目录，并把实例写成一个**临时**叶子节点。
- 停机时 `PreStop` 在 pre-stop 延迟之前向每个中心注销（删除 znode），让发现体系在在途
  请求仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。若进程崩溃，会话过期后
  ZooKeeper 自动删除这些节点。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测，再（在有 Docker 时）启动一个 ZooKeeper
节点，启动 [example](example/example.go)（注册、列举 znode、然后向自己发 SIGTERM 以触发
注销路径），并断言实例已出现。无 Docker 时优雅跳过。

### 运行时权重调整

`Server.UpdateWeight(ctx, weight)`（`registryServer` bean 上）不注销实例、仅以新权重在
**每一个**中心重新宣告：消费端（loadbalance 池）在下一个发现快照（一个 watch 推送周期）
生效。权重 0 即摘流（不接流量但保持注册），是下线前零损失轮转的标准一步。运维也可以直接
改某个注册中心里的权重值，效果等同。
