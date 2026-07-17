# go-kratos —— HTTP(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个以 Go-Spring 方式驱动的 [go-kratos](https://github.com/go-kratos/kratos)
`Greeter` 示例:`gs.Run()` 掌管生命周期,服务是一个 IoC bean,kratos HTTP transport
server 由 [`starter-kratos/http`](../../../starter/starter-kratos) starter 贡献,而非在
`main()` 里手工接线。

provider 启动时把自己注册进 **etcd 注册中心**;consumer 从不知道 provider 的 host:port,
而是通过 `discovery:///<name>` 从同一个 etcd 解析出一个存活地址。

这是一个可运行的示例,**不是**可复用的 starter 模块。同一 `Greeter` 服务的 gRPC 与
WebSocket 两半在隔壁 [`../grpc`](../grpc) 与 [`../ws`](../ws) —— 各自是独立 module,接线
自己的 kratos transport,因此 import 其中一个绝不会拖入其它的依赖。

## 拓扑

```
                ┌──────────────┐
    注册         │     etcd     │    发现
  ┌────────────▶│  :2379       │◀────────────┐
  │  kratos-http └──────────────┘ kratos-http │
  │  (name)                        (name)     │ 解析 provider 地址
  │                                           │
┌─┴───────────┐        HTTP :8000        ┌────┴────────┐
│  provider   │◀─────────────────────────│  consumer   │
│ gs.Run()    │      SayHello("Kratos")  │ gs.Run()    │
└─────────────┘                          └─────────────┘
```

## 结构

```
http/
├── provider/            gs.Run() + ServiceRegister bean(handler.go)
│   └── conf/app.properties
├── consumer/            走 etcd 发现的 HTTP 客户端
│   └── conf/app.properties
├── idl/helloworld/v1/   proto + 生成的 stub(gen-code.sh 重新生成)
├── docker-compose.yml   仅 etcd
└── scripts/smoke-test.sh
```

## 运行

```bash
# 1. 启动 etcd。
docker compose up -d

# 2. 启动 provider(把 kratos-http 注册进 etcd,监听 HTTP :8000)。
go run ./provider

# 3. 另开一个终端,运行 consumer(发现并通过 HTTP 调用)。
go run ./consumer
# → Response from discovered provider (HTTP): Hello Kratos
```

或一键跑完整回环:

```bash
./scripts/smoke-test.sh
```

## 配置

kratos HTTP server 从 `${spring.kratos.http.server}` 前缀绑定(见
`provider/conf/app.properties`):

| 键 | 默认值 | 含义 |
| --- | --- | --- |
| `spring.kratos.http.server.name` | `kratos-http` | 发布进 etcd 的服务名 |
| `spring.kratos.http.server.addr` | `0.0.0.0:8000` | HTTP 监听地址 |
| `spring.kratos.http.server.etcd.addr` | *(空)* | etcd 端点;空 = 直连,不注册 |

可观测(tracing/metrics)让路给
[`starter-otel`](../../../starter/starter-otel):import 它,starter 的
`tracing.Server()` 中间件便会自动导出 span。本精简示例二者都不含。
