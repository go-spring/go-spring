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

```properties
# 注册中心：一个命名块一个中心，块数不限、后端可混用。
# 每个块是一个 bean，名字为 "<backend>.<name>"。
spring.registry.etcd.main.endpoints=10.0.0.1:2379
spring.registry.etcd.dr.endpoints=10.9.0.1:2379      # 双注册
spring.registry.zookeeper.bz.servers=10.1.0.1:2181   # 混用后端

# 实例身份——所有中心共享同一份。service-name 是注册意图信号：
# 有值 = 发布本进程；无值 = 纯消费端（不向任何中心注册）。
spring.registry.service-name=orders
spring.registry.addr=10.0.0.5:8080
# spring.registry.id=            # 可选，留空自动派生
# spring.registry.weight=100
# spring.registry.metadata.zone=cn-north

# 消费端用 bean 名引用某个中心：
# spring.http-client.backends.users.discovery=etcd.main
```

| key | 含义 |
|-----|------|
| `spring.registry.service-name` | 注册意图 + 消费端解析的服务名 |
| `spring.registry.addr` | 对外发布的 host:port |
| `spring.registry.id` | 服务内的实例 id（留空自动派生） |
| `spring.registry.weight` | 初始负载均衡权重（默认 100；0 = 摘流） |
| `spring.registry.metadata.*` | 后端无关的附加属性 |

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
