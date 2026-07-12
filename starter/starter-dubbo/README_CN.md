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
spring.dubbo.server.protocols.tri.port=20000
```

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
