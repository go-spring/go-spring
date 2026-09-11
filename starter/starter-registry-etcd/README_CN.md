# starter-registry-etcd

[English](README.md) | [中文](README_CN.md)

`starter-registry-etcd` 把**当前实例**注册进 etcd 集群 —— 它是 Go-Spring 客户端服务
发现(`cloud/discovery`)的注册侧对应物 —— **并**提供消费侧的 etcd discovery 后端,
一个 starter 覆盖命名体系的两半,相当于 Spring Cloud `ServiceRegistry` +
`DiscoveryClient`,以 etcd 租约(lease)为底座。

适用于**虚机 / 裸机 / 混合**部署,即平台不替你注册实例的场景。**纯 Kubernetes**
下则完全用不到本 starter:平台已把每个 Pod 注册在 Service 之后,你用
[starter-discovery-k8s](../starter-discovery-k8s) 去**发现**对端即可,无需注册。

本 starter 注册的是一个**朴素实例**(任意传输协议 —— HTTP、gRPC……)。RPC 框架的
provider 注册仍保持框架原生,不在本 starter 范围内(见
[starter/DESIGN_CN.md §3](../DESIGN_CN.md))。

## 模型

配置是**命名块**:每个 `spring.registry.etcd.<name>.*` 块描述一个 etcd 集群,并成为
一个名为 `etcd.<name>` 的后端 bean,同时实现写侧(`discovery.Registrar`)与读侧
(`discovery.Discovery`),共享该块的客户端与键前缀。没有默认/无名块。

注册本身由 [starter-registry](../starter-registry) 核心拥有(传递依赖自动引入):它
唯一的 `registryServer`(`gs.Server`)收集**所有**已配置中心的 registrar bean——跨后端
——在应用就绪后把实例注册进每一个中心,停机时从全部中心注销,并向所有中心广播
`UpdateWeight`。配两个块(etcd + etcd,或 etcd + zookeeper……)再加
`spring.registry.service-name`,即得双中心/多中心注册,零额外配置——Dubbo 式默认。

本 starter 不开端口:导出的 `gs.Server` 纯粹为了让注册接入服务生命周期——**应用就绪
后**发布实例,**停机开始时**(经 `PreStop`)注销,使发现体系在实例真正停止服务之前
就把它摘除。正是这个顺序让滚动重启无损。

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
# 每个命名块一个 etcd 集群;"main" 是块名(自选)。
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.registry.etcd.main.ttl=15s
spring.registry.etcd.main.key-prefix=/services/

# 第二个块 = 第二个中心 = 双注册(零额外配置)。
# spring.registry.etcd.dr.endpoints=10.9.0.1:2379

# 要注册的实例(与后端无关,所有中心共享)。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

到此即可:启动时把实例写进**每一个**已配置集群的**租约**下并由后台 keep-alive 保活,
停机时撤销租约并删除键。若进程异常退出,租约在约 `ttl` 后过期,etcd 自动删除该键。
别处的客户端通过读取同一键前缀解析到它。

## 配置项

### 注册(本实例)

连接配置,按块绑定于 `spring.registry.etcd.<name>`:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `endpoints` | (必填) | etcd 集群节点;设置一个块即激活它。 |
| `username` | (空) | 认证用户名,匿名集群留空。 |
| `password` | (空) | 认证密码。 |
| `dial-timeout` | `5s` | 限定初次连接与启动探测耗时。 |
| `ttl` | `15s` | 租约时长;实例存活期间持续保活。向上取整到整秒。 |
| `key-prefix` | `/services/` | 拼在每个键之前,便于多应用共享一个集群。 |
| `tls.*` | (关闭) | 可选客户端 TLS(`enabled`、`cert-file`、`key-file`、`ca-file`)。 |

每个块成为**一个**后端 bean `etcd.<name>`——一个客户端、一次启动探测、一个生命周期;
命名体系的两半共享它们,读写永不分裂。**只有设置了 `service-name` 才会注册**;纯消费方
应用省略该键,不注册任何实例。

