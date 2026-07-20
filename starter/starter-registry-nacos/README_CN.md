# starter-registry-nacos

[English](README.md) | [中文](README_CN.md)

`starter-registry-nacos` 把**当前实例**注册进 Nacos 命名服务 —— 它是 Go-Spring
客户端服务发现(`spring/discovery`)的注册侧对应物,相当于 Spring Cloud Alibaba
`nacos-discovery` 的注册方向,也是 [starter-config-nacos](../starter-config-nacos)
配置角色的注册角色对应物(两者是配置前缀各自独立的两个 starter)。

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
go get go-spring.org/starter-registry-nacos
```

## 快速上手

### 1. 匿名导入

```go
import _ "go-spring.org/starter-registry-nacos"
```

### 2. 配置 Nacos 服务与实例

```properties
# Nacos 服务(设置 server 即启用本 starter)。
spring.registry.nacos.server=127.0.0.1:8848
spring.registry.nacos.group=DEFAULT_GROUP
spring.registry.nacos.cluster=DEFAULT

# 要注册的实例(与后端无关)。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可:启动时把实例注册为**临时(ephemeral)实例**并由 SDK 自身的心跳保活,停机时
注销。别处的客户端通过对应注册中心的 `discovery.Discovery` 后端按服务名解析到它。

## 配置项

连接配置,绑定于 `spring.registry.nacos`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `server` | (必填) | Nacos 服务 `host:port`;设置它即启用本 starter。 |
| `namespace` | (空) | 注册到的 namespace id,空则用 `public`。 |
| `group` | `DEFAULT_GROUP` | 服务分组;客户端须在同一分组下解析。 |
| `cluster` | `DEFAULT` | 实例所属的 Nacos 集群名。 |
| `username` | (空) | 认证用户名,匿名集群留空。 |
| `password` | (空) | 认证密码。 |
| `timeout-ms` | `5000` | 每次调用超时,含启动探测。 |
| `name` | `default` | 本 registrar 在 `spring/discovery` registrar 注册表中的名字。 |

实例配置,绑定于 `spring.registry`(与后端无关 —— 切换注册中心后端只需替换匿名导入,
无需迁移配置):

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | (必填) | 要发布的逻辑名,也是客户端解析用的名字。 |
| `addr` | (必填) | 对外通告的可连 `host:port`。 |
| `id` | (空) | 为与其他后端对齐而保留;Nacos 不用,它以 `ip:port` 标识实例。 |
| `weight` | `0` | 负载均衡权重;`0` 回退为 Nacos 默认的 `1`。 |
| `metadata.*` | (无) | 随实例存储的任意键值属性。 |
| `backend` | `default` | 按注册表名字选择发布到哪个 registrar 后端。 |

## 工作原理

- 在容器的 bean 注册阶段,starter 构建一个 Nacos `discovery.Registrar` 并以 `name`
  放进 `spring/discovery` 的 registrar 注册表 —— 与 `starter-discovery-k8s` 注册发现
  后端的方式一致。它会探测服务端(列举服务),不可达的 Nacos 会让启动失败。公司可用
  另一个名字注册自己的 `Registrar`,再把 `spring.registry.backend` 指向它。
- 导出的 `gs.Server` 按 `backend` 解析后端,等待就绪,然后把实例注册为**临时实例**。
  Nacos SDK 以后台心跳保活;若进程未注销就退出,Nacos 会自动摘除。
- 停机时 `PreStop` 在 pre-stop 延迟之前注销实例,让发现体系在在途请求仍被服务时就摘掉
  它。`Stop` 作为幂等兜底再次注销。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 Nacos
standalone 服务,启动 [example](example/main.go)(注册、回读命名服务、然后向自己发
SIGTERM 以触发注销路径),并断言实例已出现。无 Docker 时优雅跳过。
