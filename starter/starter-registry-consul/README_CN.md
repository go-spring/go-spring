# starter-registry-consul

[English](README.md) | [中文](README_CN.md)

`starter-registry-consul` 把**当前实例**注册进 Consul 服务注册中心 —— 它是
Go-Spring 客户端服务发现(`cloud/discovery`)的注册侧对应物 —— **并且**提供消费侧的
Consul discovery 后端,一个 starter 同时覆盖命名两半。相当于 Spring Cloud 的
`ServiceRegistry` + `DiscoveryClient`,以 Consul TTL 健康检查为底。

适用于**虚机 / 裸机 / 混合**部署,即平台不替你注册实例的场景。**纯 Kubernetes**
下则完全用不到本 starter:平台已把每个 Pod 注册在 Service 之后,你用
[starter-registry-k8s](../starter-registry-k8s)（家族中只做发现的后端）去**发现**对端即可,无需注册。

本 starter 注册的是一个**朴素实例**(任意传输协议 —— HTTP、gRPC……)。RPC 框架的
provider 注册仍保持框架原生,不在本 starter 范围内(见
[starter/DESIGN_CN.md §3](../../DESIGN_CN.md))。

## 模型

配置是**命名块**:每个 `spring.registry.consul.<name>.*` 块描述一个 Consul
agent/集群,并成为一个名为 `consul.<name>` 的后端 bean,同时实现写侧
(`discovery.Registrar`)与读侧(`discovery.Discovery`),共享该块的客户端。没有默认/无名块。

注册本身由 [starter-registry](../starter-registry) 核心拥有(传递依赖自动引入):它
唯一的 `registryServer`(`gs.Server`)收集**所有**已配置中心的 registrar bean——跨后端
——在应用就绪后把实例注册进每一个中心,停机时从全部中心注销,并向所有中心广播
`UpdateWeight`。配两个块(consul + consul,或 consul + etcd……)再加
`spring.registry.service-name`,即得双中心/多中心注册,零额外配置。

本 starter 不开端口:导出的 `gs.Server` 纯粹为了让注册接入服务生命周期——**应用就绪
后**发布实例,**停机开始时**(经 `PreStop`)注销,使发现体系在实例真正停止服务之前
就把它摘除。正是这个顺序让滚动重启无损。

## 安装

```bash
go get go-spring.org/starter-registry-consul
```

## 快速上手

### 1. 匿名导入

```go
import _ "go-spring.org/starter-registry-consul"
```

### 2. 配置 Consul agent 与实例

```properties
# 每个命名块一个 Consul agent;"main" 是块名(自选)。
spring.registry.consul.main.address=127.0.0.1:8500
spring.registry.consul.main.ttl=15s
spring.registry.consul.main.deregister-critical-after=1m

# 第二个块 = 第二个中心 = 双注册(零额外配置)。
# spring.registry.consul.dr.address=10.9.0.1:8500

# 要注册的实例(与后端无关,所有中心共享)。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可:启动时把实例注册进**每一个**已配置 agent 并由 TTL 心跳保活,停机时全部
注销。每个块的 bean 同时是名为 `consul.<name>` 的 discovery 后端(见下文),别处的
客户端 —— 或本进程自身 —— 经共享连接按服务名解析到它。

## 配置项

### 注册(本实例)

连接配置,按块绑定于 `spring.registry.consul.<name>`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `address` | (必填) | Consul HTTP API 地址;设置一个块即激活它。 |
| `scheme` | `http` | `http` 或 `https`。 |
| `datacenter` | (空) | 注册到的 datacenter,空则用 agent 的。 |
| `token` | (空) | ACL token。 |
| `namespace` | (空) | Consul Enterprise namespace。 |
| `ttl` | `15s` | TTL 健康检查;starter 以 TTL 一半的间隔心跳。 |
| `deregister-critical-after` | `1m` | 检查持续 critical 超过此时长(如崩溃后),Consul 自动摘除实例。 |

每个块成为**一个**后端 bean `consul.<name>`——一个客户端、一次启动探测、一个生命周期;
命名体系的两半共享它们,读写永不分裂。**只有设置了 `service-name` 才会注册**;纯消费方
应用省略该键,不注册任何实例。

实例配置,绑定于 `spring.registry`(描述实例本身,所有已配置中心共享,与注册中心后端
无关——换注册后端是换 blank-import,不是配置迁移):

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | (必填) | 要发布的逻辑名,也是客户端解析用的名字。 |
| `addr` | (必填) | 对外通告的可连 `host:port`。 |
| `id` | (空) | 实例 id 覆盖;空则由 `service-name` + `addr` 推导出稳定 id。 |
| `weight` | `100` | 随实例存储的负载均衡权重。 |
| `metadata.*` | (无) | 随实例存储的任意键值属性。 |

### 发现(消费侧)

发现**无需任何自有配置**:每个块的 bean 本身就是一个 `cloud/discovery` 后端,bean 名即
`consul.<name>`。client starter 按该名字引用:

```properties
spring.registry.consul.main.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# 发现:引用块的 bean 名即可,无需任何配置
spring.http-client.backends.users.discovery=consul.main
```

discovery bean 是惰性的——从不做解析的纯 provider 不会为它付出任何代价。**纯消费方**
只配连接块,不注册任何实例——注册仅在设置 `spring.registry.service-name` 后激活:

```properties
# 纯消费应用:只配连接块,无 service-name/addr,不注册
spring.registry.consul.main.address=127.0.0.1:8500
spring.http-client.backends.users.discovery=consul.main
```

多 agent 发现现在就是多个块:注册进 `consul.main` 与 `consul.dr` 的同时从 `consul.dr`
发现——或只从某个从不注册进去的块发现。

Resolve 只向 Consul 查询 **passing** 实例,不健康实例不会进入快照。保鲜靠每个已解析
服务一条后台 Consul blocking query(基于 index 的长轮询):首次 Resolve 播种缓存,
blocking query 持续刷新,后续 Resolve 都是内存读。通告的 passing 权重即端点权重;
元数据里可选的 `scheme` 键承担传输选择,与 etcd/nacos 适配器对齐。

## 工作原理

- 每个块在 bean 构造阶段成为**一个**后端 bean(`consul.<name>`),持有该 agent 的
  客户端与命名体系的两半。它会探测 agent(`Catalog().Services`),不可达即启动失败,
  每块一次。
- [starter-registry](../starter-registry) 核心(传递依赖自动引入)的 `registryServer`
  收集每个后端的 registrar——跨所有后端——等待就绪,然后带一个 Consul **TTL 健康检查**
  把实例 `Register` 进每个中心。它立即让检查通过,并以 TTL 一半的间隔在后台心跳保活。
- 停机时 `PreStop` 在 pre-stop 延迟之前从每个中心注销实例(停心跳并从 Consul 摘除),
  让发现体系在在途请求仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 Consul
dev agent,启动 [example](example/example.go)(注册、经派生的 discovery 后端回解析自身
注册、然后向自己发 SIGTERM 以触发注销路径),并断言实例已出现。无 Docker 时优雅跳过。


### 运行时权重调整

`Server.UpdateWeight(ctx, weight)` 不注销实例、仅以新权重重新宣告：消费端
（loadbalance 池）在下一个发现快照（一个 Watch 推送周期）生效。权重 0 即摘流
（不接流量但保持注册），是下线前零损失轮转的标准一步。运维也可以直接改注册中心
里的权重值，效果等同。
