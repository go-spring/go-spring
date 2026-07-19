# starter-registry-etcd

[English](README.md) | [中文](README_CN.md)

`starter-registry-etcd` 把**当前实例**注册进 etcd 集群 —— 它是 Go-Spring 客户端服务
发现(`stdlib/discovery`)的注册侧对应物,相当于 Spring Cloud `ServiceRegistry` 的注册
方向,以 etcd 租约(lease)为底座。

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
go get go-spring.org/starter-registry-etcd
```

## 快速上手

### 1. 匿名导入

```go
import _ "go-spring.org/starter-registry-etcd"
```

### 2. 配置 etcd 集群与实例

```properties
# etcd 集群(设置 endpoints 即启用本 starter)。
spring.registry.etcd.endpoints=127.0.0.1:2379
spring.registry.etcd.ttl=15s
spring.registry.etcd.key-prefix=/services/

# 要注册的实例(与后端无关)。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可:启动时把实例写在一个**租约**下并由后台 keep-alive 保活,停机时撤销租约并
删除键。若进程异常退出,租约在约 `ttl` 后过期,etcd 自动删除该键。别处的客户端通过读取
同一键前缀解析到它。

## 配置项

连接配置,绑定于 `spring.registry.etcd`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `endpoints` | (必填) | etcd 集群节点;设置它即启用本 starter。 |
| `username` | (空) | 认证用户名,匿名集群留空。 |
| `password` | (空) | 认证密码。 |
| `dial-timeout` | `5s` | 限定初次连接与启动探测耗时。 |
| `ttl` | `15s` | 租约时长;实例存活期间持续保活。向上取整到整秒。 |
| `key-prefix` | `/services/` | 拼在每个键之前,便于多应用共享一个集群。 |
| `tls.*` | (关闭) | 可选客户端 TLS(`enabled`、`cert-file`、`key-file`、`ca-cert-file`)。 |
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
`<key-prefix><service-name>/<id>`,读取同一前缀的 discovery 后端即可还原成 `Endpoint`。

## 工作原理

- 在容器的 bean 注册阶段,starter 构建一个 etcd `discovery.Registrar` 并以 `name`
  放进 `stdlib/discovery` 的 registrar 注册表。它会探测集群(一次 `Status` 调用),
  不可达的 etcd 会让启动失败。公司可用另一个名字注册自己的 `Registrar`,再把
  `spring.registry.backend` 指向它。
- 导出的 `gs.Server` 按 `backend` 解析后端,等待就绪,然后 `Register` 实例:它申请一个
  **租约**、把键写在该租约下,并用后台 keep-alive 保活租约。
- 停机时 `PreStop` 在 pre-stop 延迟之前注销实例(停 keep-alive、撤销租约并删除键),让
  发现体系在在途请求仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。若进程崩溃,租约
  过期后 etcd 自动删除该键。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 etcd 节点,
启动 [example](example/main.go)(注册、回读键、然后向自己发 SIGTERM 以触发注销路径),
并断言实例已出现。无 Docker 时优雅跳过。
