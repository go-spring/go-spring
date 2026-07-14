# starter-dubbo

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-dubbo` 基于 [dubbo.apache.org/dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3)
为 Go-Spring 服务提供轻量封装。**服务端**只需注册服务，Starter 会自动完成 Dubbo
服务器构建、生命周期管理与优雅停机；**客户端**则直接提供可用的 `*client.Client`
Bean（一个默认客户端，外加任意具名实例），用于基于注册中心的服务发现。两种角色共享
同一个 dubbo `Instance` 与定义在 `${spring.dubbo.registries}` 下的全局注册中心。

## 安装

```bash
go get go-spring.org/starter-dubbo
```

## 快速开始

### 1. 引入 `starter-dubbo` 包

参见 [example.go](example/example.go) 文件。

```go
import StarterDubbo "go-spring.org/starter-dubbo"
```

### 2. 配置 Dubbo 服务器

在项目的[配置文件](example/conf/app.properties)中添加 Dubbo 配置：

```properties
spring.http.server.enabled=false
spring.dubbo.application.name=greet-example
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379
spring.dubbo.server.protocols.tri.port=20000
```

`spring.dubbo.application.name` 为**必填**：server 与 client 共享同一个 dubbo
`Instance`，应用名会作为 metrics/注册中心的身份标识，缺失时 starter 会直接报错。
其余应用字段可选：`organization`、`module`、`version`、`owner`、`environment`。

`spring.dubbo.registries` 同样为**必填**，是注册中心的唯一事实来源——只在顶层定义一次，
不再内联到 server/client 之下。只有至少定义了一个注册中心时，starter 才会创建 Dubbo
组件，且每个条目都必须带 `address`（二者都会在启动时前置校验）。各角色通过 `registry-ids`
按 ID 选择要用哪些注册中心（留空表示全部）。注册中心为 map 驱动——map 的 key 即逻辑
注册中心 ID——因此可同时定义多个：

```properties
# 只在顶层定义一次注册中心（etcdv3/nacos/zookeeper/polaris/...）
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379
spring.dubbo.registries.nacos.protocol=nacos
spring.dubbo.registries.nacos.address=127.0.0.1:8848

# server（或 client）按 ID 选择注册中心；留空表示全部
spring.dubbo.server.registry-ids=etcd

# 一个 server 上开启多种协议
spring.dubbo.server.protocols.tri.port=20000
spring.dubbo.server.protocols.dubbo.port=20001
```

`${spring.dubbo.server}` 下的所有配置项都是可选的，空值/零值会被跳过，dubbo-go
沿用自身默认值。

Provider 级通用配置：

```properties
spring.dubbo.server.group=g1
spring.dubbo.server.version=1.0.0
spring.dubbo.server.cluster=failover        # failover|failfast|failsafe|failback|forking|available|broadcast|zoneAware
spring.dubbo.server.load-balance=random     # random|roundrobin|leastactive|consistenthashing|p2c
spring.dubbo.server.serialization=hessian2  # hessian2|protobuf|msgpack|json
spring.dubbo.server.retries=2
spring.dubbo.server.filter=echo,tps
spring.dubbo.server.token=xxx
spring.dubbo.server.auth=true
spring.dubbo.server.tag=gray
spring.dubbo.server.access-log=true
spring.dubbo.server.warmup=10m
spring.dubbo.server.not-register=false
spring.dubbo.server.adaptive-service=false
```

单协议（`protocols.<name>`）：`port`、`ip`、`params.<k>`。
单注册中心（顶层 `registries.<name>`）：`address`、`namespace`、`group`、`username`、
`password`、`timeout`（如 `5s`）、`ttl`（如 `15m`）、`weight`、`zone`、
`simplified`、`preferred`、`params.<k>`。

### 3. 注册 Dubbo 服务

参见 [example.go](example/example.go) 文件。`ServiceRegister` 是一个把服务注册到 Dubbo
`server.Server` 上的函数；因为 Dubbo 生成的 `Register*Handler` 会返回 error，所以它也返回
error。

```go
gs.Provide(func() StarterDubbo.ServiceRegister {
    return func(svr *server.Server) error {
        return greet.RegisterGreetServiceHandler(svr, &GreetProvider{})
    }
})
```

## 客户端

Starter 同样以 Bean 形式提供 Dubbo 客户端，其开启条件与服务端相同的 `*Instance`
（没有注册中心的项目不会得到任何客户端）。客户端配置位于 `${spring.dubbo.client}` 下；
注册中心与可观测性都从共享的 `Instance` 继承，因此客户端本身只需关心
protocol/timeout/registry-ids。

只要存在 `Instance`，就始终有一个**默认客户端** Bean（名为 `__default__`）：

```properties
spring.dubbo.client.protocol=tri        # dubbo(默认)|tri|triple|jsonrpc
spring.dubbo.client.timeout=3s          # 单次请求超时，如 "3s"
spring.dubbo.client.registry-ids=etcd   # 按 ID 选择全局注册中心；留空表示全部
```

用 `__default__` Bean 名注入后，再构建生成的 stub：

```go
type Consumer struct {
    Client *client.Client `autowire:"__default__"`
}
// svc, _ := greet.NewGreetService(c.Client)
```

需要**多个客户端**（不同协议或不同注册中心）时，在
`${spring.dubbo.client.instances.<name>}` 下声明具名实例，每个都会成为以 map key
命名的 Bean：

```properties
spring.dubbo.client.instances.orders.protocol=tri
spring.dubbo.client.instances.orders.registry-ids=etcd
spring.dubbo.client.instances.legacy.protocol=dubbo
```

```go
type Caller struct {
    Orders *client.Client `autowire:"orders"`
    Legacy *client.Client `autowire:"legacy"`
}
```

## 可观测性（内置）

metrics 与 tracing 默认开启，每个 server 和 client 零配置即具备可观测能力：

```properties
# Metrics —— Prometheus，默认开启，暴露在 http://127.0.0.1:9090/metrics
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.metrics.path=/metrics

