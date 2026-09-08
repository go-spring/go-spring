# starter-registry-consul

[English](README.md) | [中文](README_CN.md)

`starter-registry-consul` 把**当前实例**注册进 Consul 服务注册中心 —— 它是
Go-Spring 客户端服务发现(`cloud/discovery`)的注册侧对应物 —— **并且**提供消费侧的
Consul discovery 后端,一个 starter 同时覆盖命名两半。相当于 Spring Cloud 的
`ServiceRegistry` + `DiscoveryClient`,以 Consul TTL 健康检查为底。

适用于**虚机 / 裸机 / 混合**部署,即平台不替你注册实例的场景。**纯 Kubernetes**
下则完全用不到本 starter:平台已把每个 Pod 注册在 Service 之后,你用
[starter-discovery-k8s](../starter-discovery-k8s) 去**发现**对端即可,无需注册。

本 starter 注册的是一个**朴素实例**(任意传输协议 —— HTTP、gRPC……)。RPC 框架的
provider 注册仍保持框架原生,不在本 starter 范围内(见
[starter/DESIGN_CN.md §3](../../DESIGN_CN.md))。

## 形态

全局 / 基础设施类(见 [starter/DESIGN_CN.md §2.4](../../DESIGN_CN.md)):不开端口。
它导出一个 `gs.Server`,让注册接入服务生命周期 —— **应用就绪后**注册实例,**停机
开始时**(经 `PreStop`)注销,使发现体系在实例真正停止服务之前就把它摘除。正是这个
顺序让滚动重启无损。

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
# Consul agent(设置 address 即启用本 starter)。
spring.registry.consul.address=127.0.0.1:8500
spring.registry.consul.ttl=15s
spring.registry.consul.deregister-critical-after=1m

# 要注册的实例(与后端无关)。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可:启动时注册实例并由 TTL 心跳保活,停机时注销。同一份
`spring.registry.consul` 配置还会派生一个名为 `consul` 的 discovery 后端 bean
(见下文),别处的客户端 —— 或本进程自身 —— 经共享连接按服务名解析到它。

## 配置项

### 注册(本实例)

连接配置,绑定于 `spring.registry.consul`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `address` | (必填) | Consul HTTP API 地址;设置它即启用本 starter。 |
| `scheme` | `http` | `http` 或 `https`。 |
| `datacenter` | (空) | 注册到的 datacenter,空则用 agent 的。 |
| `token` | (空) | ACL token。 |
| `namespace` | (空) | Consul Enterprise namespace。 |
| `ttl` | `15s` | TTL 健康检查;starter 以 TTL 一半的间隔心跳。 |
| `deregister-critical-after` | `1m` | 检查持续 critical 超过此时长(如崩溃后),Consul 自动摘除实例。 |
| `discovery-name` | `consul` | 为同一 agent 派生该标签下的 discovery 后端 bean —— 一份配置两半齐备(共享客户端)。留空禁用派生后端。 |

实例配置,绑定于 `spring.registry`(描述实例本身,与注册中心后端无关):

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | (必填) | 要发布的逻辑名,也是客户端解析用的名字。 |
| `addr` | (必填) | 对外通告的可连 `host:port`。 |
| `id` | (空) | 实例 id 覆盖;空则由 `service-name` + `addr` 推导出稳定 id。 |
| `weight` | `0` | 负载均衡权重;`0` 用 Consul 默认。 |
| `metadata.*` | (无) | 随实例存储的任意键值属性。 |

### 发现(消费侧)

消费侧是一个 `cloud/discovery` 后端 bean。两种获得方式:

**从中心派生(双角色应用)。** 只配置 `spring.registry.consul`,即在同一共享客户端
上派生一个标签为 `discovery-name`(默认 `consul`)的后端 bean —— 一份 agent 配置,
两半齐备:

```properties
spring.registry.consul.address=127.0.0.1:8500
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# client 侧引用: <client>.discovery=consul
```

**独立块(其他 agent / 纯消费端)。** 在 `spring.discovery.consul.<name>` 下每个
agent 一块;client starter 引用该名字,经它解析:

```properties
spring.discovery.consul.prod.address=127.0.0.1:8500
spring.discovery.consul.prod.tag=v2
```

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `address` | (空) | Consul HTTP API 地址;留空则**继承** `spring.registry.consul` 中心连接(共享客户端)。 |
| `scheme` | `http` | `http` 或 `https`。 |
| `datacenter` | (空) | 查询的 datacenter,空则用 agent 的。 |
| `token` | (空) | ACL token。 |
| `namespace` | (空) | Consul Enterprise namespace。 |
| `tag` | (空) | 收窄每次查询的 Consul 服务标签(`Health().Service` 的 tag 参数)。 |

Resolve 只向 Consul 查询 **passing** 实例,不健康实例不会进入快照。保鲜靠每个已解析
服务一条后台 Consul blocking query(基于 index 的长轮询):首次 Resolve 播种缓存,
blocking query 持续刷新,后续 Resolve 都是内存读。通告的 passing 权重即端点权重;
元数据里可选的 `scheme` 键承担传输选择,与 etcd/nacos 适配器对齐。

## 工作原理

- 在 bean 构造阶段,starter 为 `spring.registry.consul` 构建共享 Consul 客户端
  (探测 agent,不可达即启动失败),并由它派生 registrar 与 `discovery-name` 标签的
  discovery 后端。registrar 注入导出的 `gs.Server`。
- 导出的 `gs.Server` 等待就绪,然后带一个 Consul **TTL 健康检查**`Register` 实例。它
  立即让检查通过,并以 TTL 一半的间隔在后台心跳保活。
- 停机时 `PreStop` 在 pre-stop 延迟之前注销实例(停心跳并从 Consul 摘除),让发现体系
  在在途请求仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 Consul
dev agent,启动 [example](example/example.go)(注册、经派生的 discovery 后端回解析自身
注册、然后向自己发 SIGTERM 以触发注销路径),并断言实例已出现。无 Docker 时优雅跳过。


### 运行时权重调整

`Server.UpdateWeight(ctx, weight)` 不注销实例、仅以新权重重新宣告：消费端
（loadbalance 池）在下一个发现快照（一个 Watch 推送周期）生效。权重 0 即摘流
（不接流量但保持注册），是下线前零损失轮转的标准一步。运维也可以直接改注册中心
里的权重值，效果等同。
