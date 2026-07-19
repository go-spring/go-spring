# starter-registry-zookeeper

[English](README.md) | [中文](README_CN.md)

`starter-registry-zookeeper` 把**当前实例**注册进 ZooKeeper 集群 —— 它是 Go-Spring
客户端服务发现(`stdlib/discovery`)的注册侧对应物,相当于 Spring Cloud
`ServiceRegistry` 的注册方向,以临时节点(ephemeral znode)为底座。

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
go get go-spring.org/starter-registry-zookeeper
```

## 快速上手

### 1. 匿名导入

```go
import _ "go-spring.org/starter-registry-zookeeper"
```

### 2. 配置 ZooKeeper 集群与实例

```properties
# ZooKeeper 集群(设置 servers 即启用本 starter)。
spring.registry.zookeeper.servers=127.0.0.1:2181
spring.registry.zookeeper.session-timeout=10s
spring.registry.zookeeper.base-path=/services

# 要注册的实例(与后端无关)。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可:启动时把实例写成一个**临时节点**,其生命周期绑定客户端会话;停机时删除该节点。
若进程异常退出,会话过期后 ZooKeeper 自动删除该节点。别处的客户端通过列举同一 base 路径
解析到它。

## 配置项

连接配置,绑定于 `spring.registry.zookeeper`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `servers` | (必填) | 集群成员;设置它即启用本 starter。 |
| `session-timeout` | `10s` | 会话超时;临时节点在会话存续期间存活。 |
| `base-path` | `/services` | 创建服务目录所在的持久父 znode。 |
| `username` | (空) | digest 认证用户名,设置即启用认证。 |
| `password` | (空) | digest 认证密码。 |
| `name` | `default` | 本 registrar 在 `stdlib/discovery` registrar 注册表中的名字。 |

实例配置,绑定于 `spring.registry`(与后端无关 —— 切换注册中心后端只需替换匿名导入,
无需迁移配置):

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | (必填) | 要发布的逻辑名,也是客户端解析用的名字。 |
| `addr` | (必填) | 对外通告的可连 `host:port`。 |
| `id` | (空) | 实例 id 覆盖;空则由 `service-name` + `addr` 推导出稳定 id。 |
| `weight` | `0` | 随实例存储的负载均衡权重。 |
| `metadata.*` | (无) | 随实例存储的任意键值属性。 |
| `backend` | `default` | 按注册表名字选择发布到哪个 registrar 后端。 |

实例以 JSON(`service_name`、`addr`、`weight`、`metadata`)存储于
`<base-path>/<service-name>/<id>`,列举同一 base 路径的 discovery 后端即可还原成
`Endpoint`。

## 工作原理

- 在容器的 bean 注册阶段,starter 连接集群、构建一个 ZooKeeper `discovery.Registrar`
  并以 `name` 放进 `stdlib/discovery` 的 registrar 注册表。它会探测集群(一次 `Exists`
  调用会阻塞到会话连上),不可达的 ZooKeeper 会让启动失败。公司可用另一个名字注册自己的
  `Registrar`,再把 `spring.registry.backend` 指向它。
- 导出的 `gs.Server` 按 `backend` 解析后端,等待就绪,然后 `Register` 实例:按需创建
  持久父目录,并把实例写成一个**临时**叶子节点。
- 停机时 `PreStop` 在 pre-stop 延迟之前注销实例(删除该 znode),让发现体系在在途请求
  仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。若进程崩溃,会话过期后 ZooKeeper
  自动删除该节点。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 ZooKeeper
节点,启动 [example](example/main.go)(注册、列举 znode、然后向自己发 SIGTERM 以触发
注销路径),并断言实例已出现。无 Docker 时优雅跳过。
