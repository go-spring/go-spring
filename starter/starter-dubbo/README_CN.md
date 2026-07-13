# starter-dubbo

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-dubbo` 基于 [dubbo.apache.org/dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3)
为 Go-Spring 服务提供轻量的 Dubbo 服务器封装：只需注册服务，Starter 会自动完成 Triple
服务器构建、生命周期和优雅停机。

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
spring.dubbo.server.protocols.tri.port=20000
```

`spring.dubbo.application.name` 为**必填**：server 与 client 共享同一个 dubbo
`Instance`，应用名会作为 metrics/注册中心的身份标识，缺失时 starter 会直接报错。
其余应用字段可选：`organization`、`module`、`version`、`owner`、`environment`。

协议与注册中心均为 map 驱动——map 的 key 即 dubbo-go 名称，只有配置了的条目才会生效，
因此一个 server 可同时暴露多种协议并注册到多个后端：

```properties
# 一个 server 上开启多种协议
spring.dubbo.server.protocols.tri.port=20000
spring.dubbo.server.protocols.dubbo.port=20001
# 注册到注册中心（etcdv3/nacos/zookeeper/polaris）
spring.dubbo.server.registries.etcdv3.address=127.0.0.1:2379
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
单注册中心（`registries.<name>`）：`address`、`namespace`、`group`、`username`、
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

类型化配置未覆盖的能力都可以通过 IoC 补充：提供一个对应 optioner 类型的 Bean，其
options 会在最后追加（优先级最高）。

```go
// instance 级：如配置中心、元数据上报
gs.Provide(func() StarterDubbo.InstanceOptioner {
    return func() []dubbo.InstanceOption { return []dubbo.InstanceOption{ /* ... */ } }
})
// server 级
gs.Provide(func() StarterDubbo.ServerOptioner {
    return func() []server.ServerOption { return []server.ServerOption{ /* ... */ } }
})
// client 级
gs.Provide(func() StarterDubbo.ClientOptioner {
    return func() []client.ClientOption { return []client.ClientOption{ /* ... */ } }
})
```

## 核心功能

[示例](example/example.go) 展示了一次 Dubbo Triple 端到端调用，并在 `runTest` 中做了断言：

1. **一元 Greet 调用**：服务端通过 Triple 协议在配置端口导出 `greet.GreetService`。客户端用
   `client.WithClientURL` 直连并调用 `Greet`，拿到原样返回的请求名作为问候语，验证标准的
   请求/响应链路。
2. **服务无关的服务器**：`DubboServer` 完全不认识 `GreetService`，只依赖一个
   `ServiceRegister` Bean，因此同一个服务器可驱动任意 Dubbo 服务；具体注册逻辑放在
   应用层。

## 说明

- 协议与注册中心通过 `${spring.dubbo.server.protocols}` / `${spring.dubbo.server.registries}`
  以 map 驱动，map 的 key 即 dubbo-go 名称，只有配置了的条目才会生效，空字段会被跳过。
  未配置任何协议时，默认使用 20000 端口的 Triple 监听。
- Dubbo 服务器默认开启，可通过 `spring.dubbo.server.enabled=false` 关闭。
- 只需要注册一个 `ServiceRegister` Bean 即可激活整个服务器。
- `spring.dubbo.application.name` 必填；metrics（Prometheus）与 tracing（OTel/stdout）
  内置且默认开启 —— 参见[可观测性](#可观测性内置)。
