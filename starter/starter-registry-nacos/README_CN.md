# starter-registry-nacos

[English](README.md) | [中文](README_CN.md)

`starter-registry-nacos` 把 Nacos 适配为服务注册中心：每一个配置的 Nacos 服务端都成为
一个后端 bean，同时服务命名习语的两半 —— 写侧（把**当前实例**注册进 Nacos）与读侧
（供解析其他实例的 `cloud/discovery` 后端）。它相当于 Spring Cloud Alibaba
`nacos-discovery` 的 Go-Spring 对应物，也是 [starter-config-nacos](../starter-config-nacos)
配置角色的注册角色对应物（两者是配置前缀各自独立的两个 starter）。

适用于**虚机 / 裸机 / 混合**部署，即平台不替你注册实例的场景。**纯 Kubernetes**
下则完全用不到本 starter：平台已把每个 Pod 注册在 Service 之后，你用
[starter-discovery-k8s](../starter-discovery-k8s) 去**发现**对端即可，无需注册。

本 starter 注册的是一个**朴素实例**（任意传输协议 —— HTTP、gRPC……）。RPC 框架的
provider 注册仍保持框架原生，不在本 starter 范围内（见
[starter/DESIGN_CN.md §3](../../DESIGN_CN.md)）。

## 命名块，一个中心一个 bean

配置是**命名块**：每个 Nacos 服务端一个 `spring.registry.nacos.<name>.*` 块。没有默认
块、没有匿名块 —— 名字就是地址的一部分。每个块成为**一个**名为 `nacos.<name>` 的后端
bean，同时实现 `discovery.Registrar`（写）与 `discovery.Discovery`（读），共享该块的
client 与 namespace/group/cluster：

```properties
spring.registry.nacos.main.server=10.0.0.1:8848
spring.registry.nacos.dr.server=10.0.2.1:8848   # 第二个中心，bean 名 nacos.dr
```

两个块 = 一次注册同时写入两个 Nacos（Dubbo 式默认双写）；不同后端（nacos +
zookeeper……）的块也可以在同一应用里自由混用。

## 注册归 starter-registry 所有

导入本 starter 会传递导入注册核心 [starter-registry](../starter-registry)：唯一的
`registryServer`（`gs.Server`）收集**所有**后端的全部 registrar bean，在应用就绪后把实例
注册进**每一个**已配置的中心，停机开始时（经 `PreStop`）从所有中心注销，并广播
`UpdateWeight`。任一中心注册失败即启动失败，各中心的消费侧视图因此不会分裂。

注册意图由 `spring.registry.service-name`（连同 `spring.registry.addr`）表达，所有中心
共享。纯消费方应用只配置连接块，不注册任何实例。

## 安装

```bash
go get go-spring.org/starter-registry-nacos
```

## 快速上手

### 1. 匿名导入

```go
import _ "go-spring.org/starter-registry-nacos"
```

### 2. 配置 Nacos 服务与实例

```properties
# 一个 Nacos 服务端 = 一个命名块（设置 server 即启用该块）。
spring.registry.nacos.main.server=127.0.0.1:8848
spring.registry.nacos.main.group=DEFAULT_GROUP
spring.registry.nacos.main.cluster=DEFAULT

# 要注册的实例（与后端无关，所有中心共享）。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可：启动时把实例注册为每个已配置块中的**临时（ephemeral）实例**，由 SDK 自身的
心跳保活；停机时处处注销。

## 发现

消费方引用块的 **bean 名** —— 不存在任何 discovery 专属 key。该 bean 读侧惰性：从不引用
它的应用不为发现半区付出任何成本。

```properties
spring.registry.nacos.main.server=127.0.0.1:8848
spring.http-client.backends.users.discovery=nacos.main
```

读写共享该块的 namespace/group/cluster，天然不会漂移。

## 配置项

连接配置，按块绑定于 `spring.registry.nacos.<name>`：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `server` | （必填） | Nacos 服务 `host:port`；设置它即启用该块。 |
| `namespace` | （空） | 注册到的 namespace id，空则用 `public`。 |
| `group` | `DEFAULT_GROUP` | 服务分组；客户端须在同一分组下解析。 |
| `cluster` | `DEFAULT` | 实例所属的 Nacos 集群名。 |
| `username` | （空） | 认证用户名，匿名集群留空。 |
| `password` | （空） | 认证密码。 |
| `timeout-ms` | `5000` | 每次调用超时，含启动探测。 |

实例配置，绑定于 `spring.registry`（描述实例本身，与注册中心后端无关）：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | （空） | 要发布的逻辑名；设置它即表达注册意图。 |
| `addr` | （注册时必填） | 对外通告的可连 `host:port`。Nacos 按 ip:port 识别实例，因此没有 id key。 |
| `weight` | `100` | 负载均衡权重；写入时 `<=0` 归一为 `1`。 |
| `metadata.*` | （无） | 随实例存储的任意键值属性。 |

## 工作原理

- 每个块的后端 bean 立即构造：构建 naming client 并探测服务端（列举服务），配置错误或
  不可达的 Nacos 会让启动失败，每块一次。
- starter-registry 核心的 `registryServer` 收集每个后端的 registrar，等待就绪，然后把实例
  作为**临时实例**注册进每个中心。Nacos SDK 以后台心跳保活；若进程未注销就退出，Nacos
  会自动摘除。
- 停机时 `PreStop` 在 pre-stop 延迟之前向每个中心注销，让发现体系在在途请求仍被服务时就
  摘掉它。`Stop` 作为幂等兜底再次注销。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测，再（在有 Docker 时）启动一个 Nacos
standalone 服务，启动 [example](example/example.go)（注册、回读命名服务、然后向自己发
SIGTERM 以触发注销路径），并断言实例已出现。无 Docker 时优雅跳过。

### 运行时权重调整

`Server.UpdateWeight(ctx, weight)`（`registryServer` bean 上）不注销实例、仅以新权重在
**每一个**中心重新宣告：消费端（loadbalance 池）在下一个发现快照生效。权重 0 即摘流
（不接流量但保持注册），是下线前零损失轮转的标准一步。运维也可以直接改某个注册中心
里的权重值，效果等同。

### 日志 tag

本模块的运行期日志使用 tag `_app_registry_nacos`（nacos 注册中心）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.registry_nacos.type=Logger
logger.registry_nacos.level=WARN
logger.registry_nacos.tag=_app_registry_nacos
```
