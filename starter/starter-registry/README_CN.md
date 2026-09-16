# starter-registry

注册族的核心：唯一拥有本进程发布生命周期的 server，统摄所有已配置的注册中心。

**应用永远不直接 import 本模块。** 每个后端 starter
（starter-registry-etcd、-consul、-nacos、-zookeeper）都依赖它，因此它随传递
引入而来，无论混用几个后端，registryServer bean 每进程恰好一个。

## 它做什么

每个后端 starter 为每个已配置的 `${spring.registry.<backend>.<name>}` 块派生
一个 discovery.Registrar。本核心把它们全部收集起来——跨后端——并统一驱动：

- 应用就绪后，注册到所有中心；
- 停机开始时（PreStop，先于任何 server 停止），从所有中心反注册——无损下线时序；
- 权重变更广播到所有中心（UpdateWeight）。

任一中心注册失败即启动失败：消费方在某个中心看不到本实例等于视图分裂。

## 配置

整个 registry 的配置面分**三部分**，各自独立、可单独出现：

| 部分 | 回答的问题 | 配置前缀 | 谁来配 |
|------|-----------|---------|--------|
| ① 实例身份 | 我是谁 | `${spring.registry}.*` | 仅提供方 |
| ② 注册中心 | 往哪注册、从哪发现 | `${spring.registry.<backend>.<name>}.*` | 提供方和/或消费方 |
| ③ 引用 | 消费方用哪个中心 | 各 client starter 自己的配置段 | 仅消费方 |

### ① 实例身份——`${spring.registry}.*`

所有中心共享同一份。`service-name` 是注册意图信号：**有值 = 发布本进程；无值 = 纯消费端**
（不向任何中心注册）。

```properties
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080      # 注册时必填
# spring.registry.id=                   # 可选，留空自动派生 <name>-<addr>
# spring.registry.weight=100            # 0 = 摘流；负数写入时归一为 1
# spring.registry.version=              # 应用版本，消费端可按版本灰度路由
# spring.registry.zone=                 # 可用区，消费端可同区优先
# spring.registry.scheme=               # 传输协议提示（tcp/tls/http/https）
# spring.registry.metadata.zone=cn-north
```

| key | 含义 |
|-----|------|
| `spring.registry.service-name` | 注册意图 + 消费端解析的服务名 |
| `spring.registry.addr` | 对外发布的 host:port |
| `spring.registry.id` | 服务内的实例 id（留空自动派生） |
| `spring.registry.weight` | 初始负载均衡权重（默认 100；0 = 摘流） |
| `spring.registry.version` | 实例的应用版本（可选） |
| `spring.registry.zone` | 实例所在可用区（可选） |
| `spring.registry.scheme` | 传输协议提示，映射到消费端 `Endpoint.Scheme`（可选） |
| `spring.registry.metadata.*` | 后端无关的附加属性 |

### ② 注册中心——命名块，一块一个中心

块数不限、后端可混用（etcd + zookeeper 同进程正常）。每个块是一个 bean，名字为
`<backend>.<name>`，**一体两面**：写侧被本核心收集为 Registrar，读侧是消费端按名引用的
Discovery 后端。各后端的完整 key 见其 starter 的 README/USAGE。

```properties
spring.registry.etcd.main.endpoints=10.0.0.1:2379
spring.registry.etcd.dr.endpoints=10.9.0.1:2379      # 双注册：注册扇出到两个中心
spring.registry.zookeeper.bz.servers=10.1.0.1:2181   # 混用后端
```

### ③ 引用——消费端按 bean 名指定中心

写在各 client starter 自己的配置段里；发现侧自身零配置，registry 不感知谁在引用。

```properties
# http-client:
spring.http-client.instances.users.service-name=users
spring.http-client.instances.users.discovery=etcd.main    # bean 名 "<backend>.<name>"

# gateway:
spring.gateway.discovery=nacos.main                       # 全局默认
# spring.gateway.routes.<id>.upstream.discovery=...       # 路由级覆盖

# redigo（redis）:
spring.redigo.instances.cache.service-name=cache-svc
spring.redigo.instances.cache.discovery=consul.main

# gorm（数据库，按方言）:
spring.gorm.mysql.orders.service-name=mysql-svc
spring.gorm.mysql.orders.discovery=etcd.main

# mongodb:
spring.mongodb.instances.logs.service-name=mongo-svc
spring.mongodb.instances.logs.discovery=nacos.main

# discovery 支持家族级默认：实例块没写时回退 ${<family>.default.discovery}。
spring.http-client.default.discovery=etcd.main           # 家族默认
spring.http-client.instances.pay.service-name=pay-svc
spring.http-client.instances.pay.discovery=etcd.main     # 实例级覆盖（不写则走默认）
```

凡是走发现模式建 `loadbalance.Pool` 的 client starter 都是这个形状：实例配置里给
`service-name` + `discovery=<bean 名>`。各 starter 的完整 key 见各自 README/USAGE。

### 三部分的边界

- **①与②解耦**：配了块 + service-name 就注册，哪怕没有任何消费端引用它；反过来纯消费端
  可以只配块、不设 service-name，什么都不注册。
- **设了 service-name 但没有任何块 = 启动失败**（fail-fast，见下"说明"）。
- **k8s 只出现在②③**：它是 discovery-only 后端，只能被引用、不参与注册（平台已替 Pod 注册），
  与①永远无关。
- **同进程可既是提供方又是消费方**：①+②+③ 全配即可。

## 运行时 API

registryServer bean 上的 `UpdateWeight(ctx, weight)` 向所有中心以新权重重新
发布本实例——实例不会离开发现视图；观察方（含 loadbalance pool）在下次快照
看到新值。

## 说明

- 本 server 不开端口；接入 Go-Spring 的 server 生命周期。
- 本核心自身不带任何注册中心后端。配了 service-name 但没有任何注册中心块时启动
  失败（fail-fast，不静默，报 "no registry center is configured"）——需加一个
  starter-registry-<backend>。
- 发现侧自身零配置——见各后端 starter 的 README。
