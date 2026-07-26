# starter-registry-consul

[English](README.md) | [中文](README_CN.md)

`starter-registry-consul` 把**当前实例**注册进 Consul 服务注册中心 —— 它是
Go-Spring 客户端服务发现(`spring/discovery`)的注册侧对应物,相当于 Spring Cloud
`ServiceRegistry` / `@EnableDiscoveryClient` 的注册方向。

适用于**虚机 / 裸机 / 混合**部署,即平台不替你注册实例的场景。**纯 Kubernetes**
下则完全用不到本 starter:平台已把每个 Pod 注册在 Service 之后,你用
[starter-discovery-k8s](../starter-discovery-k8s) 去**发现**对端即可,无需注册。

本 starter 注册的是一个**朴素实例**(任意传输协议 —— HTTP、gRPC……)。RPC 框架的
provider 注册仍保持框架原生,不在本 starter 范围内(见
[starter/DESIGN_CN.md §3](../DESIGN_CN.md))。

## 形态

全局 / 基础设施类(见 [starter/DESIGN_CN.md §2.4](../DESIGN_CN.md)):不开端口。
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

到此即可:启动时注册实例并由 TTL 心跳保活,停机时注销。别处的客户端通过对应注册中心
的 `discovery.Discovery` 后端按服务名解析到它。

## 配置项

连接配置,绑定于 `spring.registry.consul`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `address` | (必填) | Consul HTTP API 地址;设置它即启用本 starter。 |
| `scheme` | `http` | `http` 或 `https`。 |
| `datacenter` | (空) | 注册到的 datacenter,空则用 agent 的。 |
| `token` | (空) | ACL token。 |
| `namespace` | (空) | Consul Enterprise namespace。 |
| `name` | `default` | 本 registrar 在 `spring/discovery` registrar 注册表中的名字。 |
| `ttl` | `15s` | TTL 健康检查;starter 以 TTL 一半的间隔心跳。 |
| `deregister-critical-after` | `1m` | 检查持续 critical 超过此时长(如崩溃后),Consul 自动摘除实例。 |

实例配置,绑定于 `spring.registry`(与后端无关 —— 切换注册中心后端只需替换匿名导入,
无需迁移配置):

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | (必填) | 要发布的逻辑名,也是客户端解析用的名字。 |
| `addr` | (必填) | 对外通告的可连 `host:port`。 |
| `id` | (空) | 实例 id 覆盖;空则由 `service-name` + `addr` 推导出稳定 id。 |
| `weight` | `0` | 负载均衡权重;`0` 用 Consul 默认。 |
| `metadata.*` | (无) | 随实例存储的任意键值属性。 |
| `backend` | `default` | 按注册表名字选择发布到哪个 registrar 后端。 |

## 工作原理

- 在容器的 bean 注册阶段,starter 构建一个 Consul `discovery.Registrar` 并以 `name`
  放进 `spring/discovery` 的 registrar 注册表 —— 与 `starter-discovery-k8s` 注册发现
  后端的方式一致。公司可用另一个名字注册自己的 `Registrar`,再把
  `spring.registry.backend` 指向它。
- 导出的 `gs.Server` 按 `backend` 解析后端,等待就绪,然后带一个 Consul **TTL 健康
  检查**`Register` 实例。它立即让检查通过,并以 TTL 一半的间隔在后台心跳保活。
- 停机时 `PreStop` 在 pre-stop 延迟之前注销实例(停心跳并从 Consul 摘除),让发现体系
  在在途请求仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 Consul
dev agent,启动 [example](example/main.go)(注册、回读 catalog、然后向自己发 SIGTERM
以触发注销路径),并断言实例已出现。无 Docker 时优雅跳过。