# Tracing —— OTel，默认开启，使用 stdout exporter
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=stdout        # stdout|jaeger|zipkin|otlp-http|otlp-grpc
spring.dubbo.tracing.endpoint=              # exporter 非 stdout 时必填
spring.dubbo.tracing.propagator=w3c         # w3c|b3
spring.dubbo.tracing.mode=                  # always|never|ratio（留空沿用 dubbo-go 默认）
spring.dubbo.tracing.ratio=1.0
```

任一项可用 `enable=false` 关闭。当 `exporter` 非 `stdout` 时必须提供 `endpoint`
（否则 starter 会直接报错）。例如把 trace 发往 OTLP 采集器：

```properties
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317
```

## 定制化（逃生舱）

类型化配置未覆盖的能力可以通过 map 形式的 `params` 字段（如每个协议的 `params`）补充，
这些参数会原样透传给 dubbo-go。

## 核心功能

[示例](example/example.go) 展示了一次 Dubbo Triple 端到端调用，并在 `runTest` 中做了断言：

1. **一元 Greet 调用**：服务端通过 Triple 协议在配置端口导出 `greet.GreetService`。客户端用
   `client.WithClientURL` 直连并调用 `Greet`，拿到原样返回的请求名作为问候语，验证标准的
   请求/响应链路。
2. **服务无关的服务器**：`SimpleDubboServer` 完全不认识 `GreetService`，只依赖一个
   `ServiceRegister` Bean，因此同一个服务器可驱动任意 Dubbo 服务；具体注册逻辑放在
   应用层。

## 说明

- 协议以 map 驱动，位于 `${spring.dubbo.server.protocols}`，map 的 key 即 dubbo-go
  协议名，只有配置了的条目才会生效；注册中心则只在顶层 `${spring.dubbo.registries}`
  定义一次，各角色通过 `registry-ids` 按 ID 选择（留空表示全部）。空字段会被跳过。
  未配置任何协议时，默认使用 20000 端口的 Triple 监听。
- Dubbo 服务器默认开启，可通过 `spring.dubbo.server.enabled=false` 关闭。
- 只需要注册一个 `ServiceRegister` Bean 即可激活整个服务器。
- `spring.dubbo.application.name` 必填；metrics（Prometheus）与 tracing（OTel/stdout）
  内置且默认开启 —— 参见[可观测性](#可观测性内置)。