实例配置,绑定于 `spring.registry`(描述实例本身,所有已配置中心共享,与注册中心后端
无关):

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `service-name` | (必填) | 要发布的逻辑名,也是客户端解析用的名字。 |
| `addr` | (必填) | 对外通告的可连 `host:port`。 |
| `id` | (空) | 实例 id 覆盖;空则由 `service-name` + `addr` 推导出稳定 id。 |
| `weight` | `100` | 随实例存储的负载均衡权重。 |
| `metadata.*` | (无) | 随实例存储的任意键值属性。 |

实例以 JSON(`service_name`、`addr`、`weight`、`metadata`)存储于
`<key-prefix><service-name>/<id>`,读取同一前缀的 discovery 后端即可还原成 `Endpoint`。

### 发现(消费侧)

发现**无需任何自有配置**:每个块的 bean 本身就是一个 `cloud/discovery` 后端,bean 名即
`etcd.<name>`。client starter 按该名字引用:

```properties
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.registry.etcd.dr.endpoints=10.9.0.1:2379
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# 发现:引用块的 bean 名即可,无需任何配置
spring.http-client.backends.users.discovery=etcd.main
```

discovery bean 是惰性的——从不做解析的纯 provider 不会为它付出任何代价。**纯消费方**
只配连接块,不注册任何实例——注册仅在设置 `spring.registry.service-name` 后激活:

```properties
# 纯消费应用:只配连接块,无 service-name/addr,不注册
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.http-client.backends.users.discovery=etcd.main
```

多集群发现现在就是多个块:注册进 `etcd.main` 与 `etcd.dr` 的同时从 `etcd.dr` 发现——
或只从某个从不注册进去的块发现。

健康由键的存活性推导:实例键只在租约存活期间存在,因此找到的每个键都是活实例 ——
无需探测协议。注册实例元数据里可选的 `scheme` 键承担传输选择,
与 nacos 适配器对齐。

## 工作原理

- 每个块在 bean 构造阶段成为**一个**后端 bean(`etcd.<name>`),持有该集群的客户端与
  命名体系的两半。它会探测集群(一次 `Status` 调用),不可达的 etcd 会让启动失败,每块
  一次。
- [starter-registry](../starter-registry) 核心(传递依赖自动引入)的 `registryServer`
  收集每个后端的 registrar——跨所有后端——等待就绪,然后把实例 `Register` 进每个中心:
  每中心申请一个**租约**、把键写在该租约下并用后台 keep-alive 保活。若租约在服务端死亡,
  registrar 会带退避地重注册——条目无需运维干预即可回来。
- 停机时 `PreStop` 在 pre-stop 延迟之前从每个中心注销实例(停 keep-alive、撤销租约并
  删除键),让发现体系在在途请求仍被服务时就摘掉它。`Stop` 作为幂等兜底再次注销。若进程
  崩溃,租约过期后 etcd 自动删除该键。

## 冒烟测试

[example/check.sh](example/check.sh) 先跑单测,再(在有 Docker 时)启动一个 etcd 节点,
启动 [example](example/example.go)(注册、回读键、然后向自己发 SIGTERM 以触发注销路径),
并断言实例已出现。无 Docker 时优雅跳过。


### 运行时权重调整

`Server.UpdateWeight(ctx, weight)` 不注销实例、仅以新权重重新宣告：消费端
（loadbalance 池）在下一个发现快照（一个刷新周期）生效。权重 0 即摘流
（不接流量但保持注册），是下线前零损失轮转的标准一步。运维也可以直接改注册中心
里的权重值，效果等同。
### 日志 tag

本模块的运行期日志使用 tag `_app_registry_etcd`（etcd 注册中心）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.registry_etcd.type=Logger
logger.registry_etcd.level=WARN
logger.registry_etcd.tag=_app_registry_etcd
```
